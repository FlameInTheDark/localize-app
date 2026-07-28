package runtime

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/operations"
)

const githubReleasesURL = "https://api.github.com/repos/ggml-org/llama.cpp/releases?per_page=16"

var (
	versionPattern = regexp.MustCompile(`^b[0-9]+$`)
	cpuAsset       = regexp.MustCompile(`^llama-(b[0-9]+)-bin-win-cpu-x64\.zip$`)
	cudaAsset      = regexp.MustCompile(`^llama-(b[0-9]+)-bin-win-cuda-(12\.4|13\.3)-x64\.zip$`)
	cudartAsset    = regexp.MustCompile(`^cudart-llama-bin-win-cuda-(12\.4|13\.3)-x64\.zip$`)
	vulkanAsset    = regexp.MustCompile(`^llama-(b[0-9]+)-bin-win-vulkan-x64\.zip$`)
	hipAsset       = regexp.MustCompile(`^llama-(b[0-9]+)-bin-win-hip-radeon-x64\.zip$`)
)

// LlamaCatalog discovers official Windows runtime releases and installs them
// into a user-owned directory. It never writes into the installation folder,
// so development and installed builds use exactly the same runtime lifecycle.
type LlamaCatalog struct {
	root        string
	http        *http.Client
	operations  *operations.Hub
	releasesURL string
}

func NewLlamaCatalog(root string, hub *operations.Hub) *LlamaCatalog {
	return &LlamaCatalog{
		root:        root,
		http:        &http.Client{Timeout: 0},
		operations:  hub,
		releasesURL: githubReleasesURL,
	}
}

type releaseManifest struct {
	release domain.LlamaCppRelease
	cudart  domain.LlamaCppRuntimeArtifact
}

func (c *LlamaCatalog) List(ctx context.Context) ([]domain.LlamaCppRelease, error) {
	manifests, err := c.releases(ctx)
	if err != nil {
		return nil, err
	}
	releases := make([]domain.LlamaCppRelease, 0, len(manifests))
	for _, manifest := range manifests {
		releases = append(releases, manifest.release)
	}
	return releases, nil
}

func (c *LlamaCatalog) Status(selectedVersion string) (domain.LlamaCppRuntimeStatus, error) {
	if err := os.MkdirAll(c.root, 0o700); err != nil {
		return domain.LlamaCppRuntimeStatus{}, fmt.Errorf("create llama.cpp runtime directory: %w", err)
	}
	entries, err := os.ReadDir(c.root)
	if err != nil {
		return domain.LlamaCppRuntimeStatus{}, fmt.Errorf("read llama.cpp runtime directory: %w", err)
	}
	installed := make([]domain.LlamaCppInstalledRuntime, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !versionPattern.MatchString(entry.Name()) {
			continue
		}
		versionRoot := filepath.Join(c.root, entry.Name())
		cpu := fileExists(filepath.Join(versionRoot, string(domain.RuntimeCPU), "llama-server.exe"))
		cuda := fileExists(filepath.Join(versionRoot, string(domain.RuntimeCUDA), "llama-server.exe"))
		vulkan := fileExists(filepath.Join(versionRoot, string(domain.RuntimeVulkan), "llama-server.exe"))
		hip := fileExists(filepath.Join(versionRoot, string(domain.RuntimeHIP), "llama-server.exe"))
		if cpu || cuda || vulkan || hip {
			installed = append(installed, domain.LlamaCppInstalledRuntime{Version: entry.Name(), CPUInstalled: cpu, CUDAInstalled: cuda, VulkanInstalled: vulkan, HIPInstalled: hip})
		}
	}
	sort.Slice(installed, func(i, j int) bool { return installed[i].Version > installed[j].Version })
	return domain.LlamaCppRuntimeStatus{Root: c.root, SelectedVersion: strings.TrimSpace(selectedVersion), Installed: installed}, nil
}

