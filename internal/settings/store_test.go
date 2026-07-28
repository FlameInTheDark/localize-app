package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

func TestStorePersistsAtomicallyAndMigratesDefaults(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	value := store.Get()
	if value.Ollama.Endpoint != "http://127.0.0.1:11434" || value.LlamaCpp.ModelDir == "" {
		t.Fatalf("unexpected defaults: %#v", value)
	}
	value.ActiveProvider = domain.ProviderLlamaCpp
	value.LlamaCpp.ContextSize = 16384
	value.Ollama.Vision.ID = ""
	if err := store.Save(value); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "settings.json.tmp")); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains after atomic save: %v", err)
	}
	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if actual := reopened.Get(); actual.ActiveProvider != domain.ProviderLlamaCpp || actual.LlamaCpp.ContextSize != 16384 || actual.Ollama.Vision.ID != "" {
		t.Fatalf("settings did not persist: %#v", actual)
	}
	if actual := reopened.Get(); actual.Prompts.Translation == "" || actual.Prompts.WordVariants == "" {
		t.Fatalf("prompts did not persist: %#v", actual.Prompts)
	}
}

func TestMigrationUpdatesOnlyLegacyAlternativesPrompt(t *testing.T) {
	root := t.TempDir()
	legacy := domain.DefaultSettings(filepath.Join(root, "models"))
	legacy.Version = 3
	legacy.Prompts.WordVariants = domain.LegacyWordVariantsPrompt
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if settings := store.Get(); settings.Version != 7 || settings.Prompts.WordVariants == domain.LegacyWordVariantsPrompt {
		t.Fatalf("legacy alternatives prompt was not upgraded: %#v", settings.Prompts.WordVariants)
	}

	customRoot := t.TempDir()
	custom := legacy
	custom.Prompts.WordVariants = "My custom alternatives instruction"
	data, err = json.Marshal(custom)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customRoot, "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	customStore, err := New(customRoot)
	if err != nil {
		t.Fatal(err)
	}
	if actual := customStore.Get().Prompts.WordVariants; actual != custom.Prompts.WordVariants {
		t.Fatalf("custom alternatives prompt was changed: %q", actual)
	}
}

func TestMigrationRestoresMissingFields(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"version":0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if settings := store.Get(); settings.Version != 7 || settings.Ollama.Translation.ID == "" || settings.LlamaCpp.ContextSize < 1024 || settings.WhisperCpp.ModelDir == "" || settings.WhisperCpp.Language != "auto" || settings.Prompts.WordVariants == "" {
		t.Fatalf("migration did not restore defaults: %#v", settings)
	}
}
