package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strings"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/operations"
)

const muPDFReleasesURL = "https://api.github.com/repos/ArtifexSoftware/mupdf-downloads/releases?per_page=100"

var muPDFVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// MuPDFCatalog installs only user-requested official MuPDF archives into
// Localize data. It never writes into the application or installer directory.
type MuPDFCatalog struct {
	root        string
	http        *http.Client
	operations  *operations.Hub
	releasesURL string
}

func NewMuPDFCatalog(root string, hub *operations.Hub) *MuPDFCatalog {
	return &MuPDFCatalog{root: root, http: &http.Client{Timeout: 0}, operations: hub, releasesURL: muPDFReleasesURL}
}

func (c *MuPDFCatalog) List(ctx context.Context) ([]domain.MuPDFRelease, error) {
	return c.releases(ctx)
}

func (c *MuPDFCatalog) Status(selectedVersion string) (domain.MuPDFRuntimeStatus, error) {
	if err := os.MkdirAll(c.root, 0o700); err != nil {
		return domain.MuPDFRuntimeStatus{}, fmt.Errorf("create MuPDF runtime directory: %w", err)
	}
	entries, err := os.ReadDir(c.root)
	if err != nil {
		return domain.MuPDFRuntimeStatus{}, fmt.Errorf("read MuPDF runtime directory: %w", err)
	}
	installed := make([]domain.MuPDFInstalledRuntime, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !muPDFVersionPattern.MatchString(entry.Name()) {
			continue
		}
		if fileExists(filepath.Join(c.root, entry.Name(), "mutool.exe")) {
			installed = append(installed, domain.MuPDFInstalledRuntime{Version: entry.Name()})
		}
	}
	sort.Slice(installed, func(i, j int) bool { return installed[i].Version > installed[j].Version })
	return domain.MuPDFRuntimeStatus{Root: c.root, SelectedVersion: strings.TrimSpace(selectedVersion), Installed: installed}, nil
}

// Mutool returns only the selected managed executable, preventing accidental
// use of an arbitrary system-wide MuPDF installation.
func (c *MuPDFCatalog) Mutool(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", errors.New("install and select a MuPDF runtime in Settings before translating documents")
	}
	if !muPDFVersionPattern.MatchString(version) {
		return "", errors.New("the selected MuPDF runtime version is invalid")
	}
	mutool := filepath.Join(c.root, version, "mutool.exe")
	if !fileExists(mutool) {
		return "", fmt.Errorf("MuPDF %s is not installed; install it in Settings", version)
	}
	return mutool, nil
}

func (c *MuPDFCatalog) Install(ctx context.Context, request domain.MuPDFRuntimeInstallRequest, operationID string) error {
	if goruntime.GOOS != "windows" {
		return errors.New("MuPDF runtime installation is currently available on Windows only")
	}
	version := strings.TrimSpace(request.Version)
	if !muPDFVersionPattern.MatchString(version) {
		return errors.New("choose a valid MuPDF release")
	}
	releases, err := c.releases(ctx)
	if err != nil {
		return err
	}
	var release *domain.MuPDFRelease
	for index := range releases {
		if releases[index].Version == version {
			release = &releases[index]
			break
		}
	}
	if release == nil {
		return fmt.Errorf("MuPDF release %s is no longer available", version)
	}
	if err := os.MkdirAll(c.root, 0o700); err != nil {
		return fmt.Errorf("create MuPDF runtime directory: %w", err)
	}
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "mupdf-install", Stage: "prepare", Message: "Preparing MuPDF " + release.Version, Total: release.Artifact.Size})
	archive, err := c.download(ctx, *release, operationID)
	if err != nil {
		return err
	}
	if err := c.publish(*release, archive, operationID); err != nil {
		return err
	}
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "mupdf-install", Stage: "complete", Message: "Installed MuPDF " + release.Version, Completed: release.Artifact.Size, Total: release.Artifact.Size})
	return nil
}

