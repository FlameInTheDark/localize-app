package localization

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

type poEntry struct {
	comments []string
	context  string
	id       string
	plural   string
	str      string
	strs     []string
}
type poDocument struct {
	items  []poEntry
	bom    bool
	ending string
}

func parsePO(data []byte) (*poDocument, error) {
	text := string(stripBOM(data))
	if lineEnding(data) == "\r\n" {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	blocks := strings.Split(strings.TrimSpace(text), "\n\n")
	doc := &poDocument{items: make([]poEntry, 0, len(blocks)), bom: hasBOM(data), ending: lineEnding(data)}
	for _, block := range blocks {
		entry, err := parsePOEntry(strings.Split(block, "\n"))
		if err != nil {
			return nil, err
		}
		if entry.id != "" || entry.context != "" || len(entry.comments) > 0 || entry.str != "" {
			doc.items = append(doc.items, entry)
		}
	}
	if len(doc.items) == 0 {
		return nil, fmt.Errorf("PO file has no entries")
	}
	return doc, nil
}

func parsePOEntry(lines []string) (poEntry, error) {
	entry := poEntry{}
	field := ""
	index := -1
	appendValue := func(value string) {
		switch field {
		case "msgctxt":
			entry.context += value
		case "msgid":
			entry.id += value
		case "msgid_plural":
			entry.plural += value
		case "msgstr":
			entry.str += value
		case "msgstr-index":
			if index >= 0 {
				for len(entry.strs) <= index {
					entry.strs = append(entry.strs, "")
				}
				entry.strs[index] += value
			}
		}
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			entry.comments = append(entry.comments, line)
			field = ""
			continue
		}
		if strings.HasPrefix(line, "msgctxt ") {
			field, index = "msgctxt", -1
			value, err := poQuoted(strings.TrimPrefix(line, "msgctxt "))
			if err != nil {
				return entry, err
			}
			appendValue(value)
			continue
		}
		if strings.HasPrefix(line, "msgid_plural ") {
			field, index = "msgid_plural", -1
			value, err := poQuoted(strings.TrimPrefix(line, "msgid_plural "))
			if err != nil {
				return entry, err
			}
			appendValue(value)
			continue
		}
		if strings.HasPrefix(line, "msgid ") {
			field, index = "msgid", -1
			value, err := poQuoted(strings.TrimPrefix(line, "msgid "))
			if err != nil {
				return entry, err
			}
			appendValue(value)
			continue
		}
		if strings.HasPrefix(line, "msgstr[") {
			closing := strings.Index(line, "]")
			if closing < 7 || closing+1 >= len(line) {
				return entry, fmt.Errorf("invalid PO plural translation")
			}
			parsed, err := strconv.Atoi(line[7:closing])
			if err != nil {
				return entry, err
			}
			field, index = "msgstr-index", parsed
			value, err := poQuoted(strings.TrimSpace(line[closing+1:]))
			if err != nil {
				return entry, err
			}
			appendValue(value)
			continue
		}
		if strings.HasPrefix(line, "msgstr ") {
			field, index = "msgstr", -1
			value, err := poQuoted(strings.TrimPrefix(line, "msgstr "))
			if err != nil {
				return entry, err
			}
			appendValue(value)
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "\"") {
			value, err := poQuoted(strings.TrimSpace(line))
			if err != nil {
				return entry, err
			}
			appendValue(value)
			continue
		}
		if strings.TrimSpace(line) != "" {
			return entry, fmt.Errorf("invalid PO syntax near %q", line)
		}
	}
	return entry, nil
}

func poQuoted(value string) (string, error) {
	parsed, err := strconv.Unquote(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("decode PO string: %w", err)
	}
	return parsed, nil
}

