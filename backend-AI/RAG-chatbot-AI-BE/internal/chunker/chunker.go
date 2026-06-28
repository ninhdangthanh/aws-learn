package chunker

import (
	"strings"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/parser"
)

type Chunk struct {
	ChunkIndex int
	PageNumber *int32
	Content    string
	TokenCount int
}

type Chunker struct {
	chunkSize int
	overlap   int
}

func New(chunkSize, overlap int) *Chunker {
	return &Chunker{
		chunkSize: chunkSize,
		overlap:   overlap,
	}
}

type tokenWithPage struct {
	token      string
	pageNumber int
}

func (c *Chunker) Chunk(pages []parser.PageText) []Chunk {
	if len(pages) == 0 {
		return nil
	}

	tokens := make([]tokenWithPage, 0)
	for _, page := range pages {
		for _, token := range strings.Fields(page.Text) {
			tokens = append(tokens, tokenWithPage{
				token:      token,
				pageNumber: page.PageNumber,
			})
		}
	}

	if len(tokens) == 0 {
		return nil
	}

	step := c.chunkSize - c.overlap
	chunks := make([]Chunk, 0, (len(tokens)/step)+1)

	for start := 0; start < len(tokens); start += step {
		end := min(start+c.chunkSize, len(tokens))
		chunkTokens := make([]string, 0, end-start)
		for _, token := range tokens[start:end] {
			chunkTokens = append(chunkTokens, token.token)
		}

		pageNumber := int32(tokens[start].pageNumber)
		chunks = append(chunks, Chunk{
			ChunkIndex: len(chunks),
			PageNumber: &pageNumber,
			Content:    strings.Join(chunkTokens, " "),
			TokenCount: end - start,
		})

		if end == len(tokens) {
			break
		}
	}

	return chunks
}
