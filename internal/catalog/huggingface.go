package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/operations"
)

var validRepository = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)

type HuggingFace struct {
	http       *http.Client
	operations *operations.Hub
	hubURL     string
}

func NewHuggingFace(hub *operations.Hub) *HuggingFace {
	return &HuggingFace{http: &http.Client{Timeout: 0}, operations: hub, hubURL: "https://huggingface.co"}
}

func (h *HuggingFace) Search(ctx context.Context, query string) ([]domain.HuggingFaceModel, error) {
	values := url.Values{"search": []string{strings.TrimSpace(query)}, "filter": []string{"gguf"}, "gated": []string{"false"}, "limit": []string{"24"}, "sort": []string{"downloads"}, "direction": []string{"-1"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.hubURL+"/api/models?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("hugging Face search returned %s", resp.Status)
	}
	var raw []struct {
		ID           string   `json:"id"`
		Author       string   `json:"author"`
		Downloads    int64    `json:"downloads"`
		Likes        int      `json:"likes"`
		LastModified string   `json:"lastModified"`
		Tags         []string `json:"tags"`
		PipelineTag  string   `json:"pipeline_tag"`
		Gated        any      `json:"gated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	models := make([]domain.HuggingFaceModel, 0, len(raw))
	for _, model := range raw {
		if !validRepository.MatchString(model.ID) || gated(model.Gated) {
			continue
		}
		models = append(models, domain.HuggingFaceModel{ID: model.ID, Author: model.Author, Downloads: model.Downloads, Likes: model.Likes, LastModified: model.LastModified, Tags: model.Tags, PipelineTag: model.PipelineTag})
	}
	return models, nil
}

func (h *HuggingFace) Files(ctx context.Context, repository string) ([]domain.HuggingFaceFile, error) {
	if !validRepository.MatchString(repository) {
		return nil, fmt.Errorf("invalid Hugging Face repository")
	}
	// Hugging Face deliberately omits sizes and LFS digests from default model
	// metadata. blobs=true requests the per-file metadata needed for correct
	// size rendering and download verification.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.hubURL+"/api/models/"+repository+"?blobs=true", nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("hugging Face model lookup returned %s", resp.Status)
	}
	var raw struct {
		Gated    any `json:"gated"`
		Siblings []struct {
			Name string `json:"rfilename"`
			Size int64  `json:"size"`
			LFS  struct {
				OID  string `json:"oid"`
				Size int64  `json:"size"`
			} `json:"lfs"`
		} `json:"siblings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if gated(raw.Gated) {
		return nil, fmt.Errorf("this Hugging Face repository is gated and cannot be installed without an account")
	}
	files := make([]domain.HuggingFaceFile, 0, len(raw.Siblings))
	for _, file := range raw.Siblings {
		lower := strings.ToLower(file.Name)
		if !strings.HasSuffix(lower, ".gguf") {
			continue
		}
		size := file.Size
		if file.LFS.Size > 0 {
			size = file.LFS.Size
		}
		files = append(files, domain.HuggingFaceFile{Name: file.Name, Size: size, OID: file.LFS.OID, MMProj: strings.Contains(lower, "mmproj") || strings.Contains(lower, "projector")})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

func (h *HuggingFace) Install(ctx context.Context, request domain.HuggingFaceInstallRequest, modelDir, operationID string) (domain.ModelInfo, error) {
	if !validRepository.MatchString(request.Repository) {
		return domain.ModelInfo{}, fmt.Errorf("invalid Hugging Face repository")
	}
	if !safeFile(request.ModelFile) || (request.MMProjFile != "" && !safeFile(request.MMProjFile)) {
		return domain.ModelInfo{}, fmt.Errorf("invalid Hugging Face file name")
	}
	files, err := h.Files(ctx, request.Repository)
	if err != nil {
		return domain.ModelInfo{}, err
	}
	model, ok := findFile(files, request.ModelFile, false)
	if !ok {
		return domain.ModelInfo{}, fmt.Errorf("selected GGUF model is no longer available")
	}
	var projectionFile domain.HuggingFaceFile
	if request.MMProjFile != "" {
		var projectionOK bool
		projectionFile, projectionOK = findFile(files, request.MMProjFile, true)
		if !projectionOK {
			return domain.ModelInfo{}, fmt.Errorf("selected vision projection is no longer available")
		}
	}
	destination := filepath.Join(modelDir, sanitize(request.Repository), sanitize(request.ModelFile))
	if err := h.download(ctx, request.Repository, request.ModelFile, destination, model.Size, model.OID, operationID); err != nil {
		return domain.ModelInfo{}, err
	}
	projection := ""
	if request.MMProjFile != "" {
		projection = filepath.Join(modelDir, sanitize(request.Repository), sanitize(request.MMProjFile))
		if err := h.download(ctx, request.Repository, request.MMProjFile, projection, projectionFile.Size, projectionFile.OID, operationID); err != nil {
			return domain.ModelInfo{}, err
		}
	}
	info, err := os.Stat(destination)
	if err != nil {
		return domain.ModelInfo{}, err
	}
	relative, err := filepath.Rel(modelDir, destination)
	if err != nil {
		return domain.ModelInfo{}, err
	}
	return domain.ModelInfo{ID: strings.TrimSuffix(filepath.ToSlash(relative), ".gguf"), Name: request.ModelFile, Path: destination, ProjectionPath: projection, Size: info.Size(), SupportsVision: projection != ""}, nil
}

func (h *HuggingFace) download(ctx context.Context, repository, file, destination string, expectedSize int64, expectedOID, operationID string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	if stat, err := os.Stat(destination); err == nil {
		if (expectedSize <= 0 || stat.Size() == expectedSize) && checksumMatches(destination, expectedOID) {
			h.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "huggingface-download", Stage: "complete", Message: "Already installed " + file, Completed: stat.Size(), Total: expectedSize})
			return nil
		}
		return fmt.Errorf("an existing model file has an unexpected size; remove it before retrying")
	}
	partial := destination + ".part"
	var offset int64
	if stat, err := os.Stat(partial); err == nil {
		offset = stat.Size()
	}
	relativeFile := strings.ReplaceAll(file, "\\", "/")
	escapedParts := strings.Split(relativeFile, "/")
	for index := range escapedParts {
		escapedParts[index] = url.PathEscape(escapedParts[index])
	}
	requestURL := h.hubURL + "/" + repository + "/resolve/main/" + strings.Join(escapedParts, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	response, err := h.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("hugging Face download returned %s", response.Status)
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
	defer func() { _ = target.Close() }()
	total := expectedSize
	if total <= 0 && response.ContentLength >= 0 {
		total = response.ContentLength + offset
	}
	buffer := make([]byte, 256*1024)
	completed := offset
	lastEmit := time.Now()
	lastCompleted := completed
	for {
		read, readErr := response.Body.Read(buffer)
		if read > 0 {
			if _, err := target.Write(buffer[:read]); err != nil {
				return err
			}
			completed += int64(read)
			now := time.Now()
			if now.Sub(lastEmit) > 250*time.Millisecond {
				speed := int64(float64(completed-lastCompleted) / now.Sub(lastEmit).Seconds())
				h.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "huggingface-download", Stage: "download", Message: "Downloading " + file, Completed: completed, Total: total, SpeedBytesPerSecond: speed})
				lastEmit, lastCompleted = now, completed
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	if err := target.Close(); err != nil {
		return err
	}
	stat, err := os.Stat(partial)
	if err != nil {
		return err
	}
	if expectedSize > 0 && stat.Size() != expectedSize {
		return fmt.Errorf("download size validation failed: received %d bytes, expected %d", stat.Size(), expectedSize)
	}
	if !checksumMatches(partial, expectedOID) {
		return fmt.Errorf("download checksum validation failed")
	}
	if err := os.Rename(partial, destination); err != nil {
		return err
	}
	h.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "huggingface-download", Stage: "complete", Message: "Installed " + file, Completed: completed, Total: total, SpeedBytesPerSecond: 0})
	return nil
}

func findFile(files []domain.HuggingFaceFile, name string, projection bool) (domain.HuggingFaceFile, bool) {
	for _, file := range files {
		if file.Name == name && file.MMProj == projection {
			return file, true
		}
	}
	return domain.HuggingFaceFile{}, false
}

func gated(value any) bool {
	switch status := value.(type) {
	case bool:
		return status
	case string:
		return strings.TrimSpace(status) != "" && !strings.EqualFold(status, "false")
	default:
		return false
	}
}

func checksumMatches(path, oid string) bool {
	oid = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(oid)), "sha256:")
	if len(oid) != 64 {
		return true // The Hub omitted a verifiable LFS digest; size validation still applies.
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
	return hex.EncodeToString(digest.Sum(nil)) == oid
}

func safeFile(name string) bool {
	return name != "" && !strings.Contains(name, "..") && !strings.HasPrefix(name, "/") && !strings.HasPrefix(name, "\\")
}
func sanitize(value string) string {
	replacer := strings.NewReplacer("/", "__", "\\", "__", ":", "_")
	return replacer.Replace(value)
}
