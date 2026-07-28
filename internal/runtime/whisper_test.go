package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/operations"
)

func TestWhisperCatalogListsOfficialWindowsArtifacts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`[
          {"tag_name":"v1.9.1","published_at":"2026-06-19T00:00:00Z","assets":[
            {"name":"whisper-bin-x64.zip","browser_download_url":"https://example.test/cpu","size":7982101,"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
            {"name":"whisper-cublas-11.8.0-bin-x64.zip","browser_download_url":"https://example.test/cuda118","size":278557654,"digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
            {"name":"whisper-cublas-12.4.0-bin-x64.zip","browser_download_url":"https://example.test/cuda124","size":677887125,"digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}
          ]},
          {"tag_name":"v1.9.0","prerelease":true,"assets":[{"name":"whisper-bin-x64.zip","browser_download_url":"https://example.test/ignored","size":1}]}
        ]`))
	}))
	defer server.Close()
	catalog := NewWhisperCatalog(t.TempDir(), &operations.Hub{})
	catalog.releasesURL, catalog.http = server.URL, server.Client()
	releases, err := catalog.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 || releases[0].Version != "v1.9.1" || releases[0].CPU.Size != 7982101 || releases[0].CUDA.Size != 677887125 || releases[0].CUDA.URL != "https://example.test/cuda124" {
		t.Fatalf("unexpected releases: %#v", releases)
	}
}

func TestParseWhisperResult(t *testing.T) {
	result, err := parseWhisperResult([]byte(`{"result":{"language":"ru","transcription":[{"text":" Привет"},{"text":"мир "}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Language != "ru" || result.Text != "Привет мир" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseWhisperResultRejectsEmptySpeech(t *testing.T) {
	if _, err := parseWhisperResult([]byte(`{"result":{"transcription":[]}}`)); err == nil {
		t.Fatal("expected empty speech to be rejected")
	}
}

func TestWhisperArgumentsSuppressNonSpeechForEveryRuntime(t *testing.T) {
	for _, mode := range []domain.RuntimeMode{domain.RuntimeCPU, domain.RuntimeCUDA} {
		t.Run(string(mode), func(t *testing.T) {
			args := whisperArguments("model.bin", "input.wav", "result", "auto", mode)
			if !slices.Contains(args, "-sns") || !slices.Contains(args, "-nth") || !slices.Contains(args, "0.35") {
				t.Fatalf("expected silence-suppression arguments, got %#v", args)
			}
			if gotCPUFlag := slices.Contains(args, "-ng"); gotCPUFlag != (mode == domain.RuntimeCPU) {
				t.Fatalf("unexpected GPU flag for %s: %#v", mode, args)
			}
		})
	}
}
