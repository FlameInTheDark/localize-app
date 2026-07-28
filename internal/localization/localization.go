// Package localization parses supported localization sources into one stable
// entry model and writes translated copies without mutating the input file.
package localization

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

const maxFileSize = 20 * 1024 * 1024

type document interface {
	entries() []domain.LocalizationEntry
	render(map[string][]domain.LocalizationForm, domain.UntranslatedExportMode, string) ([]byte, error)
}

// Open parses path according to format, or detects a format when Auto is used.
func Open(path string, format domain.LocalizationFormat) (domain.LocalizationFile, document, error) {
	info, err := os.Stat(path)
	if err != nil {
		return domain.LocalizationFile{}, nil, err
	}
	if info.IsDir() {
		return domain.LocalizationFile{}, nil, fmt.Errorf("select a file, not a directory")
	}
	if info.Size() > maxFileSize {
		return domain.LocalizationFile{}, nil, fmt.Errorf("localization file is too large (maximum %d MB)", maxFileSize/(1024*1024))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.LocalizationFile{}, nil, err
	}
	if !utf8.Valid(stripBOM(data)) {
		return domain.LocalizationFile{}, nil, fmt.Errorf("localization files must be UTF-8 encoded")
	}
	if format == "" || format == domain.LocalizationFormatAuto {
		format, err = DetectFormat(path)
		if err != nil {
			return domain.LocalizationFile{}, nil, err
		}
	}
	doc, err := parse(format, data)
	if err != nil {
		return domain.LocalizationFile{}, nil, err
	}
	fingerprint := sha256.Sum256(data)
	file := domain.LocalizationFile{
		Path:        path,
		Name:        filepath.Base(path),
		Format:      format,
		Fingerprint: hex.EncodeToString(fingerprint[:]),
		Entries:     doc.entries(),
	}
	if len(file.Entries) == 0 {
		return domain.LocalizationFile{}, nil, fmt.Errorf("no translatable string values were found")
	}
	return file, doc, nil
}

func parse(format domain.LocalizationFormat, data []byte) (document, error) {
	switch format {
	case domain.LocalizationFormatI18NextJSON:
		return parseJSON(data)
	case domain.LocalizationFormatYAML:
		return parseYAML(data)
	case domain.LocalizationFormatProperties:
		return parseProperties(data)
	case domain.LocalizationFormatGettextPO:
		return parsePO(data)
	case domain.LocalizationFormatSourceKeyValues:
		return parseKeyValues(data)
	default:
		return nil, fmt.Errorf("unsupported localization format %q", format)
	}
}

// DetectFormat supplies the initial selector value. Ambiguous .txt and .vdf
// files intentionally require the Source KeyValues override.
func DetectFormat(path string) (domain.LocalizationFormat, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return domain.LocalizationFormatI18NextJSON, nil
	case ".yaml", ".yml":
		return domain.LocalizationFormatYAML, nil
	case ".properties":
		return domain.LocalizationFormatProperties, nil
	case ".po":
		return domain.LocalizationFormatGettextPO, nil
	case ".txt", ".vdf":
		return domain.LocalizationFormatSourceKeyValues, nil
	default:
		return "", fmt.Errorf("cannot detect a supported localization format from %s", filepath.Ext(path))
	}
}

func Filters(format domain.LocalizationFormat) []string {
	switch format {
	case domain.LocalizationFormatI18NextJSON:
		return []string{"*.json"}
	case domain.LocalizationFormatYAML:
		return []string{"*.yaml;*.yml"}
	case domain.LocalizationFormatProperties:
		return []string{"*.properties"}
	case domain.LocalizationFormatGettextPO:
		return []string{"*.po"}
	case domain.LocalizationFormatSourceKeyValues:
		return []string{"*.txt;*.vdf"}
	default:
		return []string{"*.json;*.yaml;*.yml;*.properties;*.po;*.txt;*.vdf"}
	}
}

func DefaultFilename(source string, language string) string {
	extension := filepath.Ext(source)
	base := strings.TrimSuffix(filepath.Base(source), extension)
	if language = strings.TrimSpace(language); language != "" {
		return base + "." + strings.ToLower(language) + extension
	}
	return base + ".translated" + extension
}

func Render(path string, format domain.LocalizationFormat, fingerprint string, entries []domain.LocalizationEntry, fallback domain.UntranslatedExportMode, language string) ([]byte, error) {
	file, doc, err := Open(path, format)
	if err != nil {
		return nil, err
	}
	if file.Fingerprint != fingerprint {
		return nil, fmt.Errorf("the source file changed after it was loaded; reload it before saving")
	}
	translations := make(map[string][]domain.LocalizationForm, len(entries))
	for _, entry := range entries {
		if entry.ID != "" {
			translations[entry.ID] = entry.Translation
		}
	}
	if fallback != domain.UntranslatedExportSource && fallback != domain.UntranslatedExportEmpty {
		return nil, fmt.Errorf("choose how untranslated values should be saved")
	}
	return doc.render(translations, fallback, language)
}

func stripBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return data[3:]
	}
	return data
}

func hasBOM(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf
}

func lineEnding(data []byte) string {
	if strings.Contains(string(data), "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func fallbackForms(source []domain.LocalizationForm, target []domain.LocalizationForm, mode domain.UntranslatedExportMode) []domain.LocalizationForm {
	if len(target) > 0 && allFormsPresent(target) {
		return target
	}
	forms := make([]domain.LocalizationForm, len(source))
	for index, form := range source {
		forms[index] = domain.LocalizationForm{Category: form.Category}
		if mode == domain.UntranslatedExportSource {
			forms[index].Text = form.Text
		}
	}
	return forms
}

func allFormsPresent(forms []domain.LocalizationForm) bool {
	if len(forms) == 0 {
		return false
	}
	for _, form := range forms {
		if strings.TrimSpace(form.Text) == "" {
			return false
		}
	}
	return true
}
