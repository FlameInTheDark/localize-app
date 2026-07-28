package inference

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

func TestOllamaContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/version":
			_, _ = response.Write([]byte(`{"version":"0.1"}`))
		case "/api/tags":
			_, _ = response.Write([]byte(`{"models":[{"name":"demo:latest","model":"demo","size":12,"details":{"family":"llama","parameter_size":"1B","quantization_level":"Q4"}}]}`))
		case "/api/ps":
			_, _ = response.Write([]byte(`{"models":[{"name":"demo:latest"}]}`))
		case "/api/show":
			_, _ = response.Write([]byte(`{"capabilities":["completion","vision"]}`))
		case "/api/generate":
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body["model"] != "demo:latest" || body["stream"] != false {
				t.Errorf("unexpected generation request: %#v", body)
			}
			_, _ = response.Write([]byte(`{"response":"translated"}`))
		case "/api/pull":
			_, _ = response.Write([]byte("{\"status\":\"pulling\",\"completed\":3,\"total\":10}\n{\"status\":\"success\",\"completed\":10,\"total\":10}\n"))
		case "/api/delete":
			response.WriteHeader(http.StatusOK)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	client := NewOllama(server.URL)
	ctx := context.Background()
	if err := client.Health(ctx); err != nil {
		t.Fatal(err)
	}
	models, err := client.ListModels(ctx)
	if err != nil || len(models) != 1 || models[0].Family != "llama" || !models[0].Running || !models[0].SupportsVision {
		t.Fatalf("unexpected models: %#v, %v", models, err)
	}
	text, err := client.Generate(ctx, Request{Model: "demo:latest", Prompt: "hello"})
	if err != nil || text != "translated" {
		t.Fatalf("unexpected generation: %q, %v", text, err)
	}
	var progress []domain.OperationProgress
	if err := client.Pull(ctx, "demo:latest", "operation", func(update domain.OperationProgress) { progress = append(progress, update) }); err != nil {
		t.Fatal(err)
	}
	if len(progress) != 2 || progress[1].Completed != 10 {
		t.Fatalf("unexpected pull progress: %#v", progress)
	}
	if err := client.Delete(ctx, "demo:latest"); err != nil {
		t.Fatal(err)
	}
}

func TestOllamaStructuredGenerationContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/generate" {
			http.NotFound(response, request)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["format"].(map[string]any); !ok {
			t.Fatalf("expected schema format, got %#v", body["format"])
		}
		options, ok := body["options"].(map[string]any)
		if !ok || options["temperature"] != float64(0) {
			t.Fatalf("expected deterministic options, got %#v", body["options"])
		}
		_, _ = response.Write([]byte(`{"response":"{\"variants\":[]}"}`))
	}))
	defer server.Close()
	temperature := 0.0
	_, err := NewOllama(server.URL).Generate(context.Background(), Request{Model: "demo", Prompt: "test", Schema: json.RawMessage(`{"type":"object"}`), Temperature: &temperature})
	if err != nil {
		t.Fatal(err)
	}
}
