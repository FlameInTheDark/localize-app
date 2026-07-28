package translation

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/inference"
)

func TestTranslateLocalizedPreservesProtectedTokens(t *testing.T) {
	client := &recordingClient{response: "Привет {{name}} <b>%d</b>"}
	service := New(func(context.Context, bool) (inference.Client, string, error) { return client, "test", nil }, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	forms, err := service.TranslateLocalized(context.Background(), []domain.LocalizationForm{{Category: "other", Text: "Hello {{name}} <b>%d</b>"}}, []string{"other"}, "ru", "")
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
	_, err := service.TranslateLocalized(context.Background(), []domain.LocalizationForm{{Category: "other", Text: "Hello {{name}}"}}, []string{"other"}, "ru", "")
	if err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("expected placeholder validation error, got %v", err)
	}
}

func TestTranslateLocalizedSkipsEmptySource(t *testing.T) {
	called := false
	service := New(func(context.Context, bool) (inference.Client, string, error) {
		called = true
		return fakeClient{}, "test", nil
	}, func() domain.PromptSettings { return domain.DefaultPromptSettings() })

	forms, err := service.TranslateLocalized(context.Background(), []domain.LocalizationForm{{Category: "other", Text: " \r\n\t"}}, []string{"one", "other"}, "ru", "")
	if err != nil {
		t.Fatalf("TranslateLocalized returned an error: %v", err)
	}
	if called {
		t.Fatal("empty localization text must not acquire an inference client")
	}
	want := []domain.LocalizationForm{{Category: "one"}, {Category: "other"}}
	if !reflect.DeepEqual(forms, want) {
		t.Fatalf("forms = %#v, want %#v", forms, want)
	}
}

func TestTranslateLocalizedIncludesConfirmedSourceLanguage(t *testing.T) {
	client := &recordingClient{response: "Привет"}
	service := New(func(context.Context, bool) (inference.Client, string, error) { return client, "test", nil }, func() domain.PromptSettings { return domain.DefaultPromptSettings() })
	_, err := service.TranslateLocalized(context.Background(), []domain.LocalizationForm{{Category: "other", Text: "Hello"}}, []string{"other"}, "ru", "en")
	if err != nil {
		t.Fatalf("TranslateLocalized returned an error: %v", err)
	}
	if !strings.Contains(client.request.System, "source text has been confirmed as en") {
		t.Fatalf("source-language constraint was not sent: %#v", client.request)
	}
}
