package model

import (
	"time"

	"github.com/google/uuid"
)

type Chunk struct {
	ID         uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	DocumentID uuid.UUID  `gorm:"type:uuid;not null;index" json:"document_id"`
	ChunkIndex int32      `gorm:"not null" json:"chunk_index"`
	Content    string     `gorm:"type:text;not null" json:"content"`
	PageNumber *int32     `json:"page_number"`
	TokenCount int32      `gorm:"not null" json:"token_count"`
	QdrantID   *uuid.UUID `gorm:"type:uuid" json:"qdrant_id"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (Chunk) TableName() string {
	return "chunks"
}
