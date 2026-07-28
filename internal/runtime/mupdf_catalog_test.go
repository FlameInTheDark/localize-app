package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/localize-app/internal/operations"
)

func TestMuPDFCatalogListsStableWindowsReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != "application/vnd.github+json" {
			t.Fatalf("missing GitHub accept header")
		}
		_, _ = writer.Write([]byte(`[
  {"tag_name":"1.28.0","published_at":"2026-06-26T09:13:23Z","assets":[{"name":"mupdf-1.28.0-windows.zip","browser_download_url":"https://example.test/1.28.0.zip","size":94238502}]},
  {"tag_name":"1.28.0-rc2","published_at":"2026-06-20T12:28:01Z","assets":[{"name":"mupdf-1.28.0-rc2-windows.zip","browser_download_url":"https://example.test/rc.zip","size":1}]},
  {"tag_name":"1.27.2","published_at":"2026-02-20T12:33:00Z","assets":[{"name":"mupdf-1.27.2-source.tar.gz","browser_download_url":"https://example.test/source.tar.gz","size":1}]}
]`))
	}))
	defer server.Close()

	catalog := NewMuPDFCatalog(t.TempDir(), &operations.Hub{})
	catalog.releasesURL = server.URL
	releases, err := catalog.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 {
		t.Fatalf("got %d compatible releases, want 1: %#v", len(releases), releases)
	}
	if release := releases[0]; release.Version != "1.28.0" || release.Artifact.Size != 94238502 || release.Artifact.URL != "https://example.test/1.28.0.zip" {
		t.Fatalf("unexpected release: %#v", release)
	}
}

func TestMuPDFCatalogFindsOnlyManagedSelectedRuntime(t *testing.T) {
	root := t.TempDir()
	mutool := filepath.Join(root, "1.28.0", "mutool.exe")
	if err := os.MkdirAll(filepath.Dir(mutool), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mutool, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog := NewMuPDFCatalog(root, &operations.Hub{})
	status, err := catalog.Status("1.28.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Installed) != 1 || status.Installed[0].Version != "1.28.0" {
		t.Fatalf("unexpected installed status: %#v", status)
	}
	actual, err := catalog.Mutool("1.28.0")
	if err != nil || actual != mutool {
		t.Fatalf("Mutool() = %q, %v; want %q, nil", actual, err, mutool)
	}
	if _, err := catalog.Mutool("1.27.2"); err == nil {
		t.Fatal("Mutool accepted a version that is not installed")
	}
}