func (c *LlamaCatalog) Install(ctx context.Context, request domain.LlamaCppRuntimeInstallRequest, operationID string) error {
	if runtime.GOOS != "windows" {
		return errors.New("llama.cpp runtime installation is currently available on Windows only")
	}
	if !versionPattern.MatchString(strings.TrimSpace(request.Version)) {
		return errors.New("choose a valid llama.cpp release")
	}
	if !llamaRuntimeMode(request.Mode) {
		return errors.New("choose the CPU, CUDA, Vulkan, or HIP runtime")
	}
	manifests, err := c.releases(ctx)
	if err != nil {
		return err
	}
	var selected *releaseManifest
	for index := range manifests {
		if manifests[index].release.Version == request.Version {
			selected = &manifests[index]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("llama.cpp release %s is no longer available", request.Version)
	}
	artifacts := []namedArtifact{{name: "CPU runtime", artifact: selected.release.CPU}}
	switch request.Mode {
	case domain.RuntimeCUDA:
		if selected.release.CUDA.URL == "" || selected.cudart.URL == "" {
			return fmt.Errorf("llama.cpp release %s does not provide a complete CUDA runtime", request.Version)
		}
		artifacts = []namedArtifact{{name: "CUDA runtime", artifact: selected.release.CUDA}, {name: "CUDA libraries", artifact: selected.cudart}}
	case domain.RuntimeVulkan:
		if selected.release.Vulkan.URL == "" {
			return fmt.Errorf("llama.cpp release %s does not provide a Vulkan runtime", request.Version)
		}
		artifacts = []namedArtifact{{name: "Vulkan runtime", artifact: selected.release.Vulkan}}
	case domain.RuntimeHIP:
		if selected.release.HIP.URL == "" {
			return fmt.Errorf("llama.cpp release %s does not provide an AMD HIP runtime", request.Version)
		}
		artifacts = []namedArtifact{{name: "AMD HIP runtime", artifact: selected.release.HIP}}
	}
	total := int64(0)
	for _, item := range artifacts {
		total += item.artifact.Size
	}
	if err := os.MkdirAll(c.root, 0o700); err != nil {
		return fmt.Errorf("create llama.cpp runtime directory: %w", err)
	}
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "llamacpp-install", Stage: "prepare", Message: "Preparing llama.cpp " + request.Version, Total: total})
	archives := make([]string, 0, len(artifacts))
	completedBase := int64(0)
	for _, item := range artifacts {
		archive, err := c.download(ctx, request.Version, item, operationID, completedBase, total)
		if err != nil {
			return err
		}
		archives = append(archives, archive)
		completedBase += item.artifact.Size
	}
	if err := c.publish(ctx, request, archives, operationID, total); err != nil {
		return err
	}
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "llamacpp-install", Stage: "complete", Message: "Installed llama.cpp " + request.Version + " " + strings.ToUpper(string(request.Mode)), Completed: total, Total: total})
	return nil
}

type namedArtifact struct {
	name     string
	artifact domain.LlamaCppRuntimeArtifact
}

