package service

import (
	"context"
	"fmt"

	"github.com/ninhdangthanh/rag-chatbot/internal/config"
	"github.com/ninhdangthanh/rag-chatbot/internal/model"
	"github.com/ninhdangthanh/rag-chatbot/internal/repository"
)

type RetrievalService struct {
	qdrant *repository.Qdrant
	cfg    *config.Config
}

func NewRetrievalService(qdrant *repository.Qdrant, cfg *config.Config) *RetrievalService {
	return &RetrievalService{qdrant: qdrant, cfg: cfg}
}

func (s *RetrievalService) Search(ctx context.Context, query string, topK int, scoreThreshold float64) ([]model.SearchResult, error) {
	if topK <= 0 {
		topK = s.cfg.SearchTopK
	}
	if scoreThreshold <= 0 {
		scoreThreshold = 0.0
	}

	embeddings, err := NewEmbeddingService(s.cfg).EmbedText(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	points, err := s.qdrant.Search(ctx, embeddings[0], topK, scoreThreshold)
	if err != nil {
		return nil, err
	}

	results := []model.SearchResult{}
	for _, item := range points {
		payload := item.Payload
		documentID := fmt.Sprintf("%v", payload["document_id"])
		filename := fmt.Sprintf("%v", payload["filename"])
		pageNumber := 0
		if v, ok := payload["page_number"]; ok {
			switch n := v.(type) {
			case float64:
				pageNumber = int(n)
			case int:
				pageNumber = n
			}
		}
		chunkIndex := 0
		if v, ok := payload["chunk_index"]; ok {
			switch n := v.(type) {
			case float64:
				chunkIndex = int(n)
			case int:
				chunkIndex = n
			}
		}
		result := model.SearchResult{
			DocumentID: documentID,
			Filename:   filename,
			PageNumber: pageNumber,
			ChunkIndex: chunkIndex,
			Score:      float64(item.Score),
			Text:       fmt.Sprintf("%v", payload["text"]),
		}
		results = append(results, result)
	}
	return results, nil
}
