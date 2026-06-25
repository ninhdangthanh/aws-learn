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

type chunkRepository interface {
	GetByDocument(ctx context.Context, documentID uuid.UUID) ([]model.Chunk, error)
	UpdateQdrantIDs(ctx context.Context, qdrantIDsByChunkID map[uuid.UUID]uuid.UUID) error
}

type parseChunkProcessor interface {
	Process(ctx context.Context, documentID uuid.UUID, filePath string) ([]model.Chunk, error)
}

type embedder interface {
	EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
}

type vectorStore interface {
	DeleteByDocument(ctx context.Context, documentID uuid.UUID) error
	UpsertChunks(ctx context.Context, document model.Document, chunks []model.Chunk, vectors [][]float32) ([]uuid.UUID, error)
}

type embedTaskDistributor interface {
	EnqueueEmbedDocument(ctx context.Context, documentID uuid.UUID) error
}

type TaskProcessor struct {
	parseService    parseChunkProcessor
	documentRepo    documentStatusRepository
	chunkRepo       chunkRepository
	embedder        embedder
	vectorStore     vectorStore
	taskDistributor embedTaskDistributor
}

type TaskProcessorOption func(*TaskProcessor)

func NewTaskProcessor(parseService parseChunkProcessor, documentRepo documentStatusRepository, options ...TaskProcessorOption) *TaskProcessor {
	processor := &TaskProcessor{
		parseService: parseService,
		documentRepo: documentRepo,
	}
	for _, option := range options {
		option(processor)
	}
	return processor
}

func WithTaskDistributor(taskDistributor embedTaskDistributor) TaskProcessorOption {
	return func(processor *TaskProcessor) {
		processor.taskDistributor = taskDistributor
	}
}

