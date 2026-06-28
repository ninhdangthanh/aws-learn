package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/llm"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
)

const (
	defaultChatTopK        = 5
	maxChatTopK            = 10
	recentChatHistoryLimit = 6
)

type AnswerGenerator interface {
	Generate(ctx context.Context, input llm.GenerateInput) (llm.GenerateOutput, error)
}

type RAGChatService struct {
	searcher  SemanticSearchService
	generator AnswerGenerator
	chatStore ChatStore
}

type SemanticSearchService interface {
	Search(ctx context.Context, input SearchInput) (SearchOutput, error)
}

type ChatStore interface {
	CreateSession(ctx context.Context, input repository.CreateChatSessionInput) (model.ChatSession, error)
	GetSession(ctx context.Context, id uuid.UUID) (model.ChatSession, error)
	CreateMessage(ctx context.Context, input repository.CreateChatMessageInput) (model.ChatMessage, error)
	ListRecentMessagesBySession(ctx context.Context, sessionID uuid.UUID, limit int) ([]model.ChatMessage, error)
	TouchSession(ctx context.Context, id uuid.UUID) error
}

type ChatInput struct {
	Question       string
	SessionID      *uuid.UUID
	TopK           int
	ScoreThreshold *float64
}

type ChatOutput struct {
	Answer     string         `json:"answer"`
	Citations  []Citation     `json:"citations"`
	SessionID  uuid.UUID      `json:"session_id"`
	TokenUsage llm.TokenUsage `json:"token_usage"`
	LatencyMS  int64          `json:"latency_ms"`
}

type Citation struct {
	ChunkID     uuid.UUID `json:"chunk_id"`
	DocumentID  uuid.UUID `json:"document_id"`
	Filename    string    `json:"filename"`
	PageNumber  *int32    `json:"page_number"`
	ChunkIndex  int32     `json:"chunk_index"`
	TextSnippet string    `json:"text_snippet"`
	Score       float64   `json:"score"`
}

func NewRAGChatService(searcher SemanticSearchService, generator AnswerGenerator, chatStore ChatStore) *RAGChatService {
	return &RAGChatService{
		searcher:  searcher,
		generator: generator,
		chatStore: chatStore,
	}
}

func (s *RAGChatService) Chat(ctx context.Context, input ChatInput) (ChatOutput, error) {
	startedAt := time.Now()

	question := strings.TrimSpace(input.Question)
	if question == "" {
		return ChatOutput{}, fmt.Errorf("question is required")
	}

	session, err := s.resolveSession(ctx, question, input.SessionID)
	if err != nil {
		return ChatOutput{}, err
	}

	history, err := s.chatStore.ListRecentMessagesBySession(ctx, session.ID, recentChatHistoryLimit)
	if err != nil {
		return ChatOutput{}, fmt.Errorf("load recent chat history: %w", err)
	}

	topK := normalizeChatTopK(input.TopK)
	searchOutput, err := s.searcher.Search(ctx, SearchInput{
		Query:          question,
		TopK:           topK,
		ScoreThreshold: input.ScoreThreshold,
	})
	if err != nil {
		return ChatOutput{}, fmt.Errorf("retrieve context: %w", err)
	}

	citations := buildCitations(searchOutput.Results)
	if len(searchOutput.Results) == 0 {
		output := ChatOutput{
			Answer:     "I cannot answer from the uploaded documents because no relevant context was found.",
			Citations:  citations,
			SessionID:  session.ID,
			TokenUsage: llm.TokenUsage{},
			LatencyMS:  time.Since(startedAt).Milliseconds(),
		}
		if err := s.saveExchange(ctx, session.ID, question, output); err != nil {
			return ChatOutput{}, err
		}
		return output, nil
	}

	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt(),
		},
		{
			Role:    "user",
			Content: buildUserPrompt(question, history, searchOutput.Results),
		},
	}

	generated, err := s.generator.Generate(ctx, llm.GenerateInput{Messages: messages})
	if err != nil {
		return ChatOutput{}, fmt.Errorf("generate grounded answer: %w", err)
	}

	output := ChatOutput{
		Answer:     generated.Content,
		Citations:  citations,
		SessionID:  session.ID,
		TokenUsage: generated.Usage,
		LatencyMS:  time.Since(startedAt).Milliseconds(),
	}
	if err := s.saveExchange(ctx, session.ID, question, output); err != nil {
		return ChatOutput{}, err
	}

	log.Printf(
		"rag chat completed: session_id=%s question_len=%d top_k=%d history=%d retrieved=%d prompt_tokens=%d completion_tokens=%d latency_ms=%d",
		session.ID,
		len(question),
		topK,
		len(history),
		len(searchOutput.Results),
		output.TokenUsage.PromptTokens,
		output.TokenUsage.CompletionTokens,
		output.LatencyMS,
	)

	return output, nil
}

