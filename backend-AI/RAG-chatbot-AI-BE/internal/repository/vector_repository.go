package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/config"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
)

type VectorRepository struct {
	client     *qdrant.Client
	collection string
	dimensions int
}

type VectorPoint struct {
	ID         uuid.UUID
	DocumentID uuid.UUID
	ChunkID    uuid.UUID
	Score      float64
	PageNumber *int32
	ChunkIndex int32
	Text       string
}

func NewVectorRepository(cfg config.QdrantConfig, dimensions int) (*VectorRepository, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host:                   cfg.Host,
		Port:                   cfg.GRPCPort,
		APIKey:                 cfg.APIKey,
		SkipCompatibilityCheck: true,
	})
	if err != nil {
		return nil, fmt.Errorf("create qdrant client: %w", err)
	}

	return &VectorRepository{
		client:     client,
		collection: cfg.Collection,
		dimensions: dimensions,
	}, nil
}

func (r *VectorRepository) EnsureCollection(ctx context.Context) error {
	exists, err := r.client.CollectionExists(ctx, r.collection)
	if err != nil {
		return fmt.Errorf("check qdrant collection: %w", err)
	}
	if exists {
		return nil
	}

	if err := r.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: r.collection,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(r.dimensions),
			Distance: qdrant.Distance_Cosine,
		}),
	}); err != nil {
		return fmt.Errorf("create qdrant collection: %w", err)
	}

	return nil
}

func (r *VectorRepository) DeleteByDocument(ctx context.Context, documentID uuid.UUID) error {
	_, err := r.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: r.collection,
		Wait:           qdrant.PtrOf(true),
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: &qdrant.Filter{
					Must: []*qdrant.Condition{
						qdrant.NewMatchKeyword("document_id", documentID.String()),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("delete qdrant points by document: %w", err)
	}

	return nil
}

func (r *VectorRepository) UpsertChunks(ctx context.Context, document model.Document, chunks []model.Chunk, vectors [][]float32) ([]uuid.UUID, error) {
	if len(chunks) != len(vectors) {
		return nil, fmt.Errorf("chunk/vector count mismatch: chunks=%d vectors=%d", len(chunks), len(vectors))
	}

	points := make([]*qdrant.PointStruct, 0, len(chunks))
	pointIDs := make([]uuid.UUID, 0, len(chunks))
	for i, chunk := range chunks {
		pointID := chunk.ID
		payload := map[string]any{
			"document_id": document.ID.String(),
			"chunk_id":    chunk.ID.String(),
			"filename":    document.Filename,
			"chunk_index": int64(chunk.ChunkIndex),
			"text":        chunk.Content,
		}
		if chunk.PageNumber != nil {
			payload["page_number"] = int64(*chunk.PageNumber)
		}

		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(pointID.String()),
			Vectors: newDenseVectors(vectors[i]),
			Payload: qdrant.NewValueMap(payload),
		})
		pointIDs = append(pointIDs, pointID)
	}

	if len(points) == 0 {
		return pointIDs, nil
	}

	if _, err := r.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: r.collection,
		Wait:           qdrant.PtrOf(true),
		Points:         points,
	}); err != nil {
		return nil, fmt.Errorf("upsert qdrant points: %w", err)
	}

	return pointIDs, nil
}

func (r *VectorRepository) Search(ctx context.Context, vector []float32, limit int, scoreThreshold *float64) ([]VectorPoint, error) {
	if limit <= 0 {
		return []VectorPoint{}, nil
	}

	request := &qdrant.QueryPoints{
		CollectionName: r.collection,
		Query:          qdrant.NewQueryDense(vector),
		Limit:          qdrant.PtrOf(uint64(limit)),
		WithPayload: &qdrant.WithPayloadSelector{
			SelectorOptions: &qdrant.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
	}
	if scoreThreshold != nil {
		request.ScoreThreshold = qdrant.PtrOf(float32(*scoreThreshold))
	}

	points, err := r.client.Query(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("search qdrant points: %w", err)
	}

	results := make([]VectorPoint, 0, len(points))
	for _, point := range points {
		payload := point.GetPayload()
		pointID, _ := uuid.Parse(point.GetId().GetUuid())
		documentID, _ := uuid.Parse(stringPayload(payload, "document_id"))
		chunkID, _ := uuid.Parse(stringPayload(payload, "chunk_id"))
		results = append(results, VectorPoint{
			ID:         pointID,
			DocumentID: documentID,
			ChunkID:    chunkID,
			Score:      float64(point.GetScore()),
			PageNumber: int32PointerPayload(payload, "page_number"),
			ChunkIndex: int32Payload(payload, "chunk_index"),
			Text:       stringPayload(payload, "text"),
		})
	}

	return results, nil
}

func newDenseVectors(vector []float32) *qdrant.Vectors {
	return &qdrant.Vectors{
		VectorsOptions: &qdrant.Vectors_Vector{
			Vector: &qdrant.Vector{
				Data: vector,
			},
		},
	}
}

func stringPayload(payload map[string]*qdrant.Value, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	return value.GetStringValue()
}

func int32Payload(payload map[string]*qdrant.Value, key string) int32 {
	value, ok := payload[key]
	if !ok {
		return 0
	}
	return int32(value.GetIntegerValue())
}

func int32PointerPayload(payload map[string]*qdrant.Value, key string) *int32 {
	if _, ok := payload[key]; !ok {
		return nil
	}
	value := int32Payload(payload, key)
	return &value
}
