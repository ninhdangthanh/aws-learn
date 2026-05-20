package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/ninhdangthanh/rag-chatbot/internal/chunker"
	"github.com/ninhdangthanh/rag-chatbot/internal/config"
	"github.com/ninhdangthanh/rag-chatbot/internal/model"
	"github.com/ninhdangthanh/rag-chatbot/internal/parser"
	"github.com/ninhdangthanh/rag-chatbot/internal/repository"
)

type IngestionService struct {
	db     *repository.Postgres
	qdrant *repository.Qdrant
	cfg    *config.Config
}

func NewIngestionService(db *repository.Postgres, qdrant *repository.Qdrant, cfg *config.Config) *IngestionService {
	return &IngestionService{db: db, qdrant: qdrant, cfg: cfg}
}

func (s *IngestionService) ParseDocument(ctx context.Context, documentID string, filePath string) error {
	doc, err := s.db.GetDocumentByID(ctx, documentID)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document %s not found", documentID)
	}

	if err := s.db.UpdateDocumentStatus(ctx, documentID, model.StatusParsing, ""); err != nil {
		return err
	}

	var parserImpl parser.Parser
	if doc.FileType == "docx" {
		parserImpl = parser.NewDocxParser()
	} else {
		parserImpl = parser.NewPDFParser()
	}

	pages, err := parserImpl.ExtractText(filePath)
	if err != nil {
		s.db.UpdateDocumentStatus(ctx, documentID, model.StatusFailed, err.Error())
		return err
	}

	text := strings.Join(pages, "\n")
	chunks := chunker.ChunkText(text, s.cfg.ChunkSize, s.cfg.ChunkOverlap)
	records := make([]model.Chunk, 0, len(chunks))
	for i, chunkText := range chunks {
		records = append(records, model.Chunk{
			ID:         uuid.New().String(),
			DocumentID: documentID,
			ChunkIndex: i,
			Content:    chunkText,
			PageNumber: 0,
			TokenCount: len(strings.Split(chunkText, " ")),
		})
	}

	if err := s.db.CreateChunks(ctx, records); err != nil {
		s.db.UpdateDocumentStatus(ctx, documentID, model.StatusFailed, err.Error())
		return err
	}

	if err := s.db.SetDocumentChunkCount(ctx, documentID, len(records)); err != nil {
		return err
	}
	if err := s.db.UpdateDocumentStatus(ctx, documentID, model.StatusChunked, ""); err != nil {
		return err
	}

	return nil
}

func (s *IngestionService) EnqueueEmbedTask(ctx context.Context, client *asynq.Client, documentID string) error {
	payload, err := json.Marshal(map[string]interface{}{"document_id": documentID})
	if err != nil {
		return err
	}
	task := asynq.NewTask("embed:chunks", payload)
	_, err = client.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(15*time.Minute))
	return err
}
