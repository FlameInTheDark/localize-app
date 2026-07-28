package translation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/inference"
)

const maxVariantSourceRunes = 6000
const maxVariantContextRunes = 1000
const maxVariantTermRunes = 160
const minVariantCount = 5
const maxVariantCount = 10
const selectionStartMarker = "<alt-selection>"
const selectionEndMarker = "</alt-selection>"

var languageCode = regexp.MustCompile(`(?i)\b[a-z]{2,3}\b`)

var variantsSchema = json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "properties":{
    "variants":{
      "type":"array",
      "minItems":5,
      "maxItems":10,
      "items":{
        "type":"object",
        "additionalProperties":false,
        "properties":{
          "target":{"type":"string","minLength":1,"maxLength":120},
          "replacement":{"type":"string","minLength":1,"maxLength":120}
        },
        "required":["target","replacement"]
      }
    }
  },
  "required":["variants"]
}`)

// ClientResolver selects the configured provider and model for a task.
type ClientResolver func(context.Context, bool) (inference.Client, string, error)

// PromptResolver supplies saved prompt settings without exposing storage details.
type PromptResolver func() domain.PromptSettings

type Service struct {
	client  ClientResolver
	prompts PromptResolver
}

func New(client ClientResolver, prompts PromptResolver) *Service {
	return &Service{client: client, prompts: prompts}
}

func (s *Service) Translate(ctx context.Context, text, language string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("text is required")
	}
	if strings.TrimSpace(language) == "" {
		return "", fmt.Errorf("target language is required")
	}
	client, model, err := s.client(ctx, false)
	if err != nil {
		return "", err
	}
	return client.Generate(ctx, inference.Request{Model: model, System: s.promptSettings().TranslationFor(language), Prompt: text})
}

func (s *Service) Detect(ctx context.Context, text string) ([]string, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("text is required")
	}
	client, model, err := s.client(ctx, false)
	if err != nil {
		return nil, err
	}
	response, err := client.Generate(ctx, inference.Request{Model: model, System: s.promptSettings().Detection(), Prompt: text})
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var codes []string
	for _, code := range languageCode.FindAllString(strings.ToLower(response), -1) {
		if len(code) != 2 {
			continue
		}
		if _, ok := seen[code]; !ok {
			seen[code] = struct{}{}
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("the model did not return an ISO 639-1 language code")
	}
	return codes, nil
}

func (s *Service) OCR(ctx context.Context, image []byte) (string, error) {
	if len(image) == 0 {
		return "", fmt.Errorf("image is required")
	}
	mime := http.DetectContentType(image)
	if !strings.HasPrefix(mime, "image/") {
		return "", fmt.Errorf("unsupported image type: %s", mime)
	}
	client, model, err := s.client(ctx, true)
	if err != nil {
		return "", err
	}
	return client.Generate(ctx, inference.Request{Model: model, System: s.promptSettings().OCR(), Prompt: "Read the image.", ImageBase64: base64.StdEncoding.EncodeToString(image), MimeType: mime})
}

// Variants returns contextual replacements for a selected piece of translated text.
func (s *Service) Variants(ctx context.Context, request domain.TranslationVariantsRequest) (domain.TranslationVariantsResult, error) {
	request = normalizeVariantRequest(request)
	if err := validateVariantRequest(request); err != nil {
		return domain.TranslationVariantsResult{}, err
	}
	client, model, err := s.client(ctx, false)
	if err != nil {
		return domain.TranslationVariantsResult{}, err
	}
	temperature := 0.25
	response, err := client.Generate(ctx, inference.Request{
		Model:       model,
		System:      s.promptSettings().VariantsFor(request.Language),
		Prompt:      variantUserMessage(request),
		Schema:      variantsSchema,
		Temperature: &temperature,
	})
	if err != nil {
		return domain.TranslationVariantsResult{}, err
	}
	initial, initialErr := parseVariants(response, request)
	if initialErr == nil && len(initial.Variants) >= minVariantCount {
		return initial, nil
	}

	// A local model can return a valid but undersized list, or produce a
	// slightly malformed object despite a response schema. Try once more with a
	// fixed recovery instruction rather than exposing a blank popover.
	retry, retryErr := client.Generate(ctx, inference.Request{
		Model:       model,
		System:      s.promptSettings().VariantsFor(request.Language) + "\nRecovery instruction: the previous answer had too few usable choices. Return ten complete JSON alternatives now. Copy every target exactly from TARGET_CONTEXT, including the selected occurrence.",
		Prompt:      variantUserMessage(request),
		Schema:      variantsSchema,
		Temperature: &temperature,
	})
	if retryErr != nil {
		if initialErr != nil {
			return domain.TranslationVariantsResult{}, initialErr
		}
		return initial, nil
	}
	additional, additionalErr := parseVariants(retry, request)
	if initialErr != nil && additionalErr != nil {
		return domain.TranslationVariantsResult{}, additionalErr
	}
	return mergeVariants(initial, additional), nil
}

func normalizeVariantRequest(request domain.TranslationVariantsRequest) domain.TranslationVariantsRequest {
	request.SourceText = strings.TrimSpace(request.SourceText)
	request.TargetContext = strings.TrimSpace(request.TargetContext)
	request.MarkedTargetContext = strings.TrimSpace(request.MarkedTargetContext)
	request.SelectedText = strings.TrimSpace(request.SelectedText)
	request.Language = strings.TrimSpace(request.Language)
	return request
}

func (s *Service) promptSettings() PromptBuilder {
	if s.prompts == nil {
		return NewPromptBuilder(domain.DefaultPromptSettings())
	}
	return NewPromptBuilder(s.prompts())
}

func validateVariantRequest(request domain.TranslationVariantsRequest) error {
	if request.SourceText == "" || request.TargetContext == "" || request.MarkedTargetContext == "" || request.SelectedText == "" || request.Language == "" {
		return fmt.Errorf("source text, target context, marked target context, selected text, and target language are required")
	}
	if utf8.RuneCountInString(request.SourceText) > maxVariantSourceRunes {
		return fmt.Errorf("source text is too long for alternatives")
	}
	if utf8.RuneCountInString(request.TargetContext) > maxVariantContextRunes {
		return fmt.Errorf("target context is too long for alternatives")
	}
	if utf8.RuneCountInString(request.MarkedTargetContext) > maxVariantContextRunes+len(selectionStartMarker)+len(selectionEndMarker) {
		return fmt.Errorf("marked target context is too long for alternatives")
	}
	if utf8.RuneCountInString(request.SelectedText) > maxVariantTermRunes {
		return fmt.Errorf("selected text is too long for alternatives")
	}
	context, selectedRange, err := markedSelection(request.MarkedTargetContext)
	if err != nil {
		return err
	}
	if context != request.TargetContext || context[selectedRange.start:selectedRange.end] != request.SelectedText {
		return fmt.Errorf("marked target context must identify the selected text in target context")
	}
	return nil
}

func variantUserMessage(request domain.TranslationVariantsRequest) string {
	return fmt.Sprintf("Create alternatives for the one marked occurrence. The marker tags are instructions only and are not part of the target text.\n\nTARGET_LANGUAGE:\n%s\n\nSELECTED_TEXT:\n%s\n\nSOURCE_TEXT (meaning reference only):\n%s\n\nTARGET_CONTEXT (copy target phrases only from here):\n%s\n\nMARKED_TARGET_CONTEXT:\n%s", request.Language, request.SelectedText, request.SourceText, request.TargetContext, request.MarkedTargetContext)
}

func parseVariants(response string, request domain.TranslationVariantsRequest) (domain.TranslationVariantsResult, error) {
	_, selectedRange, err := markedSelection(request.MarkedTargetContext)
	if err != nil {
		return domain.TranslationVariantsResult{}, err
	}
	result, err := decodeVariants(response, request.SelectedText)
	if err != nil {
		return domain.TranslationVariantsResult{}, err
	}
	unique := make(map[string]struct{}, len(result.Variants))
	valid := make([]domain.TranslationVariant, 0, len(result.Variants))
	for _, variant := range result.Variants {
		variant.Replacement = strings.TrimSpace(variant.Replacement)
		resolvedTarget, ok := resolveVariantTarget(request.TargetContext, variant.Target, selectedRange)
		if !ok || variant.Replacement == "" || strings.EqualFold(resolvedTarget, variant.Replacement) || !replacementFitsTarget(resolvedTarget, variant.Replacement) {
			continue
		}
		variant.Target = resolvedTarget
		key := strings.ToLower(variant.Target) + "\x00" + strings.ToLower(variant.Replacement)
		if _, exists := unique[key]; exists {
			continue
		}
		unique[key] = struct{}{}
		valid = append(valid, variant)
		if len(valid) == maxVariantCount {
			break
		}
	}
	if len(valid) == 0 {
		return domain.TranslationVariantsResult{}, fmt.Errorf("model returned no usable alternatives")
	}
	return domain.TranslationVariantsResult{Variants: valid}, nil
}

func decodeVariants(response, selectedText string) (domain.TranslationVariantsResult, error) {
	response = strings.TrimSpace(response)
	var result domain.TranslationVariantsResult
	if json.Unmarshal([]byte(response), &result) == nil {
		return result, nil
	}
	if start, end := strings.Index(response, "{"), strings.LastIndex(response, "}"); start >= 0 && end > start && json.Unmarshal([]byte(response[start:end+1]), &result) == nil {
		return result, nil
	}
	var list []domain.TranslationVariant
	if json.Unmarshal([]byte(response), &list) == nil {
		return domain.TranslationVariantsResult{Variants: list}, nil
	}
	if replacements := plainListVariants(response, selectedText); len(replacements) > 0 {
		return domain.TranslationVariantsResult{Variants: replacements}, nil
	}
	return domain.TranslationVariantsResult{}, fmt.Errorf("model returned invalid alternatives data")
}

func plainListVariants(response, selectedText string) []domain.TranslationVariant {
	var variants []domain.TranslationVariant
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if !looksLikeListItem(line) {
			continue
		}
		replacement := strings.TrimSpace(strings.TrimLeft(line, "-*•0123456789. )\t"))
		replacement = strings.Trim(replacement, "\"'«»“”")
		if utf8.RuneCountInString(replacement) == 0 || utf8.RuneCountInString(replacement) > maxVariantTermRunes {
			continue
		}
		variants = append(variants, domain.TranslationVariant{Target: selectedText, Replacement: replacement})
		if len(variants) == maxVariantCount {
			break
		}
	}
	return variants
}

func looksLikeListItem(line string) bool {
	if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "•") {
		return true
	}
	for _, runeValue := range line {
		if unicode.IsDigit(runeValue) {
			continue
		}
		return runeValue == '.' || runeValue == ')'
	}
	return false
}

func mergeVariants(first, second domain.TranslationVariantsResult) domain.TranslationVariantsResult {
	merged := make([]domain.TranslationVariant, 0, maxVariantCount)
	seen := make(map[string]struct{}, len(first.Variants)+len(second.Variants))
	for _, result := range []domain.TranslationVariantsResult{first, second} {
		for _, variant := range result.Variants {
			key := strings.ToLower(variant.Target) + "\x00" + strings.ToLower(variant.Replacement)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, variant)
			if len(merged) == maxVariantCount {
				return domain.TranslationVariantsResult{Variants: merged}
			}
		}
	}
	return domain.TranslationVariantsResult{Variants: merged}
}

type textRange struct{ start, end int }

func markedSelection(marked string) (string, textRange, error) {
	start := strings.Index(marked, selectionStartMarker)
	if start < 0 || strings.Count(marked, selectionStartMarker) != 1 || strings.Count(marked, selectionEndMarker) != 1 {
		return "", textRange{}, fmt.Errorf("marked target context must contain exactly one selection marker pair")
	}
	afterStart := start + len(selectionStartMarker)
	endOffset := strings.Index(marked[afterStart:], selectionEndMarker)
	if endOffset < 0 {
		return "", textRange{}, fmt.Errorf("marked target context has an incomplete selection marker")
	}
	end := afterStart + endOffset
	if end == afterStart {
		return "", textRange{}, fmt.Errorf("marked target context selection cannot be empty")
	}
	context := marked[:start] + marked[afterStart:end] + marked[end+len(selectionEndMarker):]
	return context, textRange{start: start, end: start + end - afterStart}, nil
}

func resolveVariantTarget(context, target string, selected textRange) (string, bool) {
	selectedText := context[selected.start:selected.end]
	for _, candidate := range targetCandidates(target) {
		if matched, ok := overlappingTarget(context, candidate, selected, false); ok {
			return matched, true
		}
		if matched, ok := overlappingTarget(context, candidate, selected, true); ok {
			return matched, true
		}
	}
	if strings.EqualFold(trimPhraseEdges(target), trimPhraseEdges(selectedText)) {
		return selectedText, true
	}
	return "", false
}

func targetCandidates(target string) []string {
	target = strings.TrimSpace(target)
	trimmed := trimPhraseEdges(target)
	if trimmed == "" || trimmed == target {
		return []string{target}
	}
	return []string{target, trimmed}
}

func trimPhraseEdges(value string) string {
	return strings.TrimFunc(strings.TrimSpace(value), func(runeValue rune) bool {
		return unicode.IsPunct(runeValue) || unicode.IsSymbol(runeValue)
	})
}

func overlappingTarget(context, target string, selected textRange, caseInsensitive bool) (string, bool) {
	if target == "" {
		return "", false
	}
	haystack, needle := context, target
	if caseInsensitive {
		haystack, needle = strings.ToLower(context), strings.ToLower(target)
	}
	for index := strings.Index(haystack, needle); index >= 0; {
		end := index + len(needle)
		if index < selected.end && end > selected.start {
			return context[index:end], true
		}
		next := index + len(needle)
		if next >= len(haystack) {
			break
		}
		nextIndex := strings.Index(haystack[next:], needle)
		if nextIndex < 0 {
			break
		}
		index = next + nextIndex
	}
	return "", false
}

func replacementFitsTarget(target, replacement string) bool {
	targetWords := phraseWordCount(target)
	replacementWords := phraseWordCount(replacement)
	if targetWords == 0 || replacementWords == 0 {
		return false
	}
	// A lexical alternative may need a compact phrase, but it cannot be a
	// sentence-level rewrite when the clicked target is only one or two words.
	maxWords := max(8, targetWords+6)
	if replacementWords > maxWords {
		return false
	}
	if targetWords == 1 && strings.Contains(strings.ToLower(replacement), strings.ToLower(target)) {
		return false
	}
	return true
}

func phraseWordCount(value string) int {
	words := 0
	inWord := false
	for _, runeValue := range value {
		word := unicode.IsLetter(runeValue) || unicode.IsNumber(runeValue) || unicode.IsMark(runeValue)
		if word && !inWord {
			words++
		}
		inWord = word
	}
	return words
}

func SplitText(text string, limit int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= limit {
		return []string{text}
	}
	var chunks []string
	remaining := text
	for len(remaining) > limit {
		cut := strings.LastIndexAny(remaining[:limit], "\n.!?; ")
		if cut < limit/2 {
			cut = limit
		}
		chunks = append(chunks, strings.TrimSpace(remaining[:cut]))
		remaining = strings.TrimSpace(remaining[cut:])
	}
	if remaining != "" {
		chunks = append(chunks, remaining)
	}
	return chunks
}
