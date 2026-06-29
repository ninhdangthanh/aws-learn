package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
)

type DocumentRepository struct {
	db *gorm.DB
}

type CreateDocumentInput struct {
	Filename    string
	StoragePath *string
	FileSize    int64
	FileType    string
	Status      model.DocumentStatus
}

type UpdateDocumentStatusInput struct {
	ID         uuid.UUID
	Status     model.DocumentStatus
	ChunkCount int32
	ErrorMsg   *string
}

type ListDocumentsInput struct {
	Status model.DocumentStatus
	Limit  int
	Offset int
}

func NewDocumentRepository(db *gorm.DB) *DocumentRepository {
	return &DocumentRepository{
		db: db,
	}
}

func (r *DocumentRepository) Create(ctx context.Context, input CreateDocumentInput) (model.Document, error) {
	status := input.Status
	if status == "" {
		status = model.DocumentStatusPending
	}

	document := model.Document{
		Filename:    input.Filename,
		StoragePath: input.StoragePath,
		FileSize:    input.FileSize,
		FileType:    input.FileType,
		Status:      status,
	}

	err := r.db.WithContext(ctx).Create(&document).Error
	return document, err
}

func (r *DocumentRepository) Get(ctx context.Context, id uuid.UUID) (model.Document, error) {
	var document model.Document
	err := r.db.WithContext(ctx).First(&document, "id = ?", id).Error
	return document, err
}

func (r *DocumentRepository) List(ctx context.Context, input ListDocumentsInput) ([]model.Document, error) {
	var documents []model.Document
	query := r.db.WithContext(ctx).Order("created_at DESC")
	if input.Status != "" {
		query = query.Where("status = ?", input.Status)
	}
	if input.Limit > 0 {
		query = query.Limit(input.Limit)
	}
	if input.Offset > 0 {
		query = query.Offset(input.Offset)
	}
	err := query.Find(&documents).Error
	return documents, err
}

func (r *DocumentRepository) UpdateStatus(ctx context.Context, input UpdateDocumentStatusInput) (model.Document, error) {
	updates := map[string]any{
		"status":      input.Status,
		"chunk_count": input.ChunkCount,
		"error_msg":   input.ErrorMsg,
	}

	if err := r.db.WithContext(ctx).
		Model(&model.Document{}).
		Where("id = ?", input.ID).
		Updates(updates).Error; err != nil {
		return model.Document{}, err
	}

	return r.Get(ctx, input.ID)
}

func (r *DocumentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.Document{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
