package domain

import (
	"fmt"
	"strings"
)

const MaxPromptLength = 8000

// LegacyWordVariantsPrompt is retained only to migrate the prior built-in
// prompt without overwriting a user-customized alternatives prompt.
const LegacyWordVariantsPrompt = "You are a lexical translation assistant. Use SOURCE_TEXT only as semantic reference and TARGET_CONTEXT as grammatical context. Return 3 to 6 distinct natural replacements in {{target_language}}. Each target must be an exact contiguous phrase from TARGET_CONTEXT, include SELECTED_TEXT, and may include adjacent words only when grammar requires it. Each replacement must naturally replace its target while preserving meaning, grammar, agreement, and register. Exclude an unchanged replacement. Return only JSON matching the supplied schema."

// PreviousWordVariantsPrompt is the default used before version 5. It remains
// exported only so persisted built-in prompts can be migrated without touching
// a prompt the user customised themselves.
const PreviousWordVariantsPrompt = "You are a lexical translation assistant. Use SOURCE_TEXT only as semantic reference and TARGET_CONTEXT as grammatical context. MARKED_TARGET_CONTEXT contains exactly one selected occurrence, enclosed by the literal markers <alt-selection> and </alt-selection>. The markers are instructions only, never translated text. Return 3 to 6 distinct natural replacements in {{target_language}}. Each target must be an exact contiguous unmarked phrase from TARGET_CONTEXT, overlap the marked selection, and may include adjacent words only when grammar requires it. Never include marker tags in target or replacement. Each replacement must naturally replace its target while preserving meaning, grammar, agreement, and register. Exclude an unchanged replacement. Return only JSON matching the supplied schema."

const defaultWordVariantsPrompt = "You are a professional translation editor. Produce 5 to 10 distinct, click-to-apply contextual alternatives in {{target_language}}. Always provide alternatives: when a one-word synonym would be weak, vary a short phrase containing the selected text, its grammatical form, wording, register, or local construction. Prefer replacing exactly the selected text; expand target only when grammar makes it necessary. The replacement must have the same local scope as target: never repeat or rewrite surrounding sentence material that is not in target. Use SOURCE_TEXT only to preserve intended meaning; use TARGET_CONTEXT for grammar. MARKED_TARGET_CONTEXT identifies exactly the clicked occurrence. Every replacement must read naturally in the target context, preserve meaning and register, and differ from the selected wording."

// PromptSettings contains system instructions shared by both local providers.
type PromptSettings struct {
	Translation  string `json:"translation"`
	Detection    string `json:"detection"`
	OCR          string `json:"ocr"`
	WordVariants string `json:"wordVariants"`
}

func DefaultPromptSettings() PromptSettings {
	return PromptSettings{
		Translation:  "You are a translation engine. Translate the entire user content into {{target_language}}. Preserve meaning, tone, names, structure, and line breaks. Return only the translated text; never add commentary, markdown fences, or labels.",
		Detection:    "Detect every natural language present in the user content. Return only comma-separated ISO 639-1 codes, for example en,fr. Do not add commentary.",
		OCR:          "Transcribe only the text visible in the image. Preserve paragraphs and reading order. Return only the transcription with no commentary.",
		WordVariants: defaultWordVariantsPrompt,
	}
}

// Validate ensures a saved prompt cannot remove a required model instruction.
func (p PromptSettings) Validate() error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"translation", p.Translation},
		{"detection", p.Detection},
		{"OCR", p.OCR},
		{"word variants", p.WordVariants},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s prompt is required", field.name)
		}
		if len([]rune(field.value)) > MaxPromptLength {
			return fmt.Errorf("%s prompt must not exceed %d characters", field.name, MaxPromptLength)
		}
	}
	return nil
}
