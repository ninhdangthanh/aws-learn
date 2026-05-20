package parser

import (
	"github.com/ledongthuc/pdf"
)

type PDFParser struct{}

func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

func (p *PDFParser) ExtractText(path string) ([]string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pages []string
	total := r.NumPage()
	for i := 1; i <= total; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		pages = append(pages, text)
	}
	return pages, nil
}
