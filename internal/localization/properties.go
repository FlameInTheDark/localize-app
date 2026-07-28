package localization

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

type propertyLine struct {
	raw, key, value, prefix string
	entry                   bool
}
type propertiesDocument struct {
	lines  []propertyLine
	bom    bool
	ending string
}

func parseProperties(data []byte) (*propertiesDocument, error) {
	text := string(stripBOM(data))
	ending := lineEnding(data)
	if ending == "\r\n" {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	rawLines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	doc := &propertiesDocument{lines: make([]propertyLine, 0, len(rawLines)), bom: hasBOM(data), ending: ending}
	for _, raw := range rawLines {
		line := propertyLine{raw: raw}
		trimmed := strings.TrimSpace(raw)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "!") {
			if index := propertySeparator(raw); index >= 0 {
				line.key = strings.TrimSpace(raw[:index])
				line.prefix = raw[:index+1]
				line.value = decodeProperties(strings.TrimLeft(raw[index+1:], " \t"))
				line.entry = line.key != ""
			} else if trimmed != "" {
				line.key, line.prefix, line.value, line.entry = trimmed, trimmed+"=", "", true
			}
		}
		doc.lines = append(doc.lines, line)
	}
	return doc, nil
}

func (d *propertiesDocument) entries() []domain.LocalizationEntry {
	entries := make([]domain.LocalizationEntry, 0)
	for index, line := range d.lines {
		if line.entry {
			entries = append(entries, domain.LocalizationEntry{ID: fmt.Sprintf("line:%d", index), Key: line.key, Source: []domain.LocalizationForm{{Category: "other", Text: line.value}}, Translation: []domain.LocalizationForm{}})
		}
	}
	return entries
}

func (d *propertiesDocument) render(translations map[string][]domain.LocalizationForm, fallback domain.UntranslatedExportMode, _ string) ([]byte, error) {
	lines := make([]string, len(d.lines))
	for index, line := range d.lines {
		lines[index] = line.raw
		if forms, ok := translations[fmt.Sprintf("line:%d", index)]; ok && line.entry {
			text := bestForm(fallbackForms([]domain.LocalizationForm{{Category: "other", Text: line.value}}, forms, fallback), "other")
			lines[index] = line.prefix + escapeProperties(text)
		}
	}
	data := []byte(strings.Join(lines, d.ending) + d.ending)
	if d.bom {
		data = append([]byte{0xef, 0xbb, 0xbf}, data...)
	}
	return data, nil
}

func propertySeparator(value string) int {
	escaped := false
	for index, runeValue := range value {
		if escaped {
			escaped = false
			continue
		}
		if runeValue == '\\' {
			escaped = true
			continue
		}
		if runeValue == '=' || runeValue == ':' || runeValue == ' ' || runeValue == '\t' {
			return index
		}
	}
	return -1
}

func decodeProperties(value string) string {
	var output strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) {
			output.WriteByte(value[index])
			continue
		}
		index++
		switch value[index] {
		case 'n':
			output.WriteByte('\n')
		case 'r':
			output.WriteByte('\r')
		case 't':
			output.WriteByte('\t')
		case 'f':
			output.WriteByte('\f')
		case 'u':
			if index+4 < len(value) {
				if parsed, err := strconv.ParseUint(value[index+1:index+5], 16, 16); err == nil {
					output.WriteRune(rune(parsed))
					index += 4
					continue
				}
			}
			output.WriteString("\\u")
		default:
			output.WriteByte(value[index])
		}
	}
	return output.String()
}

func escapeProperties(value string) string {
	var output strings.Builder
	for index, runeValue := range value {
		switch runeValue {
		case '\\':
			output.WriteString("\\\\")
		case '\n':
			output.WriteString("\\n")
		case '\r':
			output.WriteString("\\r")
		case '\t':
			output.WriteString("\\t")
		case '=', ':', '#', '!':
			output.WriteByte('\\')
			output.WriteRune(runeValue)
		case ' ':
			if index == 0 {
				output.WriteString("\\ ")
			} else {
				output.WriteRune(runeValue)
			}
		default:
			if runeValue > 0x7f {
				if runeValue > 0xffff {
					high, low := utf16.EncodeRune(runeValue)
					_, _ = fmt.Fprintf(&output, "\\u%04X\\u%04X", high, low)
				} else {
					_, _ = fmt.Fprintf(&output, "\\u%04X", runeValue)
				}
			} else {
				output.WriteRune(runeValue)
			}
		}
	}
	return output.String()
}
