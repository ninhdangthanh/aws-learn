package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
)

const (
	defaultSearchTopK = 5
	maxSearchTopK     = 50
)

type QueryEmbedder interface {
	EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
}

type VectorSearcher interface {
	Search(ctx context.Context, vector []float32, limit int, scoreThreshold *float64) ([]repository.VectorPoint, error)
}

type SearchService struct {
	embedder QueryEmbedder
	vectors  VectorSearcher
}

type SearchInput struct {
	Query          string
	TopK           int
	ScoreThreshold *float64
}

type SearchOutput struct {
	Query     string        `json:"query"`
	Results   []SearchMatch `json:"results"`
	LatencyMS int64         `json:"latency_ms"`
}

type SearchMatch struct {
	ChunkID    uuid.UUID `json:"chunk_id"`
	DocumentID uuid.UUID `json:"document_id"`
	Filename   string    `json:"filename"`
	PageNumber *int32    `json:"page_number"`
	ChunkIndex int32     `json:"chunk_index"`
	Text       string    `json:"text"`
	Score      float64   `json:"score"`
}

func NewSearchService(embedder QueryEmbedder, vectors VectorSearcher) *SearchService {
	return &SearchService{
		embedder: embedder,
		vectors:  vectors,
	}
}

func (s *SearchService) Search(ctx context.Context, input SearchInput) (SearchOutput, error) {
	startedAt := time.Now()

	query := strings.TrimSpace(input.Query)
	if query == "" {
		return SearchOutput{}, fmt.Errorf("query is required")
	}

	topK := normalizeTopK(input.TopK)
	vectors, err := s.embedder.EmbedTexts(ctx, []string{query})
	if err != nil {
		return SearchOutput{}, fmt.Errorf("embed search query: %w", err)
	}
	if len(vectors) != 1 {
		return SearchOutput{}, fmt.Errorf("embedding response count mismatch: got %d, want 1", len(vectors))
	}

	points, err := s.vectors.Search(ctx, vectors[0], topK, input.ScoreThreshold)
	if err != nil {
		return SearchOutput{}, fmt.Errorf("search vectors: %w", err)
	}

	matches := make([]SearchMatch, 0, len(points))
	for _, point := range points {
		matches = append(matches, SearchMatch{
			ChunkID:    point.ChunkID,
			DocumentID: point.DocumentID,
			Filename:   point.Filename,
			PageNumber: point.PageNumber,
			ChunkIndex: point.ChunkIndex,
			Text:       point.Text,
			Score:      point.Score,
		})
	}

	output := SearchOutput{
		Query:     query,
		Results:   matches,
		LatencyMS: time.Since(startedAt).Milliseconds(),
	}

	log.Printf(
		"semantic search completed: query_len=%d top_k=%d results=%d latency_ms=%d",
		len(query),
		topK,
		len(matches),
		output.LatencyMS,
	)

	return output, nil
}

func normalizeTopK(value int) int {
	if value <= 0 {
		return defaultSearchTopK
	}
	if value > maxSearchTopK {
		return maxSearchTopK
	}
	return value
}
