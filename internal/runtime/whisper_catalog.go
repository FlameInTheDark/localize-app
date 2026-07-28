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

const whisperReleasesURL = "https://api.github.com/repos/ggml-org/whisper.cpp/releases?per_page=16"

var (
	whisperVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	whisperCPUAsset       = regexp.MustCompile(`^whisper-bin-x64\.zip$`)
	whisperCUDAAsset      = regexp.MustCompile(`^whisper-cublas-(12\.4\.0|11\.8\.0)-bin-x64\.zip$`)
)

// WhisperCatalog discovers and installs official whisper.cpp Windows runtimes.
// Installation is user-owned, resumable, checksum-verified, and never writes
// into the application installation directory.
type WhisperCatalog struct {
	root        string
	http        *http.Client
	operations  *operations.Hub
	releasesURL string
}

func NewWhisperCatalog(root string, hub *operations.Hub) *WhisperCatalog {
	return &WhisperCatalog{root: root, http: &http.Client{Timeout: 0}, operations: hub, releasesURL: whisperReleasesURL}
}

func (c *WhisperCatalog) List(ctx context.Context) ([]domain.WhisperCppRelease, error) {
	releases, err := c.releases(ctx)
	if err != nil {
		return nil, err
	}
	return releases, nil
}

func (c *WhisperCatalog) Status(selectedVersion string) (domain.WhisperCppRuntimeStatus, error) {
	if err := os.MkdirAll(c.root, 0o700); err != nil {
		return domain.WhisperCppRuntimeStatus{}, fmt.Errorf("create whisper.cpp runtime directory: %w", err)
	}
	entries, err := os.ReadDir(c.root)
	if err != nil {
		return domain.WhisperCppRuntimeStatus{}, fmt.Errorf("read whisper.cpp runtime directory: %w", err)
	}
	installed := make([]domain.WhisperCppInstalledRuntime, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !whisperVersionPattern.MatchString(entry.Name()) {
			continue
		}
		versionRoot := filepath.Join(c.root, entry.Name())
		cpu := fileExists(filepath.Join(versionRoot, string(domain.RuntimeCPU), "whisper-cli.exe"))
		cuda := fileExists(filepath.Join(versionRoot, string(domain.RuntimeCUDA), "whisper-cli.exe"))
		if cpu || cuda {
			installed = append(installed, domain.WhisperCppInstalledRuntime{Version: entry.Name(), CPUInstalled: cpu, CUDAInstalled: cuda})
		}
	}
	sort.Slice(installed, func(i, j int) bool { return installed[i].Version > installed[j].Version })
	return domain.WhisperCppRuntimeStatus{Root: c.root, SelectedVersion: strings.TrimSpace(selectedVersion), Installed: installed}, nil
}

