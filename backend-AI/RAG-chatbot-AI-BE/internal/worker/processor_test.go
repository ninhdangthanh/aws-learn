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
	updates  []repository.UpdateDocumentStatusInput
}

func (f *fakeStatusRepo) Get(ctx context.Context, id uuid.UUID) (model.Document, error) {
	if f.document.ID == uuid.Nil {
		f.document = model.Document{ID: id, Status: model.DocumentStatusPending}
	}
	return f.document, nil
}

func (f *fakeStatusRepo) UpdateStatus(ctx context.Context, input repository.UpdateDocumentStatusInput) (model.Document, error) {
	f.updates = append(f.updates, input)
	f.document.ID = input.ID
	f.document.Status = input.Status
	f.document.ChunkCount = input.ChunkCount
	f.document.ErrorMsg = input.ErrorMsg
	return f.document, nil
}

type fakeEmbedDistributor struct {
	documentIDs []uuid.UUID
	err         error
}

func (f *fakeEmbedDistributor) EnqueueEmbedDocument(ctx context.Context, documentID uuid.UUID) error {
	f.documentIDs = append(f.documentIDs, documentID)
	return f.err
}

type fakeChunkRepository struct {
	chunks             []model.Chunk
	qdrantIDsByChunkID map[uuid.UUID]uuid.UUID
}

func (f *fakeChunkRepository) GetByDocument(ctx context.Context, documentID uuid.UUID) ([]model.Chunk, error) {
	return f.chunks, nil
}

func (f *fakeChunkRepository) UpdateQdrantIDs(ctx context.Context, qdrantIDsByChunkID map[uuid.UUID]uuid.UUID) error {
	f.qdrantIDsByChunkID = qdrantIDsByChunkID
	return nil
}

type fakeEmbedder struct {
	vectors [][]float32
	texts   []string
	err     error
}

func (f *fakeEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	f.texts = texts
	return f.vectors, f.err
}

type fakeVectorStore struct {
	deletedFor uuid.UUID
	document   model.Document
	chunks     []model.Chunk
	vectors    [][]float32
	pointIDs   []uuid.UUID
	err        error
}

func (f *fakeVectorStore) DeleteByDocument(ctx context.Context, documentID uuid.UUID) error {
	f.deletedFor = documentID
	return nil
}

func (f *fakeVectorStore) UpsertChunks(ctx context.Context, document model.Document, chunks []model.Chunk, vectors [][]float32) ([]uuid.UUID, error) {
	f.document = document
	f.chunks = chunks
	f.vectors = vectors
	return f.pointIDs, f.err
}

func TestProcessDocumentParseTaskSuccess(t *testing.T) {
	documentID := uuid.New()
	statusRepo := &fakeStatusRepo{}
	embedDistributor := &fakeEmbedDistributor{}

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
	}, statusRepo, WithTaskDistributor(embedDistributor))

	task, err := tasks.NewDocumentParseTask(documentID, "/tmp/demo.pdf")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}

	if err := processor.ProcessDocumentParseTask(context.Background(), task); err != nil {
		t.Fatalf("process task: %v", err)
	}

	if len(statusRepo.updates) != 1 || statusRepo.updates[0].Status != model.DocumentStatusParsing {
		t.Fatalf("expected one parsing update, got %+v", statusRepo.updates)
	}

	if len(embedDistributor.documentIDs) != 1 || embedDistributor.documentIDs[0] != documentID {
		t.Fatalf("expected embed task to be enqueued for %s, got %+v", documentID, embedDistributor.documentIDs)
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

	if statusRepo.updates[0].Status != model.DocumentStatusParsing || statusRepo.updates[1].Status != model.DocumentStatusFailed {
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
			Status: model.DocumentStatusChunked,
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

func TestProcessDocumentEmbedTaskSuccess(t *testing.T) {
	documentID := uuid.New()
	chunkID := uuid.New()
	pointID := uuid.New()
	statusRepo := &fakeStatusRepo{
		document: model.Document{
			ID:         documentID,
			Filename:   "demo.pdf",
			Status:     model.DocumentStatusChunked,
			ChunkCount: 1,
		},
	}
	chunkRepo := &fakeChunkRepository{
		chunks: []model.Chunk{
			{
				ID:         chunkID,
				DocumentID: documentID,
				ChunkIndex: 0,
				Content:    "hello vector world",
				TokenCount: 3,
			},
		},
	}
	embedder := &fakeEmbedder{
		vectors: [][]float32{{0.1, 0.2, 0.3}},
	}
	vectorStore := &fakeVectorStore{
		pointIDs: []uuid.UUID{pointID},
	}

	processor := NewEmbeddingTaskProcessor(statusRepo, chunkRepo, embedder, vectorStore)
	task, err := tasks.NewDocumentEmbedTask(documentID)
	if err != nil {
		t.Fatalf("new task: %v", err)
	}

	if err := processor.ProcessDocumentEmbedTask(context.Background(), task); err != nil {
		t.Fatalf("process embed task: %v", err)
	}

	if len(statusRepo.updates) != 2 {
		t.Fatalf("expected embedding + ready updates, got %+v", statusRepo.updates)
	}
	if statusRepo.updates[0].Status != model.DocumentStatusEmbedding || statusRepo.updates[1].Status != model.DocumentStatusReady {
		t.Fatalf("unexpected status transitions: %+v", statusRepo.updates)
	}
	if vectorStore.deletedFor != documentID {
		t.Fatalf("expected old vectors to be deleted for %s, got %s", documentID, vectorStore.deletedFor)
	}
	if got := chunkRepo.qdrantIDsByChunkID[chunkID]; got != pointID {
		t.Fatalf("expected qdrant id %s for chunk %s, got %s", pointID, chunkID, got)
	}
}
