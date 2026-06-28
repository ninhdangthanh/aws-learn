package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/llm"
)

const (
	defaultChatTopK = 5
	maxChatTopK     = 10
)

type AnswerGenerator interface {
	Generate(ctx context.Context, input llm.GenerateInput) (llm.GenerateOutput, error)
}

type RAGChatService struct {
	searcher  SemanticSearchService
	generator AnswerGenerator
}

type SemanticSearchService interface {
	Search(ctx context.Context, input SearchInput) (SearchOutput, error)
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
	SessionID  *uuid.UUID     `json:"session_id,omitempty"`
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

func NewRAGChatService(searcher SemanticSearchService, generator AnswerGenerator) *RAGChatService {
	return &RAGChatService{
		searcher:  searcher,
		generator: generator,
	}
}

func (s *RAGChatService) Chat(ctx context.Context, input ChatInput) (ChatOutput, error) {
	startedAt := time.Now()

	question := strings.TrimSpace(input.Question)
	if question == "" {
		return ChatOutput{}, fmt.Errorf("question is required")
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
		return ChatOutput{
			Answer:     "I cannot answer from the uploaded documents because no relevant context was found.",
			Citations:  citations,
			SessionID:  input.SessionID,
			TokenUsage: llm.TokenUsage{},
			LatencyMS:  time.Since(startedAt).Milliseconds(),
		}, nil
	}

	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt(),
		},
		{
			Role:    "user",
			Content: buildUserPrompt(question, searchOutput.Results),
		},
	}

	generated, err := s.generator.Generate(ctx, llm.GenerateInput{Messages: messages})
	if err != nil {
		return ChatOutput{}, fmt.Errorf("generate grounded answer: %w", err)
	}

	output := ChatOutput{
		Answer:     generated.Content,
		Citations:  citations,
		SessionID:  input.SessionID,
		TokenUsage: generated.Usage,
		LatencyMS:  time.Since(startedAt).Milliseconds(),
	}

	log.Printf(
		"rag chat completed: question_len=%d top_k=%d retrieved=%d prompt_tokens=%d completion_tokens=%d latency_ms=%d",
		len(question),
		topK,
		len(searchOutput.Results),
		output.TokenUsage.PromptTokens,
		output.TokenUsage.CompletionTokens,
		output.LatencyMS,
	)

	return output, nil
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

func buildUserPrompt(question string, results []SearchMatch) string {
	var builder strings.Builder
	builder.WriteString("Question:\n")
	builder.WriteString(question)
	builder.WriteString("\n\nContext:\n")

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
