package documents

import "testing"

func TestSupportedExtensions(t *testing.T) {
	for _, path := range []string{"book.PDF", "book.epub", "book.mobi", "book.docx", "sheet.XLSX", "slides.pptx"} {
		if !IsSupported(path) {
			t.Fatalf("expected %s to be supported", path)
		}
	}
	if IsSupported("notes.txt") || IsSupported("archive.zip") {
		t.Fatal("unsupported extensions were accepted")
	}
}
