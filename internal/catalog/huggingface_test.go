package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestGatedRecognition(t *testing.T) {
	for _, value := range []any{true, "manual", "auto", "true"} {
		if !gated(value) {
			t.Fatalf("expected %#v to be gated", value)
		}
	}
	for _, value := range []any{false, "false", "", nil} {
		if gated(value) {
			t.Fatalf("expected %#v to be public", value)
		}
	}
}

func TestFilesRequestsBlobMetadataForActualSizes(t *testing.T) {
	var query string
	var queryMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		queryMu.Lock()
		query = request.URL.Query().Get("blobs")
		queryMu.Unlock()
		_, _ = response.Write([]byte(`{"siblings":[{"rfilename":"model.Q4_K_M.gguf","size":0,"lfs":{"oid":"abc","size":524288000}}]}`))
	}))
	defer server.Close()
	catalog := NewHuggingFace(nil)
	catalog.hubURL, catalog.http = server.URL, server.Client()
	files, err := catalog.Files(context.Background(), "demo/model")
	if err != nil {
		t.Fatal(err)
	}
	queryMu.Lock()
	actualQuery := query
	queryMu.Unlock()
	if actualQuery != "true" {
		t.Fatalf("expected blobs=true, got %q", actualQuery)
	}
	if len(files) != 1 || files[0].Size != 524288000 {
		t.Fatalf("unexpected file metadata: %#v", files)
	}
}

func TestChecksumMatchesLFSOID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.gguf.part")
	content := []byte("localize fixture")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	if !checksumMatches(path, "sha256:"+hex.EncodeToString(sum[:])) {
		t.Fatal("matching LFS digest was rejected")
	}
	if checksumMatches(path, "sha256:"+string(make([]byte, 64))) {
		t.Fatal("wrong LFS digest was accepted")
	}
}
