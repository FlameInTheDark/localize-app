package localization

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

type kvToken struct {
	value      string
	start, end int
	quoted     bool
	brace      byte
}
type kvNode struct {
	key      string
	keyToken kvToken
	value    *kvToken
	children []*kvNode
}
type keyValuesDocument struct {
	data        []byte
	roots       []*kvNode
	encoding    keyValuesEncoding
	steamRoot   *kvNode
	steamLocale *kvNode
}

type keyValuesEncoding uint8

const (
	keyValuesUTF8 keyValuesEncoding = iota
	keyValuesUTF8BOM
	keyValuesUTF16LE
	keyValuesUTF16BE
)

func parseKeyValues(data []byte) (*keyValuesDocument, error) {
	content, encoding, err := decodeKeyValues(data)
	if err != nil {
		return nil, err
	}
	tokens, err := scanKeyValues(content)
	if err != nil {
		return nil, err
	}
	position := 0
	roots, err := parseKVBlock(tokens, &position, false)
	if err != nil {
		return nil, err
	}
	doc := &keyValuesDocument{data: append([]byte(nil), content...), roots: roots, encoding: encoding}
	if doc.languageNode() != nil && doc.tokensNode() != nil {
		return doc, nil
	}
	for _, node := range roots {
		if strings.EqualFold(node.key, "localization") && len(node.children) > 0 {
			for _, child := range node.children {
				if len(child.children) > 0 {
					doc.steamRoot, doc.steamLocale = node, child
					return doc, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("unsupported KeyValues localization layout; expected Source lang/Tokens or Steam localization/<language> structure")
}

func decodeKeyValues(data []byte) ([]byte, keyValuesEncoding, error) {
	switch {
	case len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe:
		content, err := decodeUTF16(data[2:], binary.LittleEndian)
		return content, keyValuesUTF16LE, err
	case len(data) >= 2 && data[0] == 0xfe && data[1] == 0xff:
		content, err := decodeUTF16(data[2:], binary.BigEndian)
		return content, keyValuesUTF16BE, err
	case utf8.Valid(stripBOM(data)):
		if hasBOM(data) {
			return append([]byte(nil), stripBOM(data)...), keyValuesUTF8BOM, nil
		}
		return append([]byte(nil), data...), keyValuesUTF8, nil
	default:
		return nil, keyValuesUTF8, fmt.Errorf("source KeyValues files must be UTF-8 or UTF-16 encoded with a byte-order mark")
	}
}

func decodeUTF16(data []byte, order binary.ByteOrder) ([]byte, error) {
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("UTF-16 KeyValues file has an odd byte length")
	}
	units := make([]uint16, len(data)/2)
	for index := range units {
		units[index] = order.Uint16(data[index*2:])
	}
	return []byte(string(utf16.Decode(units))), nil
}

func encodeKeyValues(data []byte, encoding keyValuesEncoding) []byte {
	switch encoding {
	case keyValuesUTF8BOM:
		return append([]byte{0xef, 0xbb, 0xbf}, data...)
	case keyValuesUTF16LE, keyValuesUTF16BE:
		output := make([]byte, 2, 2+len(data)*2)
		var order binary.ByteOrder = binary.LittleEndian
		if encoding == keyValuesUTF16LE {
			output[0], output[1] = 0xff, 0xfe
		} else {
			output[0], output[1], order = 0xfe, 0xff, binary.BigEndian
		}
		for _, unit := range utf16.Encode([]rune(string(data))) {
			var encoded [2]byte
			order.PutUint16(encoded[:], unit)
			output = append(output, encoded[:]...)
		}
		return output
	default:
		return data
	}
}

func scanKeyValues(data []byte) ([]kvToken, error) {
	tokens := make([]kvToken, 0)
	for index := 0; index < len(data); {
		if data[index] == '/' && index+1 < len(data) && data[index+1] == '/' {
			for index < len(data) && data[index] != '\n' {
				index++
			}
			continue
		}
		if data[index] == ' ' || data[index] == '\t' || data[index] == '\r' || data[index] == '\n' {
			index++
			continue
		}
		if data[index] == '{' || data[index] == '}' {
			tokens = append(tokens, kvToken{start: index, end: index + 1, brace: data[index]})
			index++
			continue
		}
		if data[index] == '"' {
			start := index
			index++
			var output strings.Builder
			for index < len(data) && data[index] != '"' {
				if data[index] == '\\' && index+1 < len(data) {
					index++
					switch data[index] {
					case 'n':
						output.WriteByte('\n')
					case 't':
						output.WriteByte('\t')
					default:
						output.WriteByte(data[index])
					}
					index++
					continue
				}
				output.WriteByte(data[index])
				index++
			}
			if index >= len(data) {
				return nil, fmt.Errorf("unterminated KeyValues string")
			}
			index++
			tokens = append(tokens, kvToken{value: output.String(), start: start, end: index, quoted: true})
			continue
		}
		start := index
		for index < len(data) && !strings.ContainsRune(" \t\r\n{}", rune(data[index])) {
			index++
		}
		tokens = append(tokens, kvToken{value: string(data[start:index]), start: start, end: index})
	}
	return tokens, nil
}

func parseKVBlock(tokens []kvToken, position *int, closing bool) ([]*kvNode, error) {
	var nodes []*kvNode
	for *position < len(tokens) {
		if tokens[*position].brace == '}' {
			if !closing {
				return nil, fmt.Errorf("unexpected KeyValues closing brace")
			}
			*position++
			return nodes, nil
		}
		if tokens[*position].brace != 0 {
			return nil, fmt.Errorf("expected KeyValues key")
		}
		key := tokens[*position]
		*position++
		if *position >= len(tokens) {
			return nil, fmt.Errorf("missing value for KeyValues key %q", key.value)
		}
		node := &kvNode{key: key.value, keyToken: key}
		switch tokens[*position].brace {
		case '{':
			*position++
			children, err := parseKVBlock(tokens, position, true)
			if err != nil {
				return nil, err
			}
			node.children = children
		case 0:
			value := tokens[*position]
			node.value = &value
			*position++
		default:
			return nil, fmt.Errorf("missing value for KeyValues key %q", key.value)
		}
		nodes = append(nodes, node)
	}
	if closing {
		return nil, fmt.Errorf("missing KeyValues closing brace")
	}
	return nodes, nil
}

func (d *keyValuesDocument) langRoot() *kvNode {
	for _, node := range d.roots {
		if strings.EqualFold(node.key, "lang") {
			return node
		}
	}
	return nil
}
func (d *keyValuesDocument) languageNode() *kvNode {
	root := d.langRoot()
	if root == nil {
		return nil
	}
	for _, node := range root.children {
		if strings.EqualFold(node.key, "Language") && node.value != nil {
			return node
		}
	}
	return nil
}
func (d *keyValuesDocument) tokensNode() *kvNode {
	root := d.langRoot()
	if root == nil {
		return nil
	}
	for _, node := range root.children {
		if strings.EqualFold(node.key, "Tokens") {
			return node
		}
	}
	return nil
}

func (d *keyValuesDocument) entries() []domain.LocalizationEntry {
	entries := make([]domain.LocalizationEntry, 0)
	collectKVEntries(d.translationRoot(), nil, &entries)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries
}

func (d *keyValuesDocument) translationRoot() *kvNode {
	if tokens := d.tokensNode(); tokens != nil {
		return tokens
	}
	return d.steamLocale
}

func collectKVEntries(node *kvNode, path []string, entries *[]domain.LocalizationEntry) {
	if node == nil {
		return
	}
	if node.value != nil {
		*entries = append(*entries, domain.LocalizationEntry{ID: fmt.Sprintf("token:%d", node.value.start), Key: displayKey(path), Source: []domain.LocalizationForm{{Category: "other", Text: node.value.value}}, Translation: []domain.LocalizationForm{}})
		return
	}
	for _, child := range node.children {
		collectKVEntries(child, append(path, child.key), entries)
	}
}

func (d *keyValuesDocument) render(translations map[string][]domain.LocalizationForm, fallback domain.UntranslatedExportMode, language string) ([]byte, error) {
	replacements := map[int]replacement{}
	if languageNode := d.languageNode(); languageNode != nil && languageNode.value != nil {
		name, ok := sourceLanguageNames[strings.ToLower(language)]
		if !ok {
			return nil, fmt.Errorf("source KeyValues does not define a language name for %q", language)
		}
		replacements[languageNode.value.start] = replacement{end: languageNode.value.end, text: quoteKV(name)}
	}
	if d.steamLocale != nil {
		replacements[d.steamLocale.keyToken.start] = replacement{end: d.steamLocale.keyToken.end, text: quoteKV(steamLanguageName(language))}
	}
	var leaves []*kvNode
	collectKVLeaves(d.translationRoot(), &leaves)
	for _, node := range leaves {
		forms, selected := translations[fmt.Sprintf("token:%d", node.value.start)]
		if !selected {
			continue
		}
		text := bestForm(fallbackForms([]domain.LocalizationForm{{Category: "other", Text: node.value.value}}, forms, fallback), "other")
		replacements[node.value.start] = replacement{end: node.value.end, text: quoteKV(text)}
	}
	return encodeKeyValues(applyReplacements(d.data, replacements), d.encoding), nil
}

type replacement struct {
	end  int
	text string
}

func collectKVLeaves(node *kvNode, leaves *[]*kvNode) {
	if node == nil {
		return
	}
	if node.value != nil {
		*leaves = append(*leaves, node)
		return
	}
	for _, child := range node.children {
		collectKVLeaves(child, leaves)
	}
}
func applyReplacements(source []byte, replacements map[int]replacement) []byte {
	starts := make([]int, 0, len(replacements))
	for start := range replacements {
		starts = append(starts, start)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(starts)))
	output := append([]byte(nil), source...)
	for _, start := range starts {
		item := replacements[start]
		output = append(output[:start], append([]byte(item.text), output[item.end:]...)...)
	}
	return output
}
func quoteKV(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\t", "\\t")
	return "\"" + value + "\""
}

var sourceLanguageNames = map[string]string{"en": "English", "es": "Spanish", "fr": "French", "de": "German", "it": "Italian", "pt": "Portuguese", "ru": "Russian", "uk": "Ukrainian", "pl": "Polish", "tr": "Turkish", "ar": "Arabic", "hi": "Hindi", "zh": "schinese", "ja": "Japanese", "ko": "Korean", "vi": "Vietnamese", "th": "Thai", "id": "Indonesian", "nl": "Dutch", "sv": "Swedish", "no": "Norwegian", "da": "Danish", "fi": "Finnish", "cs": "Czech", "ro": "Romanian", "el": "Greek", "he": "Hebrew"}

func steamLanguageName(language string) string {
	if name, ok := steamLanguageNames[strings.ToLower(language)]; ok {
		return name
	}
	return strings.ToLower(language)
}

var steamLanguageNames = map[string]string{"en": "english", "es": "spanish", "fr": "french", "de": "german", "it": "italian", "pt": "brazilian", "ru": "russian", "uk": "ukrainian", "pl": "polish", "tr": "turkish", "zh": "schinese", "ja": "japanese", "ko": "koreana", "nl": "dutch", "sv": "swedish", "no": "norwegian", "da": "danish", "fi": "finnish", "cs": "czech", "ro": "romanian", "el": "greek", "he": "hebrew", "th": "thai", "vi": "vietnamese", "id": "indonesian"}