func (c *LlamaCatalog) download(ctx context.Context, version string, item namedArtifact, operationID string, base, total int64) (string, error) {
	if item.artifact.URL == "" {
		return "", fmt.Errorf("llama.cpp %s download URL is unavailable", item.name)
	}
	downloads := filepath.Join(c.root, ".downloads")
	if err := os.MkdirAll(downloads, 0o700); err != nil {
		return "", err
	}
	archive := filepath.Join(downloads, version+"-"+safeArchivePart(item.name)+".zip")
	if info, err := os.Stat(archive); err == nil && (item.artifact.Size <= 0 || info.Size() == item.artifact.Size) && checksumMatches(archive, item.artifact.SHA256) {
		c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "llamacpp-install", Stage: "download", Message: "Verified cached " + item.name, Completed: base + info.Size(), Total: total})
		return archive, nil
	}
	partial := archive + ".part"
	offset := int64(0)
	if info, err := os.Stat(partial); err == nil {
		offset = info.Size()
		if (item.artifact.Size <= 0 || offset == item.artifact.Size) && checksumMatches(partial, item.artifact.SHA256) {
			if err := os.Rename(partial, archive); err != nil {
				return "", err
			}
			c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "llamacpp-install", Stage: "verify", Message: "Verified resumed " + item.name, Completed: base + offset, Total: total})
			return archive, nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.artifact.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "LocalizeDesktop/0.1")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	response, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", item.name, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("llama.cpp download returned %s", response.Status)
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
				c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "llamacpp-install", Stage: "download", Message: "Downloading " + item.name + " for " + version, Completed: base + completed, Total: total, SpeedBytesPerSecond: speed})
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
	if item.artifact.Size > 0 && completed != item.artifact.Size {
		return "", fmt.Errorf("%s download size validation failed: received %d bytes, expected %d", item.name, completed, item.artifact.Size)
	}
	if !checksumMatches(partial, item.artifact.SHA256) {
		return "", fmt.Errorf("%s download checksum validation failed", item.name)
	}
	if err := os.Rename(partial, archive); err != nil {
		return "", err
	}
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "llamacpp-install", Stage: "verify", Message: "Verified " + item.name, Completed: base + completed, Total: total})
	return archive, nil
}

