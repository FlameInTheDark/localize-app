package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/operations"
)

const whisperModelRepository = "ggerganov/whisper.cpp"

// WhisperModels exposes the public, official whisper.cpp model repository.
// It deliberately does not support arbitrary repositories: runtime/model
// compatibility and model checksums stay predictable for desktop users.
type WhisperModels struct {
	http       *http.Client
	operations *operations.Hub
	hubURL     string
}

func NewWhisperModels(hub *operations.Hub) *WhisperModels {
	return &WhisperModels{http: &http.Client{Timeout: 0}, operations: hub, hubURL: "https://huggingface.co"}
}

func (m *WhisperModels) List(ctx context.Context, root string) ([]domain.WhisperModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.hubURL+"/api/models/"+whisperModelRepository+"?blobs=true", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "LocalizeDesktop/0.1")
	response, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch whisper models: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		return nil, fmt.Errorf("whisper model lookup returned %s", response.Status)
	}
	var payload struct {
		Siblings []struct {
			Name string `json:"rfilename"`
			Size int64  `json:"size"`
			LFS  struct {
				OID  string `json:"oid"`
				Size int64  `json:"size"`
			} `json:"lfs"`
		} `json:"siblings"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	models := make([]domain.WhisperModel, 0)
	for _, sibling := range payload.Siblings {
		name := strings.TrimSpace(sibling.Name)
		lower := strings.ToLower(name)
		if !strings.HasPrefix(lower, "ggml-") || !strings.HasSuffix(lower, ".bin") || strings.Contains(lower, "tdrz") || strings.Contains(name, "/") {
			continue
		}
		size := sibling.Size
		if sibling.LFS.Size > 0 {
			size = sibling.LFS.Size
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, "ggml-"), ".bin")
		path := filepath.Join(root, name)
		info, statErr := os.Stat(path)
		// Full SHA-256 verification happens before atomic publishing. Rehashing a
		// multi-gigabyte model whenever the Settings view refreshes would make
		// the desktop UI unresponsive, so status uses the verified file size.
		installed := statErr == nil && !info.IsDir() && (size == 0 || info.Size() == size)
		models = append(models, domain.WhisperModel{ID: id, Name: name, Path: path, Size: size, SHA256: sibling.LFS.OID, Installed: installed, Multilingual: !strings.Contains(strings.ToLower(id), ".en")})
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].Size == models[j].Size {
			return models[i].Name < models[j].Name
		}
		return models[i].Size < models[j].Size
	})
	return models, nil
}

func (m *WhisperModels) Install(ctx context.Context, request domain.WhisperModelInstallRequest, root, operationID string) (domain.WhisperModel, error) {
	if strings.TrimSpace(request.ID) == "" {
		return domain.WhisperModel{}, errors.New("choose a Whisper model")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return domain.WhisperModel{}, fmt.Errorf("create Whisper model directory: %w", err)
	}
	models, err := m.List(ctx, root)
	if err != nil {
		return domain.WhisperModel{}, err
	}
	var selected *domain.WhisperModel
	for index := range models {
		if models[index].ID == request.ID {
			selected = &models[index]
			break
		}
	}
	if selected == nil {
		return domain.WhisperModel{}, errors.New("selected Whisper model is no longer available")
	}
	if selected.Installed {
		return *selected, nil
	}
	if err := m.download(ctx, selected.Name, selected.Path, selected.Size, selected.SHA256, operationID); err != nil {
		return domain.WhisperModel{}, err
	}
	info, err := os.Stat(selected.Path)
	if err != nil {
		return domain.WhisperModel{}, err
	}
	selected.Size, selected.Installed = info.Size(), true
	return *selected, nil
}

func (m *WhisperModels) Delete(root, id string) error {
	if strings.TrimSpace(id) == "" || strings.Contains(id, "..") || strings.ContainsAny(id, `\\/`) {
		return errors.New("invalid Whisper model")
	}
	path := filepath.Clean(filepath.Join(root, "ggml-"+id+".bin"))
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return errors.New("invalid Whisper model")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove Whisper model: %w", err)
	}
	return nil
}

func (m *WhisperModels) download(ctx context.Context, file, destination string, expectedSize int64, expectedSHA, operationID string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	if stat, err := os.Stat(destination); err == nil {
		if (expectedSize <= 0 || stat.Size() == expectedSize) && checksumMatches(destination, expectedSHA) {
			m.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whisper-model-download", Stage: "complete", Message: "Already installed " + file, Completed: stat.Size(), Total: expectedSize})
			return nil
		}
		return errors.New("an existing Whisper model file has an unexpected size; remove it before retrying")
	}
	partial := destination + ".part"
	offset := int64(0)
	if stat, err := os.Stat(partial); err == nil {
		offset = stat.Size()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.hubURL+"/"+whisperModelRepository+"/resolve/main/"+url.PathEscape(file), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LocalizeDesktop/0.1")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	response, err := m.http.Do(req)
	if err != nil {
		return fmt.Errorf("download Whisper model: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("whisper model download returned %s", response.Status)
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
		return err
	}
	completed, lastCompleted, lastEmit := offset, offset, time.Now()
	buffer := make([]byte, 256*1024)
	for {
		read, readErr := response.Body.Read(buffer)
		if read > 0 {
			if _, writeErr := target.Write(buffer[:read]); writeErr != nil {
				_ = target.Close()
				return writeErr
			}
			completed += int64(read)
			now := time.Now()
			if now.Sub(lastEmit) >= 250*time.Millisecond {
				speed := int64(float64(completed-lastCompleted) / now.Sub(lastEmit).Seconds())
				m.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whisper-model-download", Stage: "download", Message: "Downloading " + file, Completed: completed, Total: expectedSize, SpeedBytesPerSecond: speed})
				lastEmit, lastCompleted = now, completed
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = target.Close()
			return readErr
		}
	}
	if err := target.Close(); err != nil {
		return err
	}
	if stat, err := os.Stat(partial); err != nil {
		return err
	} else if expectedSize > 0 && stat.Size() != expectedSize {
		return fmt.Errorf("whisper model download size validation failed: received %d bytes, expected %d", stat.Size(), expectedSize)
	}
	if !checksumMatches(partial, expectedSHA) {
		return errors.New("whisper model download checksum validation failed")
	}
	if err := os.Rename(partial, destination); err != nil {
		return err
	}
	m.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whisper-model-download", Stage: "complete", Message: "Installed " + file, Completed: completed, Total: expectedSize})
	return nil
}
