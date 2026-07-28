package localization

import (
	"bytes"
	"fmt"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"gopkg.in/yaml.v3"
)

type yamlDocument struct {
	node   yaml.Node
	bom    bool
	ending string
}

func parseYAML(data []byte) (*yamlDocument, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(stripBOM(data), &node); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	if len(node.Content) == 0 {
		return nil, fmt.Errorf("YAML file is empty")
	}
	return &yamlDocument{node: node, bom: hasBOM(data), ending: lineEnding(data)}, nil
}

func (d *yamlDocument) entries() []domain.LocalizationEntry {
	entries := make([]domain.LocalizationEntry, 0)
	collectYAMLEntries(&d.node, nil, &entries)
	return entries
}

func collectYAMLEntries(node *yaml.Node, path []string, entries *[]domain.LocalizationEntry) {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			collectYAMLEntries(child, path, entries)
		}
	case yaml.MappingNode:
		for index := 0; index+1 < len(node.Content); index += 2 {
			collectYAMLEntries(node.Content[index+1], append(path, node.Content[index].Value), entries)
		}
	case yaml.SequenceNode:
		for index, child := range node.Content {
			collectYAMLEntries(child, append(path, fmt.Sprintf("%d", index)), entries)
		}
	case yaml.ScalarNode:
		if node.Tag == "!!str" {
			*entries = append(*entries, domain.LocalizationEntry{ID: pointer(path), Key: displayKey(path), Source: []domain.LocalizationForm{{Category: "other", Text: node.Value}}, Translation: []domain.LocalizationForm{}})
		}
	}
}

func (d *yamlDocument) render(translations map[string][]domain.LocalizationForm, fallback domain.UntranslatedExportMode, _ string) ([]byte, error) {
	applyYAML(&d.node, nil, translations, fallback)
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&d.node); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	data := output.Bytes()
	if d.ending == "\r\n" {
		data = bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
	}
	if d.bom {
		data = append([]byte{0xef, 0xbb, 0xbf}, data...)
	}
	return data, nil
}

func applyYAML(node *yaml.Node, path []string, translations map[string][]domain.LocalizationForm, fallback domain.UntranslatedExportMode) {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			applyYAML(child, path, translations, fallback)
		}
	case yaml.MappingNode:
		for index := 0; index+1 < len(node.Content); index += 2 {
			applyYAML(node.Content[index+1], append(path, node.Content[index].Value), translations, fallback)
		}
	case yaml.SequenceNode:
		for index, child := range node.Content {
			applyYAML(child, append(path, fmt.Sprintf("%d", index)), translations, fallback)
		}
	case yaml.ScalarNode:
		if forms, ok := translations[pointer(path)]; ok && node.Tag == "!!str" {
			node.Value = bestForm(fallbackForms([]domain.LocalizationForm{{Category: "other", Text: node.Value}}, forms, fallback), "other")
		}
	}
}