func (c *MuPDFCatalog) download(ctx context.Context, release domain.MuPDFRelease, operationID string) (string, error) {
	downloads := filepath.Join(c.root, ".downloads")
	if err := os.MkdirAll(downloads, 0o700); err != nil {
		return "", err
	}
	archive := filepath.Join(downloads, "mupdf-"+release.Version+"-windows.zip")
	if info, err := os.Stat(archive); err == nil {
		if info.Size() == release.Artifact.Size {
			c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "mupdf-install", Stage: "download", Message: "Using cached MuPDF " + release.Version + " archive", Completed: info.Size(), Total: release.Artifact.Size})
			return archive, nil
		}
		if err := os.Remove(archive); err != nil {
			return "", fmt.Errorf("discard incomplete MuPDF archive: %w", err)
		}
	}

	partial := archive + ".part"
	offset := int64(0)
	if info, err := os.Stat(partial); err == nil {
		offset = info.Size()
		if offset > release.Artifact.Size {
			if err := os.Remove(partial); err != nil {
				return "", fmt.Errorf("discard oversized MuPDF partial download: %w", err)
			}
			offset = 0
		}
		if offset == release.Artifact.Size {
			if err := os.Rename(partial, archive); err != nil {
				return "", err
			}
			return archive, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, release.Artifact.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "LocalizeDesktop/0.1")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	response, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("download MuPDF: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("MuPDF download returned %s", response.Status)
	}
	if response.StatusCode == http.StatusOK {
		offset = 0
	}
	flags := os.O_CREATE | os.O_WRONLY
	if offset == 0 {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	target, err := os.OpenFile(partial, flags, 0o600)
	if err != nil {
		return "", err
	}
	completed, lastCompleted, lastEmit := offset, offset, time.Now()
	buffer := make([]byte, 256*1024)
	for {
		read, readErr := response.Body.Read(buffer)
		if read > 0 {
			if _, writeErr := target.Write(buffer[:read]); writeErr != nil {
				_ = target.Close()
				return "", writeErr
			}
			completed += int64(read)
			now := time.Now()
			if now.Sub(lastEmit) >= 250*time.Millisecond {
				speed := int64(float64(completed-lastCompleted) / now.Sub(lastEmit).Seconds())
				c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "mupdf-install", Stage: "download", Message: "Downloading MuPDF " + release.Version, Completed: completed, Total: release.Artifact.Size, SpeedBytesPerSecond: speed})
				lastEmit, lastCompleted = now, completed
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = target.Close()
			return "", readErr
		}
	}
	if err := target.Close(); err != nil {
		return "", err
	}
	if completed != release.Artifact.Size {
		return "", fmt.Errorf("MuPDF download size validation failed: received %d bytes, expected %d", completed, release.Artifact.Size)
	}
	if err := os.Rename(partial, archive); err != nil {
		return "", err
	}
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "mupdf-install", Stage: "verify", Message: "Validating MuPDF archive", Completed: completed, Total: release.Artifact.Size})
	return archive, nil
}

func (c *MuPDFCatalog) publish(release domain.MuPDFRelease, archive, operationID string) error {
	staging, err := os.MkdirTemp(c.root, ".mupdf-install-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "mupdf-install", Stage: "install", Message: "Installing MuPDF files", Completed: release.Artifact.Size, Total: release.Artifact.Size})
	if err := extractMuPDF(archive, staging); err != nil {
		return err
	}
	final := filepath.Join(c.root, release.Version)
	backup := final + ".previous"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(final); err == nil {
		if err := os.Rename(final, backup); err != nil {
			return fmt.Errorf("replace existing MuPDF runtime: %w", err)
		}
	}
	if err := os.Rename(staging, final); err != nil {
		if _, restoreErr := os.Stat(backup); restoreErr == nil {
			_ = os.Rename(backup, final)
		}
		return fmt.Errorf("publish MuPDF runtime: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove previous MuPDF runtime: %w", err)
	}
	return nil
}

func extractMuPDF(archive, destination string) error {
	scratch, err := os.MkdirTemp(filepath.Dir(destination), ".mupdf-extract-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	if err := extractArchive(archive, scratch); err != nil {
		return err
	}
	mutool := ""
	err = filepath.WalkDir(scratch, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "mutool.exe") {
			mutool = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return err
	}
	if mutool == "" {
		return errors.New("MuPDF archive did not contain mutool.exe")
	}
	return copyDirectory(filepath.Dir(mutool), destination)
}

func (c *MuPDFCatalog) releases(ctx context.Context) ([]domain.MuPDFRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.releasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LocalizeDesktop/0.1")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch MuPDF releases: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return nil, fmt.Errorf("MuPDF release lookup returned %s", response.Status)
	}
	var raw []struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		Draft       bool   `json:"draft"`
		Prerelease  bool   `json:"prerelease"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode MuPDF releases: %w", err)
	}
	releases := make([]domain.MuPDFRelease, 0, len(raw))
	for _, item := range raw {
		if item.Draft || item.Prerelease || !muPDFVersionPattern.MatchString(item.TagName) {
			continue
		}
		assetName := "mupdf-" + item.TagName + "-windows.zip"
		for _, asset := range item.Assets {
			if asset.Name == assetName && asset.BrowserDownloadURL != "" && asset.Size > 0 {
				releases = append(releases, domain.MuPDFRelease{
					Version:     item.TagName,
					PublishedAt: item.PublishedAt,
					Artifact:    domain.LlamaCppRuntimeArtifact{URL: asset.BrowserDownloadURL, Size: asset.Size},
				})
				break
			}
		}
	}
	if len(releases) == 0 {
		return nil, errors.New("no compatible Windows MuPDF releases are currently available")
	}
	return releases, nil
}