func NewEmbeddingTaskProcessor(documentRepo documentStatusRepository, chunkRepo chunkRepository, embedder embedder, vectorStore vectorStore) *TaskProcessor {
	return &TaskProcessor{
		documentRepo: documentRepo,
		chunkRepo:    chunkRepo,
		embedder:     embedder,
		vectorStore:  vectorStore,
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

	if document.Status == model.DocumentStatusReady {
		log.Printf(
			"skipped parse document task because document is already ready: document_id=%s duration=%s",
			documentID,
			time.Since(startedAt),
		)
		return nil
	}

	if document.Status == model.DocumentStatusChunked {
		log.Printf("document is already chunked; enqueueing embed task: document_id=%s", documentID)
		if p.taskDistributor != nil {
			if err := p.taskDistributor.EnqueueEmbedDocument(ctx, documentID); err != nil {
				return fmt.Errorf("enqueue embed task for chunked document: %w", err)
			}
		}
		return nil
	}

	log.Printf("marking document as parsing: document_id=%s previous_status=%s", documentID, document.Status)
	if _, err := p.documentRepo.UpdateStatus(ctx, repository.UpdateDocumentStatusInput{
		ID:     documentID,
		Status: model.DocumentStatusParsing,
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
			Status:   model.DocumentStatusFailed,
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

	if p.taskDistributor != nil {
		if err := p.taskDistributor.EnqueueEmbedDocument(ctx, documentID); err != nil {
			log.Printf("failed to enqueue embed document task after parse: document_id=%s err=%v", documentID, err)
			return fmt.Errorf("enqueue embed document task: %w", err)
		}
	}

	return nil
}

func (p *TaskProcessor) ProcessDocumentEmbedTask(ctx context.Context, task *asynq.Task) error {
	startedAt := time.Now()

	if p.chunkRepo == nil {
		return fmt.Errorf("chunk repository is required")
	}
	if p.embedder == nil {
		return fmt.Errorf("embedder is required")
	}
	if p.vectorStore == nil {
		return fmt.Errorf("vector store is required")
	}

	var payload tasks.DocumentEmbedPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		log.Printf("failed to decode embed document task payload: type=%s err=%v", task.Type(), err)
		return fmt.Errorf("unmarshal embed task payload: %w", err)
	}

	documentID, err := tasks.ParseDocumentID(payload.DocumentID)
	if err != nil {
		log.Printf("invalid embed document task payload: type=%s document_id=%s err=%v", task.Type(), payload.DocumentID, err)
		return fmt.Errorf("parse document id: %w", err)
	}

	log.Printf("started embed document task: type=%s document_id=%s", task.Type(), documentID)

	document, err := p.documentRepo.Get(ctx, documentID)
	if err != nil {
		log.Printf("failed to load document for embed task: document_id=%s err=%v", documentID, err)
		return fmt.Errorf("get document for embed task: %w", err)
	}

	if document.Status == model.DocumentStatusReady {
		log.Printf(
			"skipped embed document task because document is already ready: document_id=%s duration=%s",
			documentID,
			time.Since(startedAt),
		)
		return nil
	}

	log.Printf("marking document as embedding: document_id=%s previous_status=%s", documentID, document.Status)
	if _, err := p.documentRepo.UpdateStatus(ctx, repository.UpdateDocumentStatusInput{
		ID:         documentID,
		Status:     model.DocumentStatusEmbedding,
		ChunkCount: document.ChunkCount,
	}); err != nil {
		log.Printf("failed to mark document as embedding: document_id=%s err=%v", documentID, err)
		return fmt.Errorf("mark document embedding: %w", err)
	}

	chunks, err := p.chunkRepo.GetByDocument(ctx, documentID)
	if err != nil {
		return p.failEmbedTask(ctx, documentID, fmt.Errorf("get document chunks: %w", err))
	}

	if len(chunks) == 0 {
		log.Printf("document has no chunks to embed: document_id=%s", documentID)
		if _, err := p.documentRepo.UpdateStatus(ctx, repository.UpdateDocumentStatusInput{
			ID:         documentID,
			Status:     model.DocumentStatusReady,
			ChunkCount: 0,
		}); err != nil {
			return fmt.Errorf("mark empty document ready: %w", err)
		}
		return nil
	}

	texts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		texts = append(texts, chunk.Content)
	}

	log.Printf("embedding document chunks: document_id=%s chunks=%d", documentID, len(chunks))
	vectors, err := p.embedder.EmbedTexts(ctx, texts)
	if err != nil {
		return p.failEmbedTask(ctx, documentID, fmt.Errorf("embed document chunks: %w", err))
	}

	log.Printf("replacing qdrant points for document: document_id=%s", documentID)
	if err := p.vectorStore.DeleteByDocument(ctx, documentID); err != nil {
		return p.failEmbedTask(ctx, documentID, fmt.Errorf("delete existing vectors: %w", err))
	}

	pointIDs, err := p.vectorStore.UpsertChunks(ctx, document, chunks, vectors)
	if err != nil {
		return p.failEmbedTask(ctx, documentID, fmt.Errorf("upsert vectors: %w", err))
	}

	qdrantIDsByChunkID := make(map[uuid.UUID]uuid.UUID, len(chunks))
	for i, chunk := range chunks {
		qdrantIDsByChunkID[chunk.ID] = pointIDs[i]
	}
	if err := p.chunkRepo.UpdateQdrantIDs(ctx, qdrantIDsByChunkID); err != nil {
		return p.failEmbedTask(ctx, documentID, fmt.Errorf("update chunk qdrant ids: %w", err))
	}

	if _, err := p.documentRepo.UpdateStatus(ctx, repository.UpdateDocumentStatusInput{
		ID:         documentID,
		Status:     model.DocumentStatusReady,
		ChunkCount: int32(len(chunks)),
	}); err != nil {
		return fmt.Errorf("mark document ready: %w", err)
	}

	log.Printf(
		"completed embed document task: document_id=%s chunks=%d duration=%s",
		documentID,
		len(chunks),
		time.Since(startedAt),
	)

	return nil
}

func (p *TaskProcessor) failEmbedTask(ctx context.Context, documentID uuid.UUID, err error) error {
	log.Printf("failed to process embed document task: document_id=%s err=%v", documentID, err)

	document, getErr := p.documentRepo.Get(ctx, documentID)
	if getErr != nil {
		return fmt.Errorf("process embed task: %v; get document for failed status: %w", err, getErr)
	}

	message := err.Error()
	if _, updateErr := p.documentRepo.UpdateStatus(ctx, repository.UpdateDocumentStatusInput{
		ID:         documentID,
		Status:     model.DocumentStatusFailed,
		ChunkCount: document.ChunkCount,
		ErrorMsg:   &message,
	}); updateErr != nil {
		return fmt.Errorf("process embed task: %v; update failed status: %w", err, updateErr)
	}

	return fmt.Errorf("process embed task: %w", err)
}
