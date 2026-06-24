package parser

import (
	"context"
	"fmt"
	"strings"

	pdf "github.com/ledongthuc/pdf"
)

type PDFParser struct{}

func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

func (p *PDFParser) Parse(ctx context.Context, path string) ([]PageText, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	file, reader, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer file.Close()

	totalPages := reader.NumPage()
	pages := make([]PageText, 0, totalPages)
	for pageNumber := 1; pageNumber <= totalPages; pageNumber++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		page := reader.Page(pageNumber)
		if page.V.IsNull() || page.V.Key("Contents").Kind() == pdf.Null {
			continue
		}

		rows, err := page.GetTextByRow()
		if err != nil {
			return nil, fmt.Errorf("read pdf page %d: %w", pageNumber, err)
		}

		parts := make([]string, 0)
		for _, row := range rows {
			for _, word := range row.Content {
				parts = append(parts, word.S)
			}
		}

		text := strings.Join(parts, " ")
		cleaned := normalizeWhitespace(text)
		if cleaned == "" {
			continue
		}

		pages = append(pages, PageText{
			PageNumber: pageNumber,
			Text:       cleaned,
		})
	}

	return pages, nil
}

func normalizeWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
