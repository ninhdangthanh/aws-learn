package chunker

import (
	"testing"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/parser"
)

func TestChunkerCreatesOverlappingChunks(t *testing.T) {
	ch := New(5, 2)

	chunks := ch.Chunk([]parser.PageText{
		{PageNumber: 1, Text: "one two three four five six seven eight nine"},
	})

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}

	if chunks[0].Content != "one two three four five" {
		t.Fatalf("unexpected first chunk: %q", chunks[0].Content)
	}

	if chunks[1].Content != "four five six seven eight" {
		t.Fatalf("unexpected second chunk: %q", chunks[1].Content)
	}

	if chunks[2].Content != "seven eight nine" {
		t.Fatalf("unexpected third chunk: %q", chunks[2].Content)
	}
}
