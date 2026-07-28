package app

import "testing"

func TestMatchesLocalizationSourceLanguage(t *testing.T) {
	tests := []struct {
		name     string
		detected []string
		selected string
		want     bool
	}{
		{name: "matching language", detected: []string{"en"}, selected: "en", want: true},
		{name: "case insensitive", detected: []string{"RU"}, selected: "ru", want: true},
		{name: "one of several languages", detected: []string{"en", "fr"}, selected: "fr", want: true},
		{name: "different language", detected: []string{"de"}, selected: "en", want: false},
		{name: "no detection", selected: "en", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := matchesLocalizationSourceLanguage(test.detected, test.selected); got != test.want {
				t.Fatalf("matchesLocalizationSourceLanguage(%v, %q) = %t, want %t", test.detected, test.selected, got, test.want)
			}
		})
	}
}

func TestLocalizationSourceFormsPreserveOriginalText(t *testing.T) {
	source := []LocalizationForm{{Category: "one", Text: "One item"}, {Category: "other", Text: "Many items"}}
	forms := localizationSourceForms(source, []string{"one", "few", "other"})
	want := []LocalizationForm{{Category: "one", Text: "One item"}, {Category: "few", Text: "Many items"}, {Category: "other", Text: "Many items"}}
	if len(forms) != len(want) {
		t.Fatalf("form count = %d, want %d", len(forms), len(want))
	}
	for index := range want {
		if forms[index] != want[index] {
			t.Fatalf("form %d = %#v, want %#v", index, forms[index], want[index])
		}
	}
}