func (c *WhisperCatalog) Install(ctx context.Context, request domain.WhisperCppRuntimeInstallRequest, operationID string) error {
	if goruntime.GOOS != "windows" {
		return errors.New("whisper.cpp runtime installation is currently available on Windows only")
	}
	if !whisperVersionPattern.MatchString(strings.TrimSpace(request.Version)) {
		return errors.New("choose a valid whisper.cpp release")
	}
	if request.Mode != domain.RuntimeCPU && request.Mode != domain.RuntimeCUDA {
		return errors.New("choose either the CPU or CUDA runtime")
	}
	releases, err := c.releases(ctx)
	if err != nil {
		return err
	}
	var selected *domain.WhisperCppRelease
	for index := range releases {
		if releases[index].Version == request.Version {
			selected = &releases[index]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("whisper.cpp release %s is no longer available", request.Version)
	}
	artifact := selected.CPU
	label := "CPU runtime"
	if request.Mode == domain.RuntimeCUDA {
		artifact, label = selected.CUDA, "CUDA runtime"
	}
	if artifact.URL == "" {
		return fmt.Errorf("whisper.cpp release %s does not provide a %s", request.Version, strings.ToLower(label))
	}
	if err := os.MkdirAll(c.root, 0o700); err != nil {
		return fmt.Errorf("create whisper.cpp runtime directory: %w", err)
	}
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whispercpp-install", Stage: "prepare", Message: "Preparing whisper.cpp " + request.Version, Total: artifact.Size})
	archive, err := c.download(ctx, request.Version, request.Mode, label, artifact, operationID)
	if err != nil {
		return err
	}
	if err := c.publish(request, archive, operationID, artifact.Size); err != nil {
		return err
	}
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whispercpp-install", Stage: "complete", Message: "Installed whisper.cpp " + request.Version + " " + strings.ToUpper(string(request.Mode)), Completed: artifact.Size, Total: artifact.Size})
	return nil
}

func (c *WhisperCatalog) download(ctx context.Context, version string, mode domain.RuntimeMode, label string, artifact domain.LlamaCppRuntimeArtifact, operationID string) (string, error) {
	downloads := filepath.Join(c.root, ".downloads")
	if err := os.MkdirAll(downloads, 0o700); err != nil {
		return "", err
	}
	archive := filepath.Join(downloads, version+"-"+string(mode)+".zip")
	if info, err := os.Stat(archive); err == nil && (artifact.Size <= 0 || info.Size() == artifact.Size) && checksumMatches(archive, artifact.SHA256) {
		c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whispercpp-install", Stage: "download", Message: "Verified cached " + label, Completed: info.Size(), Total: artifact.Size})
		return archive, nil
	}
	partial := archive + ".part"
	offset := int64(0)
	if info, err := os.Stat(partial); err == nil {
		offset = info.Size()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "LocalizeDesktop/0.1")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	response, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("download whisper.cpp %s: %w", label, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("whisper.cpp download returned %s", response.Status)
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
				c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whispercpp-install", Stage: "download", Message: "Downloading " + label + " for " + version, Completed: completed, Total: artifact.Size, SpeedBytesPerSecond: speed})
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
	if artifact.Size > 0 && completed != artifact.Size {
		return "", fmt.Errorf("whisper.cpp download size validation failed: received %d bytes, expected %d", completed, artifact.Size)
	}
	if !checksumMatches(partial, artifact.SHA256) {
		return "", errors.New("whisper.cpp download checksum validation failed")
	}
	if err := os.Rename(partial, archive); err != nil {
		return "", err
	}
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whispercpp-install", Stage: "verify", Message: "Verified " + label, Completed: completed, Total: artifact.Size})
	return archive, nil
}

func (c *WhisperCatalog) publish(request domain.WhisperCppRuntimeInstallRequest, archive, operationID string, total int64) error {
	versionRoot := filepath.Join(c.root, request.Version)
	if err := os.MkdirAll(versionRoot, 0o700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(versionRoot, "."+string(request.Mode)+"-install-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whispercpp-install", Stage: "install", Message: "Installing whisper.cpp files", Completed: total, Total: total})
	if err := extractWhisperCLI(archive, staging); err != nil {
		return err
	}
	final := filepath.Join(versionRoot, string(request.Mode))
	backup := final + ".previous"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(final); err == nil {
		if err := os.Rename(final, backup); err != nil {
			return fmt.Errorf("replace existing whisper.cpp runtime: %w", err)
		}
	}
	if err := os.Rename(staging, final); err != nil {
		if _, restoreErr := os.Stat(backup); restoreErr == nil {
			_ = os.Rename(backup, final)
		}
		return fmt.Errorf("publish whisper.cpp runtime: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove previous whisper.cpp runtime: %w", err)
	}
	return nil
}

func (c *WhisperCatalog) releases(ctx context.Context) ([]domain.WhisperCppRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.releasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LocalizeDesktop/0.1")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch whisper.cpp releases: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return nil, fmt.Errorf("whisper.cpp release lookup returned %s", response.Status)
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
			Digest             string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return nil, err
	}
	releases := make([]domain.WhisperCppRelease, 0, len(raw))
	for _, release := range raw {
		if release.Draft || release.Prerelease || !whisperVersionPattern.MatchString(release.TagName) {
			continue
		}
		entry := domain.WhisperCppRelease{Version: release.TagName, PublishedAt: release.PublishedAt}
		cuda := map[string]domain.LlamaCppRuntimeArtifact{}
		for _, asset := range release.Assets {
			artifact := domain.LlamaCppRuntimeArtifact{URL: asset.BrowserDownloadURL, Size: asset.Size, SHA256: strings.TrimPrefix(asset.Digest, "sha256:")}
			switch {
			case whisperCPUAsset.MatchString(asset.Name):
				entry.CPU = artifact
			case len(whisperCUDAAsset.FindStringSubmatch(asset.Name)) > 0:
				match := whisperCUDAAsset.FindStringSubmatch(asset.Name)
				cuda[match[1]] = artifact
			}
		}
		for _, toolkit := range []string{"12.4.0", "11.8.0"} {
			if artifact, ok := cuda[toolkit]; ok {
				entry.CUDA = artifact
				break
			}
		}
		if entry.CPU.URL != "" {
			releases = append(releases, entry)
		}
	}
	if len(releases) == 0 {
		return nil, errors.New("no compatible Windows x64 whisper.cpp releases are currently available")
	}
	return releases, nil
}

func extractWhisperCLI(archive, destination string) error {
	scratch, err := os.MkdirTemp(filepath.Dir(destination), ".extract-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	if err := extractArchive(archive, scratch); err != nil {
		return err
	}
	cli := ""
	err = filepath.WalkDir(scratch, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "whisper-cli.exe") {
			cli = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return err
	}
	if cli == "" {
		return errors.New("whisper.cpp archive did not contain whisper-cli.exe")
	}
	return copyDirectory(filepath.Dir(cli), destination)
}