func (d *poDocument) entries() []domain.LocalizationEntry {
	entries := make([]domain.LocalizationEntry, 0, len(d.items))
	for index, item := range d.items {
		if item.id == "" {
			continue
		}
		key := item.id
		if item.context != "" {
			key = item.context + " · " + key
		}
		entry := domain.LocalizationEntry{ID: fmt.Sprintf("entry:%d", index), Key: key, Translation: []domain.LocalizationForm{}}
		if item.plural != "" {
			entry.Plural = true
			entry.Source = []domain.LocalizationForm{{Category: "one", Text: item.id}, {Category: "other", Text: item.plural}}
			for formIndex, text := range item.strs {
				entry.Translation = append(entry.Translation, domain.LocalizationForm{Category: fmt.Sprintf("%d", formIndex), Text: text})
			}
		} else {
			entry.Source = []domain.LocalizationForm{{Category: "other", Text: item.id}}
			if item.str != "" {
				entry.Translation = []domain.LocalizationForm{{Category: "other", Text: item.str}}
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

func (d *poDocument) render(translations map[string][]domain.LocalizationForm, fallback domain.UntranslatedExportMode, language string) ([]byte, error) {
	profile := PluralProfile{}
	for _, entry := range d.items {
		if entry.plural != "" {
			var err error
			profile, err = PluralProfileFor(language)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	for index := range d.items {
		entry := &d.items[index]
		if entry.id == "" {
			entry.str = updatePOHeader(entry.str, language, profile.PORule)
			continue
		}
		forms, selected := translations[fmt.Sprintf("entry:%d", index)]
		if !selected {
			continue
		}
		completed := allFormsPresent(forms)
		if entry.plural == "" {
			entry.str = bestForm(fallbackForms([]domain.LocalizationForm{{Category: "other", Text: entry.id}}, forms, fallback), "other")
		} else {
			source := []domain.LocalizationForm{{Category: "one", Text: entry.id}, {Category: "other", Text: entry.plural}}
			forms = fallbackForms(source, forms, fallback)
			entry.strs = make([]string, len(profile.POCategories))
			for formIndex, category := range profile.POCategories {
				entry.strs[formIndex] = bestForm(forms, category)
			}
		}
		if completed {
			entry.comments = clearFuzzy(entry.comments)
		}
	}
	var output bytes.Buffer
	for index, entry := range d.items {
		if index > 0 {
			output.WriteString(d.ending)
		}
		for _, comment := range entry.comments {
			output.WriteString(comment)
			output.WriteString(d.ending)
		}
		if entry.context != "" {
			writePOField(&output, "msgctxt", entry.context, d.ending)
		}
		writePOField(&output, "msgid", entry.id, d.ending)
		if entry.plural != "" {
			writePOField(&output, "msgid_plural", entry.plural, d.ending)
			for formIndex, value := range entry.strs {
				writePOField(&output, fmt.Sprintf("msgstr[%d]", formIndex), value, d.ending)
			}
		} else {
			writePOField(&output, "msgstr", entry.str, d.ending)
		}
	}
	data := output.Bytes()
	if d.bom {
		data = append([]byte{0xef, 0xbb, 0xbf}, data...)
	}
	return data, nil
}

func writePOField(output *bytes.Buffer, name, value, ending string) {
	if !strings.Contains(value, "\n") {
		output.WriteString(name)
		output.WriteByte(' ')
		output.WriteString(strconv.Quote(value))
		output.WriteString(ending)
		return
	}
	output.WriteString(name)
	output.WriteString(" \"\"")
	output.WriteString(ending)
	for len(value) > 0 {
		index := strings.Index(value, "\n")
		if index < 0 {
			output.WriteString(strconv.Quote(value))
			output.WriteString(ending)
			break
		}
		output.WriteString(strconv.Quote(value[:index+1]))
		output.WriteString(ending)
		value = value[index+1:]
	}
}

func updatePOHeader(value, language, rule string) string {
	lines := strings.Split(value, "\n")
	foundLanguage, foundPlural := false, false
	for index, line := range lines {
		if strings.HasPrefix(line, "Language: ") {
			lines[index], foundLanguage = "Language: "+strings.ToLower(language), true
		}
		if rule != "" && strings.HasPrefix(line, "Plural-Forms: ") {
			lines[index], foundPlural = "Plural-Forms: "+rule, true
		}
	}
	if !foundLanguage {
		lines = append(lines, "Language: "+strings.ToLower(language))
	}
	if rule != "" && !foundPlural {
		lines = append(lines, "Plural-Forms: "+rule)
	}
	return strings.Join(lines, "\n")
}

func clearFuzzy(comments []string) []string {
	output := make([]string, 0, len(comments))
	for _, comment := range comments {
		if strings.HasPrefix(comment, "#,") {
			fields := strings.Split(strings.TrimPrefix(comment, "#,"), ",")
			kept := make([]string, 0, len(fields))
			for _, field := range fields {
				if strings.TrimSpace(field) != "fuzzy" {
					kept = append(kept, strings.TrimSpace(field))
				}
			}
			if len(kept) > 0 {
				output = append(output, "#, "+strings.Join(kept, ", "))
			}
			continue
		}
		output = append(output, comment)
	}
	return output
}
