package inference

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLlamaCppStructuredGenerationContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			http.NotFound(response, request)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		format, ok := body["response_format"].(map[string]any)
		if !ok || format["type"] != "json_object" {
			t.Fatalf("expected llama.cpp schema response format, got %#v", body["response_format"])
		}
		_, _ = response.Write([]byte(`{"choices":[{"message":{"content":"{\"variants\":[]}"}}]}`))
	}))
	defer server.Close()
	temperature := 0.0
	response, err := NewLlamaCpp(server.URL).Generate(context.Background(), Request{Model: "demo", Prompt: "test", Schema: json.RawMessage(`{"type":"object"}`), Temperature: &temperature})
	if err != nil || response == "" {
		t.Fatalf("unexpected llama.cpp result: %q, %v", response, err)
	}
}

func TestLlamaCppTranslateGemmaCompatibilityUsesOneUserMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" {
			t.Fatalf("expected one compatible user message, got %#v", body.Messages)
		}
		if body.Messages[0].Content != "Translate into French.\n\nText to process:\nHello" {
			t.Fatalf("unexpected compatibility content: %q", body.Messages[0].Content)
		}
		_, _ = response.Write([]byte(`{"choices":[{"message":{"content":"Bonjour"}}]}`))
	}))
	defer server.Close()

	result, err := NewLlamaCpp(server.URL).Generate(context.Background(), Request{
		Model:  "mradermacher__translategemma-4b-it-GGUF",
		System: "Translate into French.",
		Prompt: "Hello",
	})
	if err != nil || result != "Bonjour" {
		t.Fatalf("unexpected compatibility result: %q, %v", result, err)
	}
}
