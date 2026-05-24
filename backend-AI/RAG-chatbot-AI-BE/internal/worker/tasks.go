package worker

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/ninhdangthanh/rag-chatbot/internal/config"
	"github.com/ninhdangthanh/rag-chatbot/internal/repository"
	"github.com/ninhdangthanh/rag-chatbot/internal/service"
)

const (
	TaskTypeParseDocument = "parse:document"
	TaskTypeEmbedChunks   = "embed:chunks"
)

type TaskHandler struct {
	db        *repository.Postgres
	qdrant    *repository.Qdrant
	embedding service.EmbeddingProvider
	ingestion *service.IngestionService
	cfg       *config.Config
}

func NewTaskHandler(db *repository.Postgres, qdrant *repository.Qdrant, embedding service.EmbeddingProvider, ingestion *service.IngestionService, cfg *config.Config) *TaskHandler {
	return &TaskHandler{db: db, qdrant: qdrant, embedding: embedding, ingestion: ingestion, cfg: cfg}
}

func (h *TaskHandler) HandleParseDocumentTask(ctx context.Context, t *asynq.Task) error {
	var payload struct {
		DocumentID string `json:"document_id"`
		FilePath   string `json:"file_path"`
	}
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}
	if err := h.ingestion.ParseDocument(ctx, payload.DocumentID, payload.FilePath); err != nil {
		return err
	}
	return nil
}

func (h *TaskHandler) HandleEmbedChunksTask(ctx context.Context, t *asynq.Task) error {
	var payload struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	chunks, err := h.db.FindChunksToEmbed(ctx, payload.DocumentID)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return nil
	}

	batchSize := 16
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]
		texts := make([]string, len(batch))
		for j, chunk := range batch {
			texts[j] = chunk.Content
		}
		vectors, err := h.embedding.EmbedText(ctx, texts)
		if err != nil {
			return err
		}

		points := make([]repository.Point, 0, len(batch))
		for j, chunk := range batch {
			pointPayload := map[string]interface{}{
				"document_id": chunk.DocumentID,
				"chunk_index": chunk.ChunkIndex,
				"page_number": chunk.PageNumber,
				"text":        chunk.Content,
			}
			points = append(points, repository.Point{ID: chunk.ID, Vector: vectors[j], Payload: pointPayload})
		}
		if err := h.qdrant.UpsertPoints(ctx, points); err != nil {
			return err
		}
		for _, chunk := range batch {
			if err := h.db.UpdateChunkQdrantID(ctx, chunk.ID, chunk.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
