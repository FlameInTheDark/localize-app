package documents

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

var supportedExtensions = map[string]struct{}{".pdf": {}, ".epub": {}, ".mobi": {}, ".docx": {}, ".xlsx": {}, ".pptx": {}}

type Extractor struct{ mutool string }

func New(mutool string) *Extractor { return &Extractor{mutool: mutool} }

func IsSupported(path string) bool {
	_, ok := supportedExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

func (e *Extractor) Extract(ctx context.Context, path string) ([]domain.DocumentSegment, error) {
	if !IsSupported(path) {
		return nil, fmt.Errorf("unsupported document type; use PDF, EPUB, MOBI, DOCX, XLSX, or PPTX")
	}
	if _, err := os.Stat(e.mutool); err != nil {
		return nil, fmt.Errorf("MuPDF runtime is unavailable: %w", err)
	}
	work, err := os.MkdirTemp("", "localize-document-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(work) }()
	pattern := filepath.Join(work, "section-%d.txt")
	command := exec.CommandContext(ctx, e.mutool, "draw", "-F", "txt", "-o", pattern, path)
	if output, err := command.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("extract document text: %w: %s", err, strings.TrimSpace(string(output)))
	}
	files, err := filepath.Glob(filepath.Join(work, "section-*.txt"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	segments := make([]domain.DocumentSegment, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		text := strings.TrimSpace(string(data))
		if text != "" {
			segments = append(segments, domain.DocumentSegment{Ordinal: len(segments) + 1, Original: text})
		}
	}
	return segments, nil
}

func (e *Extractor) Render(ctx context.Context, path string) ([]string, error) {
	if !IsSupported(path) {
		return nil, fmt.Errorf("unsupported document type")
	}
	work, err := os.MkdirTemp("", "localize-render-*")
	if err != nil {
		return nil, err
	}
	pattern := filepath.Join(work, "page-%d.png")
	command := exec.CommandContext(ctx, e.mutool, "draw", "-r", "150", "-o", pattern, path)
	if output, err := command.CombinedOutput(); err != nil {
		_ = os.RemoveAll(work)
		return nil, fmt.Errorf("render document pages: %w: %s", err, strings.TrimSpace(string(output)))
	}
	files, err := filepath.Glob(filepath.Join(work, "page-*.png"))
	if err != nil {
		_ = os.RemoveAll(work)
		return nil, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		_ = os.RemoveAll(work)
		return nil, fmt.Errorf("MuPDF did not render any pages")
	}
	// Caller owns cleanup: page paths live in a unique temp directory.
	return files, nil
}