func (c *LlamaCatalog) publish(ctx context.Context, request domain.LlamaCppRuntimeInstallRequest, archives []string, operationID string, total int64) error {
	versionRoot := filepath.Join(c.root, request.Version)
	if err := os.MkdirAll(versionRoot, 0o700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(versionRoot, "."+string(request.Mode)+"-install-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	c.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "llamacpp-install", Stage: "install", Message: "Installing llama.cpp files", Completed: total, Total: total})
	if err := extractServer(archives[0], staging); err != nil {
		return err
	}
	for _, archive := range archives[1:] {
		if err := copyArchiveFiles(archive, staging); err != nil {
			return err
		}
	}
	if !fileExists(filepath.Join(staging, "llama-server.exe")) {
		return errors.New("llama.cpp archive did not contain llama-server.exe")
	}
	final := filepath.Join(versionRoot, string(request.Mode))
	backup := final + ".previous"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(final); err == nil {
		if err := os.Rename(final, backup); err != nil {
			return fmt.Errorf("replace existing llama.cpp runtime: %w", err)
		}
	}
	if err := os.Rename(staging, final); err != nil {
		if _, restoreErr := os.Stat(backup); restoreErr == nil {
			_ = os.Rename(backup, final)
		}
		return fmt.Errorf("publish llama.cpp runtime: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove previous llama.cpp runtime: %w", err)
	}
	return nil
}

func (c *LlamaCatalog) releases(ctx context.Context) ([]releaseManifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.releasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LocalizeDesktop/0.1")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch llama.cpp releases: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return nil, fmt.Errorf("llama.cpp release lookup returned %s", response.Status)
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
	manifests := make([]releaseManifest, 0, len(raw))
	for _, release := range raw {
		if release.Draft || release.Prerelease || !versionPattern.MatchString(release.TagName) {
			continue
		}
		manifest := releaseManifest{release: domain.LlamaCppRelease{Version: release.TagName, PublishedAt: release.PublishedAt}}
		cudaServers := map[string]domain.LlamaCppRuntimeArtifact{}
		cudaLibraries := map[string]domain.LlamaCppRuntimeArtifact{}
		for _, asset := range release.Assets {
			artifact := domain.LlamaCppRuntimeArtifact{URL: asset.BrowserDownloadURL, Size: asset.Size, SHA256: strings.TrimPrefix(asset.Digest, "sha256:")}
			cpuMatch := cpuAsset.FindStringSubmatch(asset.Name)
			cudaMatch := cudaAsset.FindStringSubmatch(asset.Name)
			cudartMatch := cudartAsset.FindStringSubmatch(asset.Name)
			vulkanMatch := vulkanAsset.FindStringSubmatch(asset.Name)
			hipMatch := hipAsset.FindStringSubmatch(asset.Name)
			switch {
			case len(cpuMatch) > 0 && cpuMatch[1] == release.TagName:
				manifest.release.CPU = artifact
			case len(cudaMatch) > 0 && cudaMatch[1] == release.TagName:
				cudaServers[cudaMatch[2]] = artifact
			case len(cudartMatch) > 0:
				cudaLibraries[cudartMatch[1]] = artifact
			case len(vulkanMatch) > 0 && vulkanMatch[1] == release.TagName:
				manifest.release.Vulkan = artifact
			case len(hipMatch) > 0 && hipMatch[1] == release.TagName:
				manifest.release.HIP = artifact
			}
		}
		// CUDA 12.4 has the broadest driver support. Only expose it when its
		// matching redistributable archive is present; never mix CUDA DLL sets.
		for _, toolkit := range []string{"12.4", "13.3"} {
			if server, ok := cudaServers[toolkit]; ok {
				if libraries, libraryOK := cudaLibraries[toolkit]; libraryOK {
					manifest.release.CUDA, manifest.cudart = server, libraries
					break
				}
			}
		}
		if manifest.release.CPU.URL != "" {
			manifests = append(manifests, manifest)
		}
	}
	if len(manifests) == 0 {
		return nil, errors.New("no compatible Windows x64 llama.cpp releases are currently available")
	}
	return manifests, nil
}

func llamaRuntimeMode(mode domain.RuntimeMode) bool {
	switch mode {
	case domain.RuntimeCPU, domain.RuntimeCUDA, domain.RuntimeVulkan, domain.RuntimeHIP:
		return true
	default:
		return false
	}
}

func extractServer(archive, destination string) error {
	scratch, err := os.MkdirTemp(filepath.Dir(destination), ".extract-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	if err := extractArchive(archive, scratch); err != nil {
		return err
	}
	server := ""
	err = filepath.WalkDir(scratch, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "llama-server.exe") {
			server = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return err
	}
	if server == "" {
		return errors.New("llama.cpp archive did not contain llama-server.exe")
	}
	return copyDirectory(filepath.Dir(server), destination)
}

func copyArchiveFiles(archive, destination string) error {
	scratch, err := os.MkdirTemp(filepath.Dir(destination), ".extract-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	if err := extractArchive(archive, scratch); err != nil {
		return err
	}
	return filepath.WalkDir(scratch, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		return copyFile(path, filepath.Join(destination, entry.Name()))
	})
}

func extractArchive(archive, destination string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open llama.cpp archive: %w", err)
	}
	defer func() { _ = reader.Close() }()
	for _, file := range reader.File {
		name := filepath.Clean(file.Name)
		if filepath.IsAbs(name) || name == "." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe path in llama.cpp archive: %s", file.Name)
		}
		target := filepath.Join(destination, name)
		relative, err := filepath.Rel(destination, target)
		if err != nil || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
			return fmt.Errorf("unsafe path in llama.cpp archive: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		if err := copyStream(source, target); err != nil {
			_ = source.Close()
			return err
		}
		if err := source.Close(); err != nil {
			return err
		}
	}
	return nil
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		return copyFile(path, target)
	})
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	return copyStream(input, destination)
}

func copyStream(source io.Reader, destination string) error {
	target, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func checksumMatches(path, expected string) bool {
	expected = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(expected)), "sha256:")
	if expected == "" {
		return true
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(expected) {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return false
	}
	return hex.EncodeToString(digest.Sum(nil)) == expected
}

func safeArchivePart(name string) string {
	return strings.NewReplacer(" ", "-", "/", "-", "\\", "-").Replace(strings.ToLower(name))
}
