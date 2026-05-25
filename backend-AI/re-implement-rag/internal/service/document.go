package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/ninhdangthanh/rag-chatbot/internal/config"
	"github.com/ninhdangthanh/rag-chatbot/internal/model"
	"github.com/ninhdangthanh/rag-chatbot/internal/repository"
)

type DocumentService struct {
	db     *repository.Postgres
	qdrant *repository.Qdrant
	asynq  *asynq.Client
	cfg    *config.Config
}

func NewDocumentService(db *repository.Postgres, qdrant *repository.Qdrant, asynqClient *asynq.Client, cfg *config.Config) *DocumentService {
	return &DocumentService{db: db, qdrant: qdrant, asynq: asynqClient, cfg: cfg}
}

func (s *DocumentService) UploadDocument(ctx context.Context, fileHeader *multipart.FileHeader) (*model.Document, string, error) {
	filename := filepath.Base(fileHeader.Filename)
	ext := strings.ToLower(filepath.Ext(filename))
	fileType := "pdf"
	if ext == ".docx" {
		fileType = "docx"
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	uploadDir := "uploads"
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, "", err
	}
	fileID := uuid.New().String()
	path := filepath.Join(uploadDir, fmt.Sprintf("%s%s", fileID, ext))
	out, err := os.Create(path)
	if err != nil {
		return nil, "", err
	}
	defer out.Close()

	size, err := io.Copy(out, file)
	if err != nil {
		return nil, "", err
	}

	doc, err := s.db.CreateDocument(ctx, filename, size, fileType)
	if err != nil {
		return nil, "", err
	}

	payload, err := json.Marshal(map[string]interface{}{"document_id": doc.ID, "file_path": path})
	if err != nil {
		return nil, "", err
	}
	task := asynq.NewTask("parse:document", payload)
	if _, err := s.asynq.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(10*time.Minute)); err != nil {
		return nil, "", err
	}

	return doc, path, nil
}

func (s *DocumentService) GetDocument(ctx context.Context, id string) (*model.Document, error) {
	return s.db.GetDocumentByID(ctx, id)
}

func (s *DocumentService) ListDocuments(ctx context.Context, status string, page, limit int) ([]*model.Document, error) {
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}
	return s.db.ListDocuments(ctx, status, limit, offset)
}

func (s *DocumentService) DeleteDocument(ctx context.Context, id string) error {
	docs, err := s.db.GetDocumentByID(ctx, id)
	if err != nil {
		return err
	}
	if docs == nil {
		return fmt.Errorf("document not found")
	}
	if err := s.db.DeleteDocument(ctx, id); err != nil {
		return err
	}
	return nil
}
