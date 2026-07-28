package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FlameInTheDark/localize-app/internal/operations"
)

func TestWhisperModelsUsesLFSSizeAndFiltersSupportedModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/models/ggerganov/whisper.cpp" {
			http.NotFound(response, request)
			return
		}
		_, _ = response.Write([]byte(`{"siblings":[
          {"rfilename":"ggml-small.bin","size":133,"lfs":{"oid":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":487601967}},
          {"rfilename":"ggml-small.en.bin","size":134,"lfs":{"oid":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","size":487614201}},
          {"rfilename":"ggml-small.en-tdrz.bin","size":1,"lfs":{"size":2}},
          {"rfilename":"README.md","size":1}
        ]}`))
	}))
	defer server.Close()
	models := NewWhisperModels(&operations.Hub{})
	models.http, models.hubURL = server.Client(), server.URL
	items, err := models.List(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Size != 487601967 || !items[0].Multilingual || items[1].Multilingual {
		t.Fatalf("unexpected Whisper model list: %#v", items)
	}
}

func TestWhisperModelDeleteRejectsTraversal(t *testing.T) {
	models := NewWhisperModels(&operations.Hub{})
	if err := models.Delete(t.TempDir(), "../escape"); err == nil {
		t.Fatal("expected traversal model ID to be rejected")
	}
}
