package parser

import (
	"strings"

	"github.com/unidoc/unioffice/document"
)

type DocxParser struct{}

func NewDocxParser() *DocxParser {
	return &DocxParser{}
}

func (d *DocxParser) ExtractText(path string) ([]string, error) {
	doc, err := document.Open(path)
	if err != nil {
		return nil, err
	}
	var pages []string
	var builder strings.Builder
	for _, para := range doc.Paragraphs() {
		for _, run := range para.Runs() {
			builder.WriteString(run.Text())
		}
		builder.WriteString("\n")
	}
	pages = append(pages, builder.String())
	return pages, nil
}
