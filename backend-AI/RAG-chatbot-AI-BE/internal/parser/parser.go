package parser

import "context"

type PageText struct {
	PageNumber int
	Text       string
}

type DocumentParser interface {
	Parse(ctx context.Context, path string) ([]PageText, error)
}
