package parser

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPDFParserParse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.pdf")
	if err := writeSimplePDF(path, []string{
		"hello from page one",
		"hello from page two",
	}); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	p := NewPDFParser()
	pages, err := p.Parse(context.Background(), path)
	if err != nil {
		t.Fatalf("parse pdf: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("expected 2 parsed pages, got %d", len(pages))
	}

	if pages[0].PageNumber != 1 || !strings.Contains(pages[0].Text, "hello from page one") {
		t.Fatalf("unexpected first page: %+v", pages[0])
	}

	if pages[1].PageNumber != 2 || !strings.Contains(pages[1].Text, "hello from page two") {
		t.Fatalf("unexpected second page: %+v", pages[1])
	}
}

func writeSimplePDF(path string, pageTexts []string) error {
	if len(pageTexts) == 0 {
		return fmt.Errorf("pageTexts must not be empty")
	}

	fontID := 3 + len(pageTexts)
	objects := make([]string, 0, 3+len(pageTexts)*2)
	objects = append(objects, "<< /Type /Catalog /Pages 2 0 R >>")

	kids := make([]string, 0, len(pageTexts))
	for i := range pageTexts {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+i))
	}
	objects = append(objects, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pageTexts)))

	for i := range pageTexts {
		pageID := 3 + i
		contentID := fontID + 1 + i
		objects = append(objects, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>",
			fontID,
			contentID,
		))
		_ = pageID
	}

	objects = append(objects, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	for _, text := range pageTexts {
		stream := fmt.Sprintf("BT\n/F1 12 Tf\n72 720 Td\n(%s) Tj\nET", escapePDFText(text))
		objects = append(objects, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}

	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}

	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\n", len(objects)+1)
	fmt.Fprintf(&buf, "startxref\n%d\n%%%%EOF\n", xrefOffset)

	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func escapePDFText(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)")
	return replacer.Replace(value)
}
