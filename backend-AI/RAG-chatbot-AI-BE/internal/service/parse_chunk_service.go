package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/chunker"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/parser"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
)

type documentStatusUpdater interface {
	UpdateStatus(ctx context.Context, input repository.UpdateDocumentStatusInput) (model.Document, error)
}

type chunkBulkInserter interface {
	DeleteByDocument(ctx context.Context, documentID uuid.UUID) error
	BulkInsert(ctx context.Context, inputs []repository.CreateChunkInput) ([]model.Chunk, error)
}

type ParseChunkService struct {
	parser       parser.DocumentParser
	chunker      *chunker.Chunker
	documentRepo documentStatusUpdater
	chunkRepo    chunkBulkInserter
}

func NewParseChunkService(
	parser parser.DocumentParser,
	chunker *chunker.Chunker,
	documentRepo documentStatusUpdater,
	chunkRepo chunkBulkInserter,
) *ParseChunkService {
	return &ParseChunkService{
		parser:       parser,
		chunker:      chunker,
		documentRepo: documentRepo,
		chunkRepo:    chunkRepo,
	}
}

func (s *ParseChunkService) Process(ctx context.Context, documentID uuid.UUID, filePath string) ([]model.Chunk, error) {
	pages, err := s.parser.Parse(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("parse document: %w", err)
	}

	chunks := s.chunker.Chunk(pages)
	inputs := make([]repository.CreateChunkInput, 0, len(chunks))
	for _, chunk := range chunks {
		inputs = append(inputs, repository.CreateChunkInput{
			DocumentID: documentID,
			ChunkIndex: int32(chunk.ChunkIndex),
			Content:    chunk.Content,
			PageNumber: chunk.PageNumber,
			TokenCount: int32(chunk.TokenCount),
		})
	}

	if err := s.chunkRepo.DeleteByDocument(ctx, documentID); err != nil {
		return nil, fmt.Errorf("delete existing chunks: %w", err)
	}

	savedChunks, err := s.chunkRepo.BulkInsert(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("save chunks: %w", err)
	}

	if _, err := s.documentRepo.UpdateStatus(ctx, repository.UpdateDocumentStatusInput{
		ID:         documentID,
		Status:     model.DocumentStatusChunked,
		ChunkCount: int32(len(savedChunks)),
	}); err != nil {
		return nil, fmt.Errorf("update document status: %w", err)
	}

	return savedChunks, nil
}
