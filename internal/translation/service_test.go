package translation

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/inference"
)

type fakeClient struct{ response string }

func (f fakeClient) Health(context.Context) error                           { return nil }
func (f fakeClient) ListModels(context.Context) ([]domain.ModelInfo, error) { return nil, nil }
func (f fakeClient) Generate(context.Context, inference.Request) (string, error) {
	return f.response, nil
}

type recordingClient struct {
	response string
	request  inference.Request
}

func (f *recordingClient) Health(context.Context) error { return nil }
func (f *recordingClient) ListModels(context.Context) ([]domain.ModelInfo, error) {
	return nil, nil
}
func (f *recordingClient) Generate(_ context.Context, request inference.Request) (string, error) {
	f.request = request
	return f.response, nil
}

func TestDetectNormalizesLanguageCodes(t *testing.T) {
	service := New(func(context.Context, bool) (inference.Client, string, error) {
		return fakeClient{response: "Detected: EN, ru"}, "test", nil
	}, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	codes, err := service.Detect(context.Background(), "Hello, привет")
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 2 || codes[0] != "en" || codes[1] != "ru" {
		t.Fatalf("unexpected codes: %#v", codes)
	}
}

func TestSplitTextProducesBoundedOrderedChunks(t *testing.T) {
	input := "First sentence. Second sentence is longer. Third sentence."
	chunks := SplitText(input, 24)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %#v", chunks)
	}
	for _, chunk := range chunks {
		if len(chunk) > 24 {
			t.Fatalf("chunk exceeds limit: %q", chunk)
		}
	}
}

func TestVariantsUseSchemaAndFilterInvalidCandidates(t *testing.T) {
	client := &recordingClient{response: `{"variants":[{"target":"quick fox","replacement":"swift fox"},{"target":"quick","replacement":"quick"},{"target":"missing","replacement":"unused"},{"target":"quick fox","replacement":"swift fox"}]}`}
	settings := domain.DefaultPromptSettings()
	service := New(func(context.Context, bool) (inference.Client, string, error) { return client, "test", nil }, func() domain.PromptSettings { return settings })
	request := domain.TranslationVariantsRequest{SourceText: "The quick fox jumps.", TargetContext: "The quick fox jumps.", MarkedTargetContext: "The <alt-selection>quick</alt-selection> fox jumps.", SelectedText: "quick", Language: "en"}
	result, err := service.Variants(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Variants) != 1 || result.Variants[0].Replacement != "swift fox" {
		t.Fatalf("unexpected variants: %#v", result)
	}
	if len(client.request.Schema) == 0 || client.request.Temperature == nil || *client.request.Temperature != 0.25 {
		t.Fatalf("structured request was not configured: %#v", client.request)
	}
	if !strings.Contains(client.request.System, "<alt-selection>") || !strings.Contains(client.request.System, `{"variants"`) || !strings.Contains(client.request.System, "Recovery instruction") || !strings.Contains(client.request.Prompt, "MARKED_TARGET_CONTEXT") || !strings.Contains(client.request.Prompt, "The <alt-selection>quick</alt-selection> fox") {
		t.Fatalf("contextual prompt was not rendered: %#v", client.request)
	}
}

