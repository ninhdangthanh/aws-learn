package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/tasks"
)

type fakeParseService struct {
	processFn func(ctx context.Context, documentID uuid.UUID, filePath string) ([]model.Chunk, error)
}

func (f *fakeParseService) Process(ctx context.Context, documentID uuid.UUID, filePath string) ([]model.Chunk, error) {
	return f.processFn(ctx, documentID, filePath)
}

type fakeStatusRepo struct {
	document model.Document
	updates []repository.UpdateDocumentStatusInput
}

func (f *fakeStatusRepo) Get(ctx context.Context, id uuid.UUID) (model.Document, error) {
	if f.document.ID == uuid.Nil {
		f.document = model.Document{ID: id, Status: "pending"}
	}
	return f.document, nil
}

func (f *fakeStatusRepo) UpdateStatus(ctx context.Context, input repository.UpdateDocumentStatusInput) (model.Document, error) {
	f.updates = append(f.updates, input)
	return model.Document{ID: input.ID, Status: input.Status, ChunkCount: input.ChunkCount}, nil
}

func TestProcessDocumentParseTaskSuccess(t *testing.T) {
	documentID := uuid.New()
	statusRepo := &fakeStatusRepo{}

	processor := NewTaskProcessor(&fakeParseService{
		processFn: func(ctx context.Context, gotID uuid.UUID, filePath string) ([]model.Chunk, error) {
			if gotID != documentID {
				t.Fatalf("expected documentID %s, got %s", documentID, gotID)
			}
			if filePath != "/tmp/demo.pdf" {
				t.Fatalf("unexpected file path %q", filePath)
			}
			return []model.Chunk{{ID: uuid.New()}}, nil
		},
	}, statusRepo)

	task, err := tasks.NewDocumentParseTask(documentID, "/tmp/demo.pdf")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}

	if err := processor.ProcessDocumentParseTask(context.Background(), task); err != nil {
		t.Fatalf("process task: %v", err)
	}

	if len(statusRepo.updates) != 1 || statusRepo.updates[0].Status != "parsing" {
		t.Fatalf("expected one parsing update, got %+v", statusRepo.updates)
	}
}

func TestProcessDocumentParseTaskFailureMarksDocumentFailed(t *testing.T) {
	documentID := uuid.New()
	statusRepo := &fakeStatusRepo{}

	processor := NewTaskProcessor(&fakeParseService{
		processFn: func(ctx context.Context, gotID uuid.UUID, filePath string) ([]model.Chunk, error) {
			return nil, errors.New("parse failed")
		},
	}, statusRepo)

	task, err := tasks.NewDocumentParseTask(documentID, "/tmp/demo.pdf")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}

	if err := processor.ProcessDocumentParseTask(context.Background(), task); err == nil {
		t.Fatalf("expected task processing error")
	}

	if len(statusRepo.updates) != 2 {
		t.Fatalf("expected parsing + failed updates, got %+v", statusRepo.updates)
	}

	if statusRepo.updates[0].Status != "parsing" || statusRepo.updates[1].Status != "failed" {
		t.Fatalf("unexpected status transitions: %+v", statusRepo.updates)
	}
}

func TestProcessDocumentParseTaskInvalidPayload(t *testing.T) {
	processor := NewTaskProcessor(&fakeParseService{
		processFn: func(ctx context.Context, gotID uuid.UUID, filePath string) ([]model.Chunk, error) {
			return nil, nil
		},
	}, &fakeStatusRepo{})

	task := asynq.NewTask(tasks.TypeDocumentParse, []byte("not-json"))
	if err := processor.ProcessDocumentParseTask(context.Background(), task); err == nil {
		t.Fatalf("expected invalid payload error")
	}
}

func TestProcessDocumentParseTaskSkipsAlreadyChunkedDocument(t *testing.T) {
	documentID := uuid.New()
	statusRepo := &fakeStatusRepo{
		document: model.Document{
			ID:     documentID,
			Status: "chunked",
		},
	}

	called := false
	processor := NewTaskProcessor(&fakeParseService{
		processFn: func(ctx context.Context, gotID uuid.UUID, filePath string) ([]model.Chunk, error) {
			called = true
			return nil, nil
		},
	}, statusRepo)

	task, err := tasks.NewDocumentParseTask(documentID, "/tmp/demo.pdf")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}

	if err := processor.ProcessDocumentParseTask(context.Background(), task); err != nil {
		t.Fatalf("process task: %v", err)
	}

	if called {
		t.Fatalf("expected parse service to be skipped for already chunked document")
	}

	if len(statusRepo.updates) != 0 {
		t.Fatalf("expected no status updates, got %+v", statusRepo.updates)
	}
}
