package localization

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

func TestJSONExtractsAndRegeneratesI18NextPluralForms(t *testing.T) {
	path := fixture(t, "en.json", `{"welcome":"Hello {{name}}","item_one":"One item","item_other":"{{count}} items"}`)
	file, _, err := Open(path, domain.LocalizationFormatI18NextJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Entries) != 2 || !file.Entries[0].Plural && !file.Entries[1].Plural {
		t.Fatalf("expected simple and plural entries: %#v", file.Entries)
	}
	translations := make([]domain.LocalizationEntry, 0, 2)
	for _, entry := range file.Entries {
		switch entry.Key {
		case "welcome":
			entry.Translation = []domain.LocalizationForm{{Category: "other", Text: "Привет {{name}}"}}
		case "item":
			entry.Translation = []domain.LocalizationForm{{Category: "one", Text: "Один предмет"}, {Category: "few", Text: "{{count}} предмета"}, {Category: "many", Text: "{{count}} предметов"}, {Category: "other", Text: "{{count}} предмета"}}
		}
		translations = append(translations, entry)
	}
	data, err := Render(path, file.Format, file.Fingerprint, translations, domain.UntranslatedExportSource, "ru")
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	for _, want := range []string{`"welcome": "Привет {{name}}"`, `"item_one": "Один предмет"`, `"item_few": "{{count}} предмета"`, `"item_many": "{{count}} предметов"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("translated JSON misses %s:\n%s", want, output)
		}
	}
}

func TestYAMLPropertiesAndKeyValuesRoundTrip(t *testing.T) {
	t.Run("YAML", func(t *testing.T) {
		path := fixture(t, "en.yaml", "menu:\n  play: Play\n")
		file, _, err := Open(path, domain.LocalizationFormatYAML)
		if err != nil {
			t.Fatal(err)
		}
		file.Entries[0].Translation = []domain.LocalizationForm{{Category: "other", Text: "Играть"}}
		data, err := Render(path, file.Format, file.Fingerprint, file.Entries, domain.UntranslatedExportSource, "ru")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "Играть") {
			t.Fatalf("unexpected YAML: %s", data)
		}
	})
	t.Run("properties", func(t *testing.T) {
		path := fixture(t, "messages.properties", "# keep this comment\nplay=Play\n")
		file, _, err := Open(path, domain.LocalizationFormatProperties)
		if err != nil {
			t.Fatal(err)
		}
		file.Entries[0].Translation = []domain.LocalizationForm{{Category: "other", Text: "Играть"}}
		data, err := Render(path, file.Format, file.Fingerprint, file.Entries, domain.UntranslatedExportSource, "ru")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "# keep this comment") || !strings.Contains(string(data), "play=\\u0418") {
			t.Fatalf("unexpected properties: %s", data)
		}
	})
	t.Run("Source KeyValues", func(t *testing.T) {
		path := fixture(t, "game_english.txt", "\"lang\"\n{\n  \"Language\" \"English\"\n  \"Tokens\"\n  {\n    \"#PLAY\" \"Play\" // keep\n  }\n}\n")
		file, _, err := Open(path, domain.LocalizationFormatSourceKeyValues)
		if err != nil {
			t.Fatal(err)
		}
		file.Entries[0].Translation = []domain.LocalizationForm{{Category: "other", Text: "Играть"}}
		data, err := Render(path, file.Format, file.Fingerprint, file.Entries, domain.UntranslatedExportSource, "ru")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "\"Language\" \"Russian\"") || !strings.Contains(string(data), "\"#PLAY\" \"Играть\"") || !strings.Contains(string(data), "// keep") {
			t.Fatalf("unexpected KeyValues: %s", data)
		}
	})
	t.Run("Steam VDF localization", func(t *testing.T) {
		path := fixture(t, "localization.vdf", "\"localization\"\n{\n  \"english\"\n  {\n    \"store_tags\"\n    {\n      \"19\" \"Action\"\n    }\n  }\n}\n")
		file, _, err := Open(path, domain.LocalizationFormatSourceKeyValues)
		if err != nil {
			t.Fatal(err)
		}
		if len(file.Entries) != 1 || file.Entries[0].Key != "store_tags.19" {
			t.Fatalf("unexpected Steam VDF entries: %#v", file.Entries)
		}
		file.Entries[0].Translation = []domain.LocalizationForm{{Category: "other", Text: "Экшен"}}
		data, err := Render(path, file.Format, file.Fingerprint, file.Entries, domain.UntranslatedExportSource, "ru")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "\"russian\"") || !strings.Contains(string(data), "\"19\" \"Экшен\"") {
			t.Fatalf("unexpected Steam VDF: %s", data)
		}
	})
	t.Run("UTF-16LE Source KeyValues", func(t *testing.T) {
		path := fixtureBytes(t, "closecaption_russian.txt", utf16LE("\"lang\"\n{\n  \"language\" \"russian\"\n  \"tokens\"\n  {\n    \"caption\" \"Hello\"\n  }\n}\n"))
		file, _, err := Open(path, domain.LocalizationFormatSourceKeyValues)
		if err != nil {
			t.Fatal(err)
		}
		file.Entries[0].Translation = []domain.LocalizationForm{{Category: "other", Text: "Привет"}}
		data, err := Render(path, file.Format, file.Fingerprint, file.Entries, domain.UntranslatedExportSource, "ru")
		if err != nil {
			t.Fatal(err)
		}
		if len(data) < 2 || data[0] != 0xff || data[1] != 0xfe {
			t.Fatalf("UTF-16LE BOM was not preserved: %x", data[:min(2, len(data))])
		}
		decoded, _, err := decodeKeyValues(data)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(decoded), "\"language\" \"Russian\"") || !strings.Contains(string(decoded), "\"caption\" \"Привет\"") {
			t.Fatalf("unexpected UTF-16 KeyValues output: %s", decoded)
		}
	})
}

func TestPORenderUpdatesHeaderPluralFormsAndFuzzy(t *testing.T) {
	path := fixture(t, "en.po", "msgid \"\"\nmsgstr \"\"\n\"Language: en\\n\"\n\"Plural-Forms: nplurals=2; plural=(n != 1);\\n\"\n\n#, fuzzy\nmsgctxt \"inventory\"\nmsgid \"One item\"\nmsgid_plural \"%d items\"\nmsgstr[0] \"\"\nmsgstr[1] \"\"\n")
	file, _, err := Open(path, domain.LocalizationFormatGettextPO)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Entries) != 1 || !file.Entries[0].Plural {
		t.Fatalf("unexpected PO entries: %#v", file.Entries)
	}
	file.Entries[0].Translation = []domain.LocalizationForm{{Category: "one", Text: "Один предмет"}, {Category: "few", Text: "%d предмета"}, {Category: "many", Text: "%d предметов"}, {Category: "other", Text: "%d предмета"}}
	data, err := Render(path, file.Format, file.Fingerprint, file.Entries, domain.UntranslatedExportSource, "ru")
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	for _, want := range []string{"Language: ru", "nplurals=3", "msgstr[2]", "%d предметов"} {
		if !strings.Contains(output, want) {
			t.Fatalf("PO misses %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "fuzzy") {
		t.Fatalf("newly translated PO entry remained fuzzy:\n%s", output)
	}
}

func TestRenderRejectsChangedSource(t *testing.T) {
	path := fixture(t, "en.json", `{"play":"Play"}`)
	file, _, err := Open(path, domain.LocalizationFormatI18NextJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"play":"Changed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Render(path, file.Format, file.Fingerprint, file.Entries, domain.UntranslatedExportSource, "ru"); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("expected fingerprint error, got %v", err)
	}
}

func fixture(t *testing.T, name, content string) string {
	return fixtureBytes(t, name, []byte(content))
}

func fixtureBytes(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func utf16LE(value string) []byte {
	output := []byte{0xff, 0xfe}
	for _, unit := range utf16.Encode([]rune(value)) {
		var encoded [2]byte
		binary.LittleEndian.PutUint16(encoded[:], unit)
		output = append(output, encoded[:]...)
	}
	return output
}
