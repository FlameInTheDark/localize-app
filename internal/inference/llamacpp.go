package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

type LlamaCpp struct {
	baseURL string
	http    *http.Client
}

func NewLlamaCpp(endpoint string) *LlamaCpp {
	return &LlamaCpp{baseURL: strings.TrimRight(endpoint, "/"), http: &http.Client{Timeout: 10 * time.Minute}}
}

func (l *LlamaCpp) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.baseURL+"/v1/models", nil)
	if err != nil {
		return err
	}
	resp, err := l.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("llama.cpp returned %s", resp.Status)
	}
	return nil
}

func (l *LlamaCpp) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	var response struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			Meta    struct {
				Size int64 `json:"size"`
			} `json:"meta"`
		} `json:"data"`
	}
	if err := l.getJSON(ctx, "/v1/models", &response); err != nil {
		return nil, err
	}
	models := make([]domain.ModelInfo, 0, len(response.Data))
	for _, model := range response.Data {
		models = append(models, domain.ModelInfo{ID: model.ID, Name: model.ID, Family: model.OwnedBy, Size: model.Meta.Size})
	}
	return models, nil
}

func (l *LlamaCpp) Generate(ctx context.Context, input Request) (string, error) {
	var messages []map[string]any
	if domain.IsTranslateGemmaModel(input.Model) && input.ImageBase64 == "" {
		// The managed runtime uses the regular Gemma template as a compatibility
		// fallback for TranslateGemma GGUFs. Gemma's fallback template is most
		// reliable with one user message, so keep the saved system instructions
		// inside that message rather than sending an unsupported system role.
		messages = []map[string]any{{"role": "user", "content": compatibilityUserMessage(input.System, input.Prompt)}}
	} else if input.ImageBase64 == "" {
		messages = []map[string]any{{"role": "system", "content": input.System}, {"role": "user", "content": input.Prompt}}
	} else {
		messages = []map[string]any{{"role": "system", "content": input.System}}
		dataURL := "data:" + input.MimeType + ";base64," + input.ImageBase64
		messages = append(messages, map[string]any{"role": "user", "content": []map[string]any{{"type": "image_url", "image_url": map[string]string{"url": dataURL}}, {"type": "text", "text": input.Prompt}}})
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	payload := map[string]any{"model": input.Model, "messages": messages, "stream": false}
	if input.Temperature != nil {
		payload["temperature"] = *input.Temperature
	} else {
		payload["temperature"] = 0.1
	}
	if len(input.Schema) > 0 {
		var schema any
		if err := json.Unmarshal(input.Schema, &schema); err != nil {
			return "", fmt.Errorf("decode response schema: %w", err)
		}
		payload["response_format"] = map[string]any{"type": "json_object", "schema": schema}
	}
	if err := l.postJSON(ctx, "/v1/chat/completions", payload, &response); err != nil {
		return "", err
	}
	if response.Error.Message != "" {
		return "", fmt.Errorf("llama.cpp: %s", response.Error.Message)
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("llama.cpp returned an empty response")
	}
	return response.Choices[0].Message.Content, nil
}

func compatibilityUserMessage(system, prompt string) string {
	return strings.TrimSpace(system) + "\n\nText to process:\n" + prompt
}

func (l *LlamaCpp) getJSON(ctx context.Context, path string, destination any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := l.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("llama.cpp returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(destination)
}
func (l *LlamaCpp) postJSON(ctx context.Context, path string, payload, destination any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := l.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llama.cpp returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return json.NewDecoder(resp.Body).Decode(destination)
}
