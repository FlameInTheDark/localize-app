package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalModelsFindsSingleProjectionAndUsesStableID(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "author__model")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(directory, "model-q4.gguf")
	projection := filepath.Join(directory, "mmproj-f16.gguf")
	for _, path := range []string{model, projection} {
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	models, err := LocalModels(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "author__model/model-q4" || !models[0].SupportsVision || models[0].ProjectionPath != projection {
		t.Fatalf("unexpected model catalog: %#v", models)
	}
	if err := DeleteLocalModel(root, models[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(model); !os.IsNotExist(err) {
		t.Fatalf("model remains after deletion: %v", err)
	}
	if _, err := os.Stat(projection); err != nil {
		t.Fatalf("projection should not be removed with the model: %v", err)
	}
}

func TestDeleteLocalModelRejectsEscapingID(t *testing.T) {
	if err := DeleteLocalModel(t.TempDir(), "../outside"); err == nil {
		t.Fatal("expected escaping model ID to be rejected")
	}
}

func TestLocalModelsReturnsAnEmptyArray(t *testing.T) {
	models, err := LocalModels(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if models == nil || len(models) != 0 {
		t.Fatalf("expected an empty model array, got %#v", models)
	}
}
