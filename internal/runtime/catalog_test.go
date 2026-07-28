package runtime

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/operations"
)

func TestLlamaCatalogListsCompatibleWindowsReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`[
          {"tag_name":"b1234","published_at":"2026-01-01T00:00:00Z","assets":[
            {"name":"llama-b1234-bin-win-cpu-x64.zip","browser_download_url":"https://example.test/cpu","size":100,"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
            {"name":"llama-b1234-bin-win-cuda-12.4-x64.zip","browser_download_url":"https://example.test/cuda","size":200,"digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
            {"name":"cudart-llama-bin-win-cuda-12.4-x64.zip","browser_download_url":"https://example.test/cudart","size":20,"digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
            {"name":"llama-b1234-bin-win-vulkan-x64.zip","browser_download_url":"https://example.test/vulkan","size":300,"digest":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
            {"name":"llama-b1234-bin-win-hip-radeon-x64.zip","browser_download_url":"https://example.test/hip","size":400,"digest":"sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}
          ]},
          {"tag_name":"b1233","assets":[{"name":"llama-b1233-bin-ubuntu-x64.tar.gz","browser_download_url":"https://example.test/linux","size":1}]}
        ]`))
	}))
	defer server.Close()
	catalog := NewLlamaCatalog(t.TempDir(), &operations.Hub{})
	catalog.releasesURL, catalog.http = server.URL, server.Client()
	releases, err := catalog.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 || releases[0].Version != "b1234" || releases[0].CPU.Size != 100 || releases[0].CUDA.Size != 200 || releases[0].Vulkan.Size != 300 || releases[0].HIP.Size != 400 {
		t.Fatalf("unexpected releases: %#v", releases)
	}
}

func TestLlamaRuntimeModeAcceptsAllInstallableVariants(t *testing.T) {
	tests := []struct {
		name string
		mode domain.RuntimeMode
		want bool
	}{
		{name: "CPU", mode: domain.RuntimeCPU, want: true},
		{name: "CUDA", mode: domain.RuntimeCUDA, want: true},
		{name: "Vulkan", mode: domain.RuntimeVulkan, want: true},
		{name: "HIP", mode: domain.RuntimeHIP, want: true},
		{name: "auto is selection-only", mode: domain.RuntimeAuto, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := llamaRuntimeMode(test.mode); got != test.want {
				t.Fatalf("llamaRuntimeMode(%q) = %t, want %t", test.mode, got, test.want)
			}
		})
	}
}

func TestLlamaCatalogInstallsCPUArchiveAtomically(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows runtime archive test")
	}
	archive := testArchive(t, map[string]string{"package/llama-server.exe": "server", "package/ggml.dll": "library"})
	digest := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/archive" {
			response.Header().Set("Content-Length", fmt.Sprint(len(archive)))
			_, _ = response.Write(archive)
			return
		}
		_, _ = fmt.Fprintf(response, `[{"tag_name":"b1234","assets":[{"name":"llama-b1234-bin-win-cpu-x64.zip","browser_download_url":"%s/archive","size":%d,"digest":"sha256:%s"}]}]`, serverURL(request), len(archive), hex.EncodeToString(digest[:]))
	}))
	defer server.Close()
	catalog := NewLlamaCatalog(t.TempDir(), &operations.Hub{})
	catalog.releasesURL, catalog.http = server.URL, server.Client()
	if err := catalog.Install(context.Background(), domain.LlamaCppRuntimeInstallRequest{Version: "b1234", Mode: domain.RuntimeCPU}, "test"); err != nil {
		t.Fatal(err)
	}
	status, err := catalog.Status("b1234")
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Installed) != 1 || !status.Installed[0].CPUInstalled {
		t.Fatalf("CPU runtime was not installed: %#v", status)
	}
}

func TestLlamaCatalogInstallsVulkanAndHIPArchivesAtomically(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows runtime archive test")
	}
	archive := testArchive(t, map[string]string{"package/llama-server.exe": "server", "package/ggml.dll": "library"})
	digest := sha256.Sum256(archive)
	tests := []struct {
		name      string
		mode      domain.RuntimeMode
		assetName string
		installed func(domain.LlamaCppInstalledRuntime) bool
	}{
		{name: "Vulkan", mode: domain.RuntimeVulkan, assetName: "llama-b1234-bin-win-vulkan-x64.zip", installed: func(status domain.LlamaCppInstalledRuntime) bool { return status.VulkanInstalled }},
		{name: "HIP", mode: domain.RuntimeHIP, assetName: "llama-b1234-bin-win-hip-radeon-x64.zip", installed: func(status domain.LlamaCppInstalledRuntime) bool { return status.HIPInstalled }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/archive" {
					response.Header().Set("Content-Length", fmt.Sprint(len(archive)))
					_, _ = response.Write(archive)
					return
				}
				_, _ = fmt.Fprintf(response, `[{"tag_name":"b1234","assets":[{"name":"llama-b1234-bin-win-cpu-x64.zip","browser_download_url":"%s/archive","size":%d,"digest":"sha256:%s"},{"name":"%s","browser_download_url":"%s/archive","size":%d,"digest":"sha256:%s"}]}]`, serverURL(request), len(archive), hex.EncodeToString(digest[:]), test.assetName, serverURL(request), len(archive), hex.EncodeToString(digest[:]))
			}))
			defer server.Close()
			catalog := NewLlamaCatalog(t.TempDir(), &operations.Hub{})
			catalog.releasesURL, catalog.http = server.URL, server.Client()
			if err := catalog.Install(context.Background(), domain.LlamaCppRuntimeInstallRequest{Version: "b1234", Mode: test.mode}, "test"); err != nil {
				t.Fatal(err)
			}
			status, err := catalog.Status("b1234")
			if err != nil {
				t.Fatal(err)
			}
			if len(status.Installed) != 1 || !test.installed(status.Installed[0]) {
				t.Fatalf("%s runtime was not installed: %#v", test.name, status)
			}
		})
	}
}

func TestExtractArchiveRejectsTraversal(t *testing.T) {
	archive := testArchive(t, map[string]string{"../escape.txt": "unsafe"})
	path := t.TempDir() + "/runtime.zip"
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := extractArchive(path, t.TempDir()); err == nil {
		t.Fatal("expected archive traversal to be rejected")
	}
}

func testArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, contents := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func serverURL(request *http.Request) string { return "http://" + request.Host }
