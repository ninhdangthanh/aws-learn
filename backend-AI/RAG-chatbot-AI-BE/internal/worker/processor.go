package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/tasks"
)

type documentStatusRepository interface {
	Get(ctx context.Context, id uuid.UUID) (model.Document, error)
	UpdateStatus(ctx context.Context, input repository.UpdateDocumentStatusInput) (model.Document, error)
}

type parseChunkProcessor interface {
	Process(ctx context.Context, documentID uuid.UUID, filePath string) ([]model.Chunk, error)
}

type TaskProcessor struct {
	parseService parseChunkProcessor
	documentRepo documentStatusRepository
}

func NewTaskProcessor(parseService parseChunkProcessor, documentRepo documentStatusRepository) *TaskProcessor {
	return &TaskProcessor{
		parseService: parseService,
		documentRepo: documentRepo,
	}
}

func (p *TaskProcessor) ProcessDocumentParseTask(ctx context.Context, task *asynq.Task) error {
	startedAt := time.Now()

	var payload tasks.DocumentParsePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		log.Printf("failed to decode parse document task payload: type=%s err=%v", task.Type(), err)
		return fmt.Errorf("unmarshal parse task payload: %w", err)
	}

	documentID, err := tasks.ParseDocumentID(payload.DocumentID)
	if err != nil {
		log.Printf("invalid parse document task payload: type=%s document_id=%s err=%v", task.Type(), payload.DocumentID, err)
		return fmt.Errorf("parse document id: %w", err)
	}

	log.Printf(
		"started parse document task: type=%s document_id=%s file_path=%s",
		task.Type(),
		documentID,
		payload.FilePath,
	)

	document, err := p.documentRepo.Get(ctx, documentID)
	if err != nil {
		log.Printf("failed to load document for parse task: document_id=%s err=%v", documentID, err)
		return fmt.Errorf("get document for parse task: %w", err)
	}

	if document.Status == "chunked" {
		log.Printf(
			"skipped parse document task because document is already chunked: document_id=%s duration=%s",
			documentID,
			time.Since(startedAt),
		)
		return nil
	}

	log.Printf("marking document as parsing: document_id=%s previous_status=%s", documentID, document.Status)
	if _, err := p.documentRepo.UpdateStatus(ctx, repository.UpdateDocumentStatusInput{
		ID:     documentID,
		Status: "parsing",
	}); err != nil {
		log.Printf("failed to mark document as parsing: document_id=%s err=%v", documentID, err)
		return fmt.Errorf("mark document parsing: %w", err)
	}

	chunks, err := p.parseService.Process(ctx, documentID, payload.FilePath)
	if err != nil {
		log.Printf("failed to process parse document task: document_id=%s file_path=%s err=%v", documentID, payload.FilePath, err)
		message := err.Error()
		if _, updateErr := p.documentRepo.UpdateStatus(ctx, repository.UpdateDocumentStatusInput{
			ID:       documentID,
			Status:   "failed",
			ErrorMsg: &message,
		}); updateErr != nil {
			log.Printf("failed to mark document as failed: document_id=%s err=%v", documentID, updateErr)
			return fmt.Errorf("process parse task: %v; update failed status: %w", err, updateErr)
		}

		log.Printf("marked document as failed after parse task error: document_id=%s", documentID)
		return fmt.Errorf("process parse task: %w", err)
	}

	log.Printf(
		"completed parse document task: document_id=%s chunks=%d duration=%s",
		documentID,
		len(chunks),
		time.Since(startedAt),
	)

	return nil
}
