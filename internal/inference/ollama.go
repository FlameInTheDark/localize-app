package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

type Ollama struct {
	baseURL string
	http    *http.Client
}

func NewOllama(endpoint string) *Ollama {
	return &Ollama{baseURL: strings.TrimRight(endpoint, "/"), http: &http.Client{Timeout: 30 * time.Second}}
}

func (o *Ollama) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+"/api/version", nil)
	if err != nil {
		return err
	}
	resp, err := o.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("ollama returned %s", resp.Status)
	}
	return nil
}

func (o *Ollama) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	var response struct {
		Models []struct {
			Name       string    `json:"name"`
			Model      string    `json:"model"`
			Size       int64     `json:"size"`
			ModifiedAt time.Time `json:"modified_at"`
			Details    struct {
				Family        string `json:"family"`
				ParameterSize string `json:"parameter_size"`
				Quantization  string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := o.getJSON(ctx, "/api/tags", &response); err != nil {
		return nil, err
	}
	running, _ := o.runningModels(ctx)
	models := make([]domain.ModelInfo, 0, len(response.Models))
	for _, model := range response.Models {
		vision := knownVisionModel(model.Name)
		if capabilities, err := o.capabilities(ctx, model.Name); err == nil {
			vision = vision || capabilities["vision"]
		}
		models = append(models, domain.ModelInfo{ID: model.Name, Name: model.Model, Size: model.Size, ModifiedAt: model.ModifiedAt.Format(time.RFC3339), Family: model.Details.Family, Parameters: model.Details.ParameterSize, Quantization: model.Details.Quantization, SupportsVision: vision, Running: running[model.Name]})
	}
	return models, nil
}

func (o *Ollama) Delete(ctx context.Context, model string) error {
	body, err := json.Marshal(map[string]string{"model": model})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, o.baseURL+"/api/delete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := o.http.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode/100 != 2 {
		data, _ := io.ReadAll(response.Body)
		return fmt.Errorf("ollama delete: %s", strings.TrimSpace(string(data)))
	}
	return nil
}

func (o *Ollama) EnsureModel(ctx context.Context, model string) error {
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("select an Ollama model in Settings")
	}
	var response struct {
		ModelInfo json.RawMessage `json:"model_info"`
	}
	if err := o.postJSON(ctx, "/api/show", map[string]string{"model": model}, &response); err != nil {
		return fmt.Errorf("configured Ollama model %q is unavailable: %w", model, err)
	}
	return nil
}

func (o *Ollama) Generate(ctx context.Context, input Request) (string, error) {
	payload := map[string]any{"model": input.Model, "prompt": input.Prompt, "system": input.System, "stream": false, "keep_alive": "5m"}
	if len(input.Schema) > 0 {
		var schema any
		if err := json.Unmarshal(input.Schema, &schema); err != nil {
			return "", fmt.Errorf("decode response schema: %w", err)
		}
		payload["format"] = schema
	}
	if input.Temperature != nil {
		payload["options"] = map[string]float64{"temperature": *input.Temperature}
	}
	if input.ImageBase64 != "" {
		payload["images"] = []string{input.ImageBase64}
	}
	var response struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := o.postJSON(ctx, "/api/generate", payload, &response); err != nil {
		return "", err
	}
	if response.Error != "" {
		return "", fmt.Errorf("ollama: %s", response.Error)
	}
	if strings.TrimSpace(response.Response) == "" {
		return "", fmt.Errorf("ollama returned an empty response")
	}
	return response.Response, nil
}

func (o *Ollama) Pull(ctx context.Context, model, operationID string, emit func(domain.OperationProgress)) error {
	body, err := json.Marshal(map[string]any{"model": model, "stream": true})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama pull: %s", strings.TrimSpace(string(data)))
	}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var status struct {
			Status    string `json:"status"`
			Completed int64  `json:"completed"`
			Total     int64  `json:"total"`
			Error     string `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &status); err != nil {
			continue
		}
		if status.Error != "" {
			return fmt.Errorf("ollama pull: %s", status.Error)
		}
		emit(domain.OperationProgress{OperationID: operationID, Kind: "ollama-pull", Stage: "download", Message: status.Status, Completed: status.Completed, Total: status.Total})
	}
	return scanner.Err()
}

func (o *Ollama) getJSON(ctx context.Context, path string, destination any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := o.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("ollama returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(destination)
}

func (o *Ollama) runningModels(ctx context.Context) (map[string]bool, error) {
	var response struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := o.getJSON(ctx, "/api/ps", &response); err != nil {
		return nil, err
	}
	running := make(map[string]bool, len(response.Models))
	for _, model := range response.Models {
		running[model.Name] = true
	}
	return running, nil
}

func (o *Ollama) capabilities(ctx context.Context, model string) (map[string]bool, error) {
	var response struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := o.postJSON(ctx, "/api/show", map[string]string{"model": model}, &response); err != nil {
		return nil, err
	}
	capabilities := make(map[string]bool, len(response.Capabilities))
	for _, capability := range response.Capabilities {
		capabilities[strings.ToLower(capability)] = true
	}
	return capabilities, nil
}

func knownVisionModel(model string) bool {
	name := strings.ToLower(model)
	for _, marker := range []string{"vision", "ocr", "llava", "minicpm-v", "qwen-vl", "moondream", "gemma3"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func (o *Ollama) postJSON(ctx context.Context, path string, payload, destination any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return json.NewDecoder(resp.Body).Decode(destination)
}

func NormalizeEndpoint(endpoint string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid Ollama endpoint")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
