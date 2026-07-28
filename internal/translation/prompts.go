package translation

import (
	"strings"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

const variantResponseContract = `

Output contract:
- Return exactly one JSON object and no markdown, labels, or explanation.
- Its exact shape is {"variants":[{"target":"...","replacement":"..."}]}.
- Return 5 to 10 objects in variants.
- target must be copied verbatim from TARGET_CONTEXT as one contiguous phrase, must contain the text enclosed by <alt-selection> and </alt-selection>, and may include adjacent words only when needed for grammar. Preserve the target context's spelling, case, punctuation, and spacing. Never include marker tags in target.
- replacement must be a different natural phrase in {{target_language}} that can replace target directly. Keep its scope tight: do not repeat, paraphrase, or include words from outside target. If target is one word, return a one-word alternative or a short phrase only. Do not return an unchanged replacement and never return an empty list.
- Example of required scope: for target "orbits", "circles" or "moves in orbit" is valid; a replacement that rewrites the rest of the sentence is invalid.
`

// PromptBuilder renders saved system prompts while keeping user content separate.
type PromptBuilder struct {
	settings domain.PromptSettings
}

func NewPromptBuilder(settings domain.PromptSettings) PromptBuilder {
	return PromptBuilder{settings: settings}
}

func (b PromptBuilder) TranslationFor(language string) string {
	return renderLanguage(b.settings.Translation, language)
}

func (b PromptBuilder) VariantsFor(language string) string {
	// Keep the selection contract outside the editable prompt. It prevents a
	// custom prompt from making repeated words or sentence-initial words
	// ambiguous while retaining the user's stylistic instructions.
	return renderLanguage(b.settings.WordVariants, language) + renderLanguage(variantResponseContract, language)
}

func (b PromptBuilder) Detection() string {
	return b.settings.Detection
}

func (b PromptBuilder) OCR() string {
	return b.settings.OCR
}

func renderLanguage(prompt, language string) string {
	return strings.ReplaceAll(prompt, "{{target_language}}", strings.TrimSpace(language))
}