func (s *RAGChatService) resolveSession(ctx context.Context, question string, sessionID *uuid.UUID) (model.ChatSession, error) {
	if s.chatStore == nil {
		return model.ChatSession{}, fmt.Errorf("chat store is required")
	}

	if sessionID != nil {
		session, err := s.chatStore.GetSession(ctx, *sessionID)
		if err != nil {
			return model.ChatSession{}, fmt.Errorf("get chat session: %w", err)
		}
		return session, nil
	}

	title := chatTitle(question)
	session, err := s.chatStore.CreateSession(ctx, repository.CreateChatSessionInput{
		Title: &title,
	})
	if err != nil {
		return model.ChatSession{}, fmt.Errorf("create chat session: %w", err)
	}
	return session, nil
}

func (s *RAGChatService) saveExchange(ctx context.Context, sessionID uuid.UUID, question string, output ChatOutput) error {
	if _, err := s.chatStore.CreateMessage(ctx, repository.CreateChatMessageInput{
		SessionID: sessionID,
		Role:      "user",
		Content:   question,
	}); err != nil {
		return fmt.Errorf("save user chat message: %w", err)
	}

	citationsJSON, err := json.Marshal(output.Citations)
	if err != nil {
		return fmt.Errorf("marshal chat citations: %w", err)
	}
	citationsRaw := json.RawMessage(citationsJSON)

	tokenUsageJSON, err := json.Marshal(output.TokenUsage)
	if err != nil {
		return fmt.Errorf("marshal token usage: %w", err)
	}
	tokenUsageRaw := json.RawMessage(tokenUsageJSON)
	latency := int32(output.LatencyMS)

	if _, err := s.chatStore.CreateMessage(ctx, repository.CreateChatMessageInput{
		SessionID:  sessionID,
		Role:       "assistant",
		Content:    output.Answer,
		Citations:  &citationsRaw,
		TokenUsage: &tokenUsageRaw,
		LatencyMs:  &latency,
	}); err != nil {
		return fmt.Errorf("save assistant chat message: %w", err)
	}

	if err := s.chatStore.TouchSession(ctx, sessionID); err != nil {
		return fmt.Errorf("touch chat session: %w", err)
	}

	return nil
}

func normalizeChatTopK(value int) int {
	if value <= 0 {
		return defaultChatTopK
	}
	if value > maxChatTopK {
		return maxChatTopK
	}
	return value
}

func systemPrompt() string {
	return strings.TrimSpace(`
You are a RAG assistant for uploaded internal documents.
Answer using only the provided context.
If the context is insufficient, say: "I cannot answer from the uploaded documents."
Do not use outside knowledge.
Write in the same language as the user's question.
When facts come from context, cite sources inline using [filename, page N].
Keep the answer concise and practical.
`)
}

func buildUserPrompt(question string, history []model.ChatMessage, results []SearchMatch) string {
	var builder strings.Builder
	builder.WriteString("Question:\n")
	builder.WriteString(question)
	builder.WriteString("\n\nRecent conversation:\n")
	if len(history) == 0 {
		builder.WriteString("No prior conversation.\n")
	} else {
		for _, message := range history {
			builder.WriteString(fmt.Sprintf("%s: %s\n", message.Role, strings.TrimSpace(message.Content)))
		}
	}
	builder.WriteString("\nContext:\n")

	for i, result := range results {
		page := "unknown"
		if result.PageNumber != nil {
			page = fmt.Sprintf("%d", *result.PageNumber)
		}
		builder.WriteString(fmt.Sprintf(
			"\n[%d] filename=%s page=%s chunk_index=%d score=%.4f\n%s\n",
			i+1,
			result.Filename,
			page,
			result.ChunkIndex,
			result.Score,
			strings.TrimSpace(result.Text),
		))
	}

	builder.WriteString("\nInstructions:\n")
	builder.WriteString("- Answer only from the context above.\n")
	builder.WriteString("- Include inline citations like [filename, page N].\n")
	builder.WriteString("- If the answer is not supported by the context, say you cannot answer from the uploaded documents.\n")
	return builder.String()
}

func chatTitle(question string) string {
	title := strings.Join(strings.Fields(question), " ")
	runes := []rune(title)
	if len(runes) <= 80 {
		return title
	}
	return string(runes[:80]) + "..."
}

func buildCitations(results []SearchMatch) []Citation {
	citations := make([]Citation, 0, len(results))
	for _, result := range results {
		citations = append(citations, Citation{
			ChunkID:     result.ChunkID,
			DocumentID:  result.DocumentID,
			Filename:    result.Filename,
			PageNumber:  result.PageNumber,
			ChunkIndex:  result.ChunkIndex,
			TextSnippet: snippet(result.Text, 260),
			Score:       result.Score,
		})
	}
	return citations
}

func snippet(text string, maxRunes int) string {
	clean := strings.Join(strings.Fields(text), " ")
	runes := []rune(clean)
	if len(runes) <= maxRunes {
		return clean
	}
	return string(runes[:maxRunes]) + "..."
}
