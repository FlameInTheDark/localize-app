package translation

import (
	"context"
	"strings"
	"testing"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/inference"
)

func TestTranslateLocalizedPreservesProtectedTokens(t *testing.T) {
	client := &recordingClient{response: "Привет {{name}} <b>%d</b>"}
	service := New(func(context.Context, bool) (inference.Client, string, error) { return client, "test", nil }, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	forms, err := service.TranslateLocalized(context.Background(), []domain.LocalizationForm{{Category: "other", Text: "Hello {{name}} <b>%d</b>"}}, []string{"other"}, "ru")
	if err != nil || len(forms) != 1 || forms[0].Text != "Привет {{name}} <b>%d</b>" {
		t.Fatalf("unexpected translation: %#v, %v", forms, err)
	}
	if !strings.Contains(client.request.System, "Localization constraints") {
		t.Fatalf("localization constraints were not sent: %#v", client.request)
	}
}

func TestTranslateLocalizedRejectsBrokenPlaceholder(t *testing.T) {
	service := New(func(context.Context, bool) (inference.Client, string, error) {
		return fakeClient{response: "Привет"}, "test", nil
	}, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	_, err := service.TranslateLocalized(context.Background(), []domain.LocalizationForm{{Category: "other", Text: "Hello {{name}}"}}, []string{"other"}, "ru")
	if err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("expected placeholder validation error, got %v", err)
	}
}
