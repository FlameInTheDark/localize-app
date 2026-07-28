package runtime

import (
	"strings"
	"testing"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

func TestServerArgumentsUseCompatibleTemplateForTranslateGemma(t *testing.T) {
	settings := domain.LlamaCppSettings{ContextSize: 8192}
	assignment := domain.ModelAssignment{
		ID:   "mradermacher__translategemma-4b-it-GGUF",
		Path: `C:\models\translategemma-4b-it.Q8_0.gguf`,
	}
	arguments := strings.Join(serverArguments(settings, assignment, "12345", domain.RuntimeCUDA), " ")
	for _, expected := range []string{"--no-jinja", "--chat-template gemma", "-ngl 999"} {
		if !strings.Contains(arguments, expected) {
			t.Fatalf("missing %q from runtime arguments: %s", expected, arguments)
		}
	}
}

func TestServerArgumentsKeepRegularModelsOnTheirEmbeddedTemplate(t *testing.T) {
	settings := domain.LlamaCppSettings{ContextSize: 4096}
	assignment := domain.ModelAssignment{ID: "qwen3", Path: `C:\models\qwen3.gguf`}
	arguments := strings.Join(serverArguments(settings, assignment, "12345", domain.RuntimeCPU), " ")
	if strings.Contains(arguments, "--no-jinja") || strings.Contains(arguments, "--chat-template gemma") {
		t.Fatalf("regular model should retain its embedded template: %s", arguments)
	}
}

func TestServerArgumentsOffloadLayersForGPUBackends(t *testing.T) {
	settings := domain.LlamaCppSettings{ContextSize: 4096}
	assignment := domain.ModelAssignment{ID: "qwen3", Path: `C:\models\qwen3.gguf`}
	for _, mode := range []domain.RuntimeMode{domain.RuntimeCUDA, domain.RuntimeVulkan, domain.RuntimeHIP} {
		t.Run(string(mode), func(t *testing.T) {
			arguments := strings.Join(serverArguments(settings, assignment, "12345", mode), " ")
			if !strings.Contains(arguments, "-ngl 999") {
				t.Fatalf("expected GPU layers to be offloaded for %s: %s", mode, arguments)
			}
		})
	}
}
