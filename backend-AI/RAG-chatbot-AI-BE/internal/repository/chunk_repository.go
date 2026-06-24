package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
)

type ChunkRepository struct {
	db *gorm.DB
}

type CreateChunkInput struct {
	DocumentID uuid.UUID
	ChunkIndex int32
	Content    string
	PageNumber *int32
	TokenCount int32
	QdrantID   *uuid.UUID
}

func NewChunkRepository(db *gorm.DB) *ChunkRepository {
	return &ChunkRepository{
		db: db,
	}
}

func (r *ChunkRepository) BulkInsert(ctx context.Context, inputs []CreateChunkInput) ([]model.Chunk, error) {
	chunks := make([]model.Chunk, 0, len(inputs))
	for _, input := range inputs {
		chunks = append(chunks, model.Chunk{
			DocumentID: input.DocumentID,
			ChunkIndex: input.ChunkIndex,
			Content:    input.Content,
			PageNumber: input.PageNumber,
			TokenCount: input.TokenCount,
			QdrantID:   input.QdrantID,
		})
	}

	if len(chunks) == 0 {
		return chunks, nil
	}

	if err := r.db.WithContext(ctx).Create(&chunks).Error; err != nil {
		return nil, err
	}

	return chunks, nil
}

func (r *ChunkRepository) GetByDocument(ctx context.Context, documentID uuid.UUID) ([]model.Chunk, error) {
	var chunks []model.Chunk
	err := r.db.WithContext(ctx).
		Where("document_id = ?", documentID).
		Order("chunk_index ASC").
		Find(&chunks).Error
	return chunks, err
}
