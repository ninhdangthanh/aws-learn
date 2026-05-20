package service

import (
	"context"
	"fmt"

	"github.com/ninhdangthanh/rag-chatbot/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

type EmbeddingService struct {
	client *openai.Client
	cfg    *config.Config
}

func NewEmbeddingService(cfg *config.Config) *EmbeddingService {
	return &EmbeddingService{client: openai.NewClient(cfg.OpenAIAPIKey), cfg: cfg}
}

func (s *EmbeddingService) EmbedText(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var model openai.EmbeddingModel
	if err := model.UnmarshalText([]byte(s.cfg.OpenAIEmbeddingModel)); err != nil {
		return nil, err
	}
	req := openai.EmbeddingRequest{
		Model: model,
		Input: texts,
	}
	resp, err := s.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, err
	}
	vectors := make([][]float32, 0, len(resp.Data))
	for _, item := range resp.Data {
		vec := make([]float32, len(item.Embedding))
		for i, f := range item.Embedding {
			vec[i] = float32(f)
		}
		vectors = append(vectors, vec)
	}
	return vectors, nil
}

func (s *EmbeddingService) GenerateAnswer(ctx context.Context, prompt string) (string, error) {
	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:     s.cfg.OpenAILLMModel,
		Messages:  []openai.ChatCompletionMessage{{Role: "system", Content: "You are a helpful assistant that answers questions based ONLY on the provided context. If you cannot answer, say you don't have enough information."}, {Role: "user", Content: prompt}},
		MaxTokens: 512,
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion returned")
	}
	return resp.Choices[0].Message.Content, nil
}