func TestVariantsNormalizesSentenceFinalTarget(t *testing.T) {
	response := `{"variants":[
		{"target":"Вращается.","replacement":"движется по орбите"},
		{"target":"вращается","replacement":"обращается вокруг него"},
		{"target":"ВРАЩАЕТСЯ","replacement":"совершает оборот"},
		{"target":"вращается.","replacement":"кружит вокруг центра"},
		{"target":"вращается","replacement":"находится в обращении"}
	]}`
	service := New(func(context.Context, bool) (inference.Client, string, error) {
		return fakeClient{response: response}, "test", nil
	}, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	request := domain.TranslationVariantsRequest{
		SourceText:          "It orbits the body.",
		TargetContext:       "он вращается",
		MarkedTargetContext: "он <alt-selection>вращается</alt-selection>",
		SelectedText:        "вращается",
		Language:            "ru",
	}
	result, err := service.Variants(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Variants) != 5 {
		t.Fatalf("expected five normalized variants, got %#v", result)
	}
	for _, variant := range result.Variants {
		if variant.Target != "вращается" {
			t.Fatalf("target was not normalized to the displayed text: %#v", variant)
		}
	}
}

func TestVariantsAcceptsPlainNumberedAlternatives(t *testing.T) {
	response := "1. движется по орбите\n2. обращается вокруг тела\n3. совершает оборот\n4. кружит вокруг центра\n5. находится в обращении"
	service := New(func(context.Context, bool) (inference.Client, string, error) {
		return fakeClient{response: response}, "test", nil
	}, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	request := domain.TranslationVariantsRequest{
		SourceText:          "It orbits the body.",
		TargetContext:       "он вращается.",
		MarkedTargetContext: "он <alt-selection>вращается</alt-selection>.",
		SelectedText:        "вращается",
		Language:            "ru",
	}
	result, err := service.Variants(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Variants) != 5 || result.Variants[0].Target != "вращается" {
		t.Fatalf("plain alternatives were not recovered: %#v", result)
	}
}

func TestVariantsRejectWhenNoCandidateCanReplaceSelection(t *testing.T) {
	service := New(func(context.Context, bool) (inference.Client, string, error) {
		return fakeClient{response: `{"variants":[{"target":"fox","replacement":"wolf"}]}`}, "test", nil
	}, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	_, err := service.Variants(context.Background(), domain.TranslationVariantsRequest{SourceText: "The quick fox", TargetContext: "The quick fox", MarkedTargetContext: "The <alt-selection>quick</alt-selection> fox", SelectedText: "quick", Language: "en"})
	if err == nil || !strings.Contains(err.Error(), "no usable alternatives") {
		t.Fatalf("expected unusable alternatives error, got %v", err)
	}
}

func TestVariantsSupportSelectionAtTheBeginningOfContext(t *testing.T) {
	service := New(func(context.Context, bool) (inference.Client, string, error) {
		return fakeClient{response: `{"variants":[{"target":"The quick","replacement":"A swift"}]}`}, "test", nil
	}, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	request := domain.TranslationVariantsRequest{
		SourceText:          "Le rapide renard saute.",
		TargetContext:       "The quick fox jumps.",
		MarkedTargetContext: "<alt-selection>The</alt-selection> quick fox jumps.",
		SelectedText:        "The",
		Language:            "en",
	}
	result, err := service.Variants(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Variants) != 1 || result.Variants[0].Target != "The quick" {
		t.Fatalf("first-word selection was not preserved: %#v", result)
	}
}

func TestVariantsRejectMarkerThatDoesNotMatchSelectedText(t *testing.T) {
	service := New(func(context.Context, bool) (inference.Client, string, error) {
		return fakeClient{}, "test", nil
	}, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	_, err := service.Variants(context.Background(), domain.TranslationVariantsRequest{
		SourceText: "The quick fox", TargetContext: "The quick fox", MarkedTargetContext: "The <alt-selection>fox</alt-selection>", SelectedText: "quick", Language: "en",
	})
	if err == nil || !strings.Contains(err.Error(), "marked target context") {
		t.Fatalf("expected marker validation error, got %v", err)
	}
}

func TestVariantsKeepsSelectionContractWithCustomPrompt(t *testing.T) {
	settings := domain.DefaultPromptSettings()
	settings.WordVariants = "Return concise alternatives."
	service := New(func(context.Context, bool) (inference.Client, string, error) {
		return fakeClient{response: `{"variants":[{"target":"quick","replacement":"swift"}]}`}, "test", nil
	}, func() domain.PromptSettings { return settings })
	_, err := service.Variants(context.Background(), domain.TranslationVariantsRequest{
		SourceText: "Le rapide renard", TargetContext: "The quick fox", MarkedTargetContext: "The <alt-selection>quick</alt-selection> fox", SelectedText: "quick", Language: "en",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(service.promptSettings().VariantsFor("en"), "<alt-selection>") {
		t.Fatal("custom prompt did not retain the marker contract")
	}
}
