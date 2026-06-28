package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/chunker"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/parser"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
)

type fakeParser struct {
	pages []parser.PageText
	err   error
}

func (f *fakeParser) Parse(ctx context.Context, path string) ([]parser.PageText, error) {
	return f.pages, f.err
}

type fakeDocumentRepo struct {
	lastUpdate repository.UpdateDocumentStatusInput
}

func (f *fakeDocumentRepo) UpdateStatus(ctx context.Context, input repository.UpdateDocumentStatusInput) (model.Document, error) {
	f.lastUpdate = input
	return model.Document{ID: input.ID, Status: input.Status, ChunkCount: input.ChunkCount}, nil
}

type fakeChunkRepo struct {
	deletedFor uuid.UUID
	lastInputs []repository.CreateChunkInput
}

func (f *fakeChunkRepo) DeleteByDocument(ctx context.Context, documentID uuid.UUID) error {
	f.deletedFor = documentID
	return nil
}

func (f *fakeChunkRepo) BulkInsert(ctx context.Context, inputs []repository.CreateChunkInput) ([]model.Chunk, error) {
	f.lastInputs = inputs
	out := make([]model.Chunk, 0, len(inputs))
	for _, input := range inputs {
		out = append(out, model.Chunk{
			ID:         uuid.New(),
			DocumentID: input.DocumentID,
			ChunkIndex: input.ChunkIndex,
			Content:    input.Content,
			PageNumber: input.PageNumber,
			TokenCount: input.TokenCount,
		})
	}
	return out, nil
}

func TestParseChunkServiceProcess(t *testing.T) {
	documentID := uuid.New()
	documentRepo := &fakeDocumentRepo{}
	chunkRepo := &fakeChunkRepo{}

	svc := NewParseChunkService(
		&fakeParser{
			pages: []parser.PageText{
				{PageNumber: 1, Text: "one two three four five six"},
				{PageNumber: 2, Text: "seven eight nine ten"},
			},
		},
		chunker.New(5, 2),
		documentRepo,
		chunkRepo,
	)

	chunks, err := svc.Process(context.Background(), documentID, "/tmp/demo.pdf")
	if err != nil {
		t.Fatalf("process: %v", err)
	}

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}

	if documentRepo.lastUpdate.Status != model.DocumentStatusChunked {
		t.Fatalf("expected document status chunked, got %q", documentRepo.lastUpdate.Status)
	}

	if documentRepo.lastUpdate.ChunkCount != int32(len(chunks)) {
		t.Fatalf("expected chunk_count %d, got %d", len(chunks), documentRepo.lastUpdate.ChunkCount)
	}

	if len(chunkRepo.lastInputs) != len(chunks) {
		t.Fatalf("expected %d chunk inserts, got %d", len(chunks), len(chunkRepo.lastInputs))
	}

	if chunkRepo.deletedFor != documentID {
		t.Fatalf("expected delete existing chunks for %s, got %s", documentID, chunkRepo.deletedFor)
	}
}
