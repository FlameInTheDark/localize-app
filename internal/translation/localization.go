package translation

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/inference"
)

var localizationToken = regexp.MustCompile(`(?s)\{\{[-!]?\s*[^{}]+\}\}|\{\d+\}|%(?:\d+\$)?[-+#0 ]*\d*(?:\.\d+)?[A-Za-z]|</?[A-Za-z][^>]*>`)

const localizationConstraint = "\nLocalization constraints: return only translated text. Preserve every placeholder, printf token, indexed variable, tag, escape sequence, and line break exactly. Never translate identifiers or markup."

// TranslateLocalized translates one ordinary message or one plural message
// group. Plural calls use the providers' existing structured-output support so
// the target forms cannot be lost or reordered.
func (s *Service) TranslateLocalized(ctx context.Context, source []domain.LocalizationForm, targetCategories []string, language, sourceLanguage string) ([]domain.LocalizationForm, error) {
	if len(source) == 0 || strings.TrimSpace(language) == "" {
		return nil, fmt.Errorf("source text and target language are required")
	}
	if len(targetCategories) == 0 {
		targetCategories = []string{"other"}
	}
	if !hasLocalizationText(source) {
		return emptyLocalizedForms(targetCategories), nil
	}
	client, model, err := s.client(ctx, false)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		forms, err := s.localizedAttempt(ctx, client, model, source, targetCategories, language, sourceLanguage, attempt > 0)
		if err == nil && validLocalizedForms(source, forms) {
			return forms, nil
		}
		if attempt == 1 {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("translation changed required placeholders or markup")
		}
	}
	return nil, fmt.Errorf("localization translation failed")
}

func hasLocalizationText(forms []domain.LocalizationForm) bool {
	for _, form := range forms {
		if strings.TrimSpace(form.Text) != "" {
			return true
		}
	}
	return false
}

func emptyLocalizedForms(categories []string) []domain.LocalizationForm {
	forms := make([]domain.LocalizationForm, len(categories))
	for index, category := range categories {
		forms[index] = domain.LocalizationForm{Category: category}
	}
	return forms
}

func (s *Service) localizedAttempt(ctx context.Context, client inference.Client, model string, source []domain.LocalizationForm, targetCategories []string, language, sourceLanguage string, recovery bool) ([]domain.LocalizationForm, error) {
	system := s.promptSettings().TranslationFor(language) + localizationConstraint
	if sourceLanguage = strings.TrimSpace(sourceLanguage); sourceLanguage != "" && sourceLanguage != "auto" {
		system += "\nSource-language constraint: the source text has been confirmed as " + sourceLanguage + ". Translate from that language only."
	}
	if recovery {
		system += "\nThe previous output was invalid. Copy every protected token exactly and return the complete requested result."
	}
	if len(targetCategories) == 1 {
		text, err := client.Generate(ctx, inference.Request{Model: model, System: system, Prompt: sourceForCategory(source, targetCategories[0])})
		if err != nil {
			return nil, err
		}
		return []domain.LocalizationForm{{Category: targetCategories[0], Text: text}}, nil
	}
	schema := pluralSchema(targetCategories)
	prompt := "Translate this plural message into " + language + ". Return every requested target form.\n\nSOURCE_FORMS:\n"
	for _, form := range source {
		prompt += form.Category + ":\n" + form.Text + "\n\n"
	}
	prompt += "TARGET_FORMS:\n" + strings.Join(targetCategories, ", ")
	response, err := client.Generate(ctx, inference.Request{Model: model, System: system, Prompt: prompt, Schema: schema})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Forms map[string]string `json:"forms"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(response)), &payload); err != nil {
		start, end := strings.Index(response, "{"), strings.LastIndex(response, "}")
		if start < 0 || end <= start || json.Unmarshal([]byte(response[start:end+1]), &payload) != nil {
			return nil, fmt.Errorf("model returned invalid plural translation data")
		}
	}
	forms := make([]domain.LocalizationForm, 0, len(targetCategories))
	for _, category := range targetCategories {
		forms = append(forms, domain.LocalizationForm{Category: category, Text: payload.Forms[category]})
	}
	return forms, nil
}

func pluralSchema(categories []string) json.RawMessage {
	properties := make(map[string]any, len(categories))
	for _, category := range categories {
		properties[category] = map[string]any{"type": "string", "minLength": 1}
	}
	payload := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"forms": map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": categories}}, "required": []string{"forms"}}
	data, _ := json.Marshal(payload)
	return data
}

func validLocalizedForms(source, target []domain.LocalizationForm) bool {
	if len(target) == 0 {
		return false
	}
	for _, form := range target {
		if strings.TrimSpace(form.Text) == "" || !sameLocalizationTokens(sourceForCategory(source, form.Category), form.Text) {
			return false
		}
	}
	return true
}

func sourceForCategory(forms []domain.LocalizationForm, category string) string {
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
	return forms[0].Text
}

func sameLocalizationTokens(source, translation string) bool {
	counts := func(value string) map[string]int {
		result := map[string]int{}
		for _, token := range localizationToken.FindAllString(value, -1) {
			result[token]++
		}
		return result
	}
	want, got := counts(source), counts(translation)
	if len(want) != len(got) {
		return false
	}
	for token, count := range want {
		if got[token] != count {
			return false
		}
	}
	return true
}
