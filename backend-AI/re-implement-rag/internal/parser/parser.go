package parser

type Parser interface {
	ExtractText(path string) ([]string, error)
}
