package worker

import (
	"context"
	"encoding/json"
	"fmt"

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
	var payload tasks.DocumentParsePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal parse task payload: %w", err)
	}

	documentID, err := tasks.ParseDocumentID(payload.DocumentID)
	if err != nil {
		return fmt.Errorf("parse document id: %w", err)
	}

	document, err := p.documentRepo.Get(ctx, documentID)
	if err != nil {
		return fmt.Errorf("get document for parse task: %w", err)
	}

	if document.Status == "chunked" {
		return nil
	}

	if _, err := p.documentRepo.UpdateStatus(ctx, repository.UpdateDocumentStatusInput{
		ID:     documentID,
		Status: "parsing",
	}); err != nil {
		return fmt.Errorf("mark document parsing: %w", err)
	}

	if _, err := p.parseService.Process(ctx, documentID, payload.FilePath); err != nil {
		message := err.Error()
		if _, updateErr := p.documentRepo.UpdateStatus(ctx, repository.UpdateDocumentStatusInput{
			ID:       documentID,
			Status:   "failed",
			ErrorMsg: &message,
		}); updateErr != nil {
			return fmt.Errorf("process parse task: %v; update failed status: %w", err, updateErr)
		}

		return fmt.Errorf("process parse task: %w", err)
	}

	return nil
}
