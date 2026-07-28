package localization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

type jsonDocument struct {
	value  any
	bom    bool
	ending string
}

func parseJSON(data []byte) (*jsonDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(stripBOM(data)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, fmt.Errorf("i18next JSON must have an object at its root")
	}
	return &jsonDocument{value: value, bom: hasBOM(data), ending: lineEnding(data)}, nil
}

func (d *jsonDocument) entries() []domain.LocalizationEntry {
	entries := make([]domain.LocalizationEntry, 0)
	collectJSONEntries(d.value, nil, &entries)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries
}

func collectJSONEntries(value any, path []string, entries *[]domain.LocalizationEntry) {
	switch node := value.(type) {
	case map[string]any:
		pluralKeys := make(map[string][]string)
		for key, item := range node {
			if _, ok := item.(string); !ok {
				continue
			}
			if base, category, ok := i18nextPluralKey(key); ok {
				pluralKeys[base] = append(pluralKeys[base], category)
			}
		}
		grouped := make(map[string]bool)
		bases := make([]string, 0, len(pluralKeys))
		for base, categories := range pluralKeys {
			if contains(categories, "other") {
				bases = append(bases, base)
			}
		}
		sort.Strings(bases)
		for _, base := range bases {
			categories := pluralKeys[base]
			sort.Slice(categories, func(i, j int) bool { return formRank(categories[i]) < formRank(categories[j]) })
			source := make([]domain.LocalizationForm, 0, len(categories))
			for _, category := range categories {
				key := base + "_" + category
				source = append(source, domain.LocalizationForm{Category: category, Text: node[key].(string)})
				grouped[key] = true
			}
			id := pointer(append(path, base))
			*entries = append(*entries, domain.LocalizationEntry{ID: id, Key: displayKey(append(path, base)), Source: source, Translation: []domain.LocalizationForm{}, Plural: true})
		}
		keys := make([]string, 0, len(node))
		for key := range node {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if grouped[key] {
				continue
			}
			collectJSONEntries(node[key], append(path, key), entries)
		}
	case []any:
		for index, item := range node {
			collectJSONEntries(item, append(path, fmt.Sprintf("%d", index)), entries)
		}
	case string:
		*entries = append(*entries, domain.LocalizationEntry{ID: pointer(path), Key: displayKey(path), Source: []domain.LocalizationForm{{Category: "other", Text: node}}, Translation: []domain.LocalizationForm{}})
	}
}

func (d *jsonDocument) render(translations map[string][]domain.LocalizationForm, fallback domain.UntranslatedExportMode, language string) ([]byte, error) {
	if err := applyJSON(d.value, nil, translations, fallback, language); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(d.value, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, []byte(d.ending)...)
	if d.bom {
		data = append([]byte{0xef, 0xbb, 0xbf}, data...)
	}
	return data, nil
}

func applyJSON(value any, path []string, translations map[string][]domain.LocalizationForm, fallback domain.UntranslatedExportMode, language string) error {
	switch node := value.(type) {
	case map[string]any:
		bases := make(map[string]struct{})
		for key, item := range node {
			if _, ok := item.(string); ok {
				if base, _, plural := i18nextPluralKey(key); plural {
					bases[base] = struct{}{}
				}
			}
		}
		for base := range bases {
			id := pointer(append(path, base))
			forms, selected := translations[id]
			if !selected {
				continue
			}
			source := pluralSource(node, base)
			forms = fallbackForms(source, forms, fallback)
			if len(forms) == 0 {
				continue
			}
			categories, err := TargetPluralForms(language)
			if err != nil {
				return err
			}
			byCategory := formsByCategory(forms)
			for key, item := range node {
				if _, ok := item.(string); !ok {
					continue
				}
				if candidate, _, plural := i18nextPluralKey(key); plural && candidate == base {
					delete(node, key)
				}
			}
			for _, category := range categories {
				text, ok := byCategory[category]
				if !ok {
					text = bestForm(forms, category)
				}
				node[base+"_"+category] = text
			}
		}
		for key, item := range node {
			if text, ok := item.(string); ok {
				if forms, selected := translations[pointer(append(path, key))]; selected {
					forms = fallbackForms([]domain.LocalizationForm{{Category: "other", Text: text}}, forms, fallback)
					node[key] = bestForm(forms, "other")
				}
				continue
			}
			if err := applyJSON(item, append(path, key), translations, fallback, language); err != nil {
				return err
			}
		}
	case []any:
		for index, item := range node {
			if text, ok := item.(string); ok {
				if forms, selected := translations[pointer(append(path, fmt.Sprintf("%d", index)))]; selected {
					forms = fallbackForms([]domain.LocalizationForm{{Category: "other", Text: text}}, forms, fallback)
					node[index] = bestForm(forms, "other")
				}
				continue
			}
			if err := applyJSON(item, append(path, fmt.Sprintf("%d", index)), translations, fallback, language); err != nil {
				return err
			}
		}
	}
	return nil
}

func pluralSource(node map[string]any, base string) []domain.LocalizationForm {
	var forms []domain.LocalizationForm
	for _, category := range pluralOrder {
		if value, ok := node[base+"_"+category].(string); ok {
			forms = append(forms, domain.LocalizationForm{Category: category, Text: value})
		}
	}
	return forms
}

func formsByCategory(forms []domain.LocalizationForm) map[string]string {
	out := make(map[string]string, len(forms))
	for _, form := range forms {
		out[form.Category] = form.Text
	}
	return out
}
func bestForm(forms []domain.LocalizationForm, category string) string {
	for _, form := range forms {
		if form.Category == category {
			return form.Text
		}
	}
	for _, form := range forms {
		if form.Category == "other" {
			return form.Text
		}
	}
	if len(forms) > 0 {
		return forms[0].Text
	}
	return ""
}

var pluralOrder = []string{"zero", "one", "two", "few", "many", "other"}

func formRank(category string) int {
	for index, candidate := range pluralOrder {
		if candidate == category {
			return index
		}
	}
	return len(pluralOrder)
}
func i18nextPluralKey(key string) (string, string, bool) {
	for _, category := range pluralOrder {
		suffix := "_" + category
		if strings.HasSuffix(key, suffix) && len(key) > len(suffix) {
			return strings.TrimSuffix(key, suffix), category, true
		}
	}
	return "", "", false
}
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
func pointer(path []string) string {
	if len(path) == 0 {
		return "/"
	}
	encoded := make([]string, len(path))
	for index, part := range path {
		encoded[index] = strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
	}
	return "/" + strings.Join(encoded, "/")
}
func displayKey(path []string) string { return strings.Join(path, ".") }
