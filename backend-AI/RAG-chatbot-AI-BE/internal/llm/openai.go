package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

type Message struct {
	Role    string
	Content string
}

type GenerateInput struct {
	Messages []Message
}

type GenerateOutput struct {
	Content string
	Usage   TokenUsage
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

type OpenAIConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

func NewOpenAIClient(config OpenAIConfig) *OpenAIClient {
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}

	return &OpenAIClient{
		apiKey:     config.APIKey,
		model:      config.Model,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *OpenAIClient) Generate(ctx context.Context, input GenerateInput) (GenerateOutput, error) {
	if c.apiKey == "" {
		return GenerateOutput{}, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if c.model == "" {
		return GenerateOutput{}, fmt.Errorf("chat model is required")
	}
	if len(input.Messages) == 0 {
		return GenerateOutput{}, fmt.Errorf("messages are required")
	}

	messages := make([]chatCompletionMessage, 0, len(input.Messages))
	for i, message := range input.Messages {
		if strings.TrimSpace(message.Role) == "" {
			return GenerateOutput{}, fmt.Errorf("message role at index %d is required", i)
		}
		if strings.TrimSpace(message.Content) == "" {
			return GenerateOutput{}, fmt.Errorf("message content at index %d is required", i)
		}
		messages = append(messages, chatCompletionMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}

	requestBody := createChatCompletionRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.2,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("marshal chat completion request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("create chat completion request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("call chat completions API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var apiErr openAIErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
			return GenerateOutput{}, fmt.Errorf("chat completions API returned %s: %s", resp.Status, apiErr.Error.Message)
		}
		return GenerateOutput{}, fmt.Errorf("chat completions API returned %s", resp.Status)
	}

	var result createChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return GenerateOutput{}, fmt.Errorf("decode chat completion response: %w", err)
	}
	if len(result.Choices) == 0 {
		return GenerateOutput{}, fmt.Errorf("chat completion response has no choices")
	}

	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return GenerateOutput{}, fmt.Errorf("chat completion response is empty")
	}

	return GenerateOutput{
		Content: content,
		Usage: TokenUsage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}, nil
}

type createChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []chatCompletionMessage `json:"messages"`
	Temperature float64                 `json:"temperature,omitempty"`
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type createChatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
	Usage   chatCompletionUsage    `json:"usage"`
}

type chatCompletionChoice struct {
	Message chatCompletionMessage `json:"message"`
}

type chatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}
