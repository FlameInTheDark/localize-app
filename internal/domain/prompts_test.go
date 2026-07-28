package domain

import "testing"

func TestDefaultPromptSettingsAreValid(t *testing.T) {
	if err := DefaultPromptSettings().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPromptSettingsRejectBlankAndOversizedValues(t *testing.T) {
	settings := DefaultPromptSettings()
	settings.OCR = " "
	if err := settings.Validate(); err == nil {
		t.Fatal("expected blank prompt validation error")
	}
	settings = DefaultPromptSettings()
	settings.Translation = string(make([]rune, MaxPromptLength+1))
	if err := settings.Validate(); err == nil {
		t.Fatal("expected oversized prompt validation error")
	}
}
