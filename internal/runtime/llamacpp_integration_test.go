//go:build integration

package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/inference"
	"github.com/FlameInTheDark/localize-app/internal/operations"
	llamaruntime "github.com/FlameInTheDark/localize-app/internal/runtime"
	"github.com/FlameInTheDark/localize-app/internal/translation"
)

// TestTranslateGemmaRuntimeFallback is opt-in because it loads a local model
// and GPU runtime. It proves that the compatibility path starts the real
// llama.cpp server and produces a non-empty translation response.
func TestTranslateGemmaRuntimeFallback(t *testing.T) {
	modelPath := os.Getenv("LOCALIZE_LLAMA_MODEL")
	projectionPath := os.Getenv("LOCALIZE_LLAMA_MMPROJ")
	runtimeDir := os.Getenv("LOCALIZE_LLAMA_RUNTIME_DIR")
	if modelPath == "" || projectionPath == "" || runtimeDir == "" {
		t.Skip("set LOCALIZE_LLAMA_MODEL, LOCALIZE_LLAMA_MMPROJ, and LOCALIZE_LLAMA_RUNTIME_DIR to run this local integration test")
	}
	for _, path := range []string{modelPath, projectionPath, runtimeDir} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("required local runtime path %q: %v", path, err)
		}
	}
	version := os.Getenv("LOCALIZE_LLAMA_RUNTIME_VERSION")
	if version == "" {
		version = filepath.Base(runtimeDir)
		runtimeDir = filepath.Dir(runtimeDir)
	}
	manager := llamaruntime.NewLlamaManager(runtimeDir, &operations.Hub{})
	defer manager.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	endpoint, err := manager.Endpoint(ctx, domain.LlamaCppSettings{
		RuntimeVersion: version,
		RuntimeMode:    domain.RuntimeCUDA,
		ContextSize:    8192,
	}, domain.ModelAssignment{
		ID:             "mradermacher__translategemma-4b-it-GGUF",
		Path:           modelPath,
		ProjectionPath: projectionPath,
	}, "integration")
	if err != nil {
		t.Fatal(err)
	}
	client := inference.NewLlamaCpp(endpoint)
	translated, err := client.Generate(ctx, inference.Request{
		Model:  "mradermacher__translategemma-4b-it-GGUF",
		System: "Translate the entire user content into French. Return only the translated text.",
		Prompt: "Hello, how are you?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(translated) == "" {
		t.Fatal("llama.cpp returned an empty translation")
	}
	t.Logf("TranslateGemma result: %q", translated)

	variantsService := translation.New(func(context.Context, bool) (inference.Client, string, error) {
		return client, "mradermacher__translategemma-4b-it-GGUF", nil
	}, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	variants, err := variantsService.Variants(ctx, domain.TranslationVariantsRequest{
		SourceText:          "Apoapsis is the point in an elliptical orbit where an object is farthest from the center of mass of the body it orbits. At this farthest point, the object moves at its slowest speed.",
		TargetContext:       "Апоапсис – это точка в эллиптической орбите, где объект находится на наибольшем расстоянии от центра масс тела, вокруг которого он вращается.",
		MarkedTargetContext: "Апоапсис – это точка в эллиптической орбите, где объект находится на наибольшем расстоянии от центра масс тела, вокруг которого он <alt-selection>вращается</alt-selection>.",
		SelectedText:        "вращается",
		Language:            "ru",
	})
	if err != nil {
		t.Fatalf("generate contextual alternatives: %v", err)
	}
	if len(variants.Variants) < 5 {
		t.Fatalf("expected at least five contextual alternatives, got %#v", variants)
	}
	t.Logf("TranslateGemma alternatives: %#v", variants.Variants)
}
