package service

import "context"

// EmbeddingProvider định nghĩa cổng kết nối để chuyển đổi văn bản thành Vector (Embedding).
type EmbeddingProvider interface {
	EmbedText(ctx context.Context, texts []string) ([][]float32, error)
}

// LLMProvider định nghĩa cổng kết nối cho việc tương tác và nhận phản hồi từ LLM (Chat).
type LLMProvider interface {
	GenerateAnswer(ctx context.Context, prompt string) (string, error)
}
