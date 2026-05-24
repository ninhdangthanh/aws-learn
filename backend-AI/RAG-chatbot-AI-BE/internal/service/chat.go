package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ninhdangthanh/rag-chatbot/internal/config"
	"github.com/ninhdangthanh/rag-chatbot/internal/model"
	"github.com/ninhdangthanh/rag-chatbot/internal/prompt"
	"github.com/ninhdangthanh/rag-chatbot/internal/repository"
)

type ChatService struct {
	db        *repository.Postgres
	ingestion *IngestionService
	retrieval *RetrievalService
	embedding EmbeddingProvider
	llm       LLMProvider
	cfg       *config.Config
}

func NewChatService(db *repository.Postgres, ingestion *IngestionService, retrieval *RetrievalService, embedding EmbeddingProvider, llm LLMProvider, cfg *config.Config) *ChatService {
	return &ChatService{db: db, ingestion: ingestion, retrieval: retrieval, embedding: embedding, llm: llm, cfg: cfg}
}

func (s *ChatService) Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error) {
	if req.TopK <= 0 {
		req.TopK = s.cfg.SearchTopK
	}

	chunks, err := s.retrieval.Search(ctx, req.Question, req.TopK, 0)
	if err != nil {
		return nil, err
	}

	citations := make([]model.Citation, 0, len(chunks))
	contextParts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		contextParts = append(contextParts, fmt.Sprintf("[%d] %s", chunk.ChunkIndex+1, chunk.Text))
		citations = append(citations, model.Citation{Filename: chunk.Filename, PageNumber: chunk.PageNumber, Snippet: chunk.Text})
	}

	promptText := prompt.BuildPrompt(strings.Join(contextParts, "\n\n"), req.Question)
	start := time.Now()
	answer, err := s.llm.GenerateAnswer(ctx, promptText)
	if err != nil {
		return nil, err
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID, err = s.db.CreateChatSession(ctx, req.Question)
		if err != nil {
			return nil, err
		}
	}

	msg := model.ChatMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "assistant",
		Content:   answer,
		Citations: citations,
		LatencyMs: int(time.Since(start).Milliseconds()),
	}
	if err := s.db.CreateChatMessage(ctx, msg); err != nil {
		return nil, err
	}

	return &model.ChatResponse{Answer: answer, Citations: citations, SessionID: sessionID}, nil
}
