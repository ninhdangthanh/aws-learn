package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ninhdangthanh/rag-chatbot/internal/config"
)

// ClaudeProvider implements LLMProvider interface for Anthropic Claude.
type ClaudeProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewClaudeProvider creates a new instance of ClaudeProvider.
func NewClaudeProvider(cfg *config.Config) *ClaudeProvider {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	return &ClaudeProvider{
		apiKey: apiKey,
		model:  "claude-3-5-sonnet-20241022",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ClaudeMessage represents the message structure for Claude API.
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeRequest represents the request payload for Claude API.
type ClaudeRequest struct {
	Model     string          `json:"model"`
	Messages  []ClaudeMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"` // Claude tách riêng system prompt ở root level
}

// ClaudeResponse represents the response payload from Claude API.
type ClaudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

// GenerateAnswer calls Anthropic Claude API to generate a response based on the context.
func (p *ClaudeProvider) GenerateAnswer(ctx context.Context, prompt string) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("CLAUDE_API_KEY is not set")
	}

	// Lắp ráp payload theo đúng định dạng API của Anthropic Claude
	reqPayload := ClaudeRequest{
		Model: p.model,
		Messages: []ClaudeMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 1024,
		System:    "You are a helpful assistant that answers questions based ONLY on the provided context.",
	}

	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	// Set header bắt buộc đối với Anthropic API
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01") // Phiên bản API cố định của Anthropic

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude API returned non-OK status: %d", resp.StatusCode)
	}

	var respPayload ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return "", err
	}

	if len(respPayload.Content) == 0 {
		return "", fmt.Errorf("empty response content from Claude")
	}

	return respPayload.Content[0].Text, nil
}
