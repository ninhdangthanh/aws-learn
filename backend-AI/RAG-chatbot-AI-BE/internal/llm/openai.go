package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

type TokenHandler func(token string) error

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
	messages, err := c.validateAndConvertMessages(input.Messages)
	if err != nil {
		return GenerateOutput{}, err
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

func (c *OpenAIClient) StreamGenerate(ctx context.Context, input GenerateInput, onToken TokenHandler) (GenerateOutput, error) {
	messages, err := c.validateAndConvertMessages(input.Messages)
	if err != nil {
		return GenerateOutput{}, err
	}

	requestBody := createChatCompletionRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.2,
		Stream:      true,
		StreamOptions: &chatCompletionStreamOptions{
			IncludeUsage: true,
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("marshal streaming chat completion request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("create streaming chat completion request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return GenerateOutput{}, fmt.Errorf("call streaming chat completions API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var apiErr openAIErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
			return GenerateOutput{}, fmt.Errorf("streaming chat completions API returned %s: %s", resp.Status, apiErr.Error.Message)
		}
		return GenerateOutput{}, fmt.Errorf("streaming chat completions API returned %s", resp.Status)
	}

	var builder strings.Builder
	var usage TokenUsage
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var event chatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return GenerateOutput{}, fmt.Errorf("decode streaming chat completion event: %w", err)
		}
		if event.Usage != nil {
			usage = TokenUsage{
				PromptTokens:     event.Usage.PromptTokens,
				CompletionTokens: event.Usage.CompletionTokens,
				TotalTokens:      event.Usage.TotalTokens,
			}
		}

		for _, choice := range event.Choices {
			token := choice.Delta.Content
			if token == "" {
				continue
			}
			builder.WriteString(token)
			if onToken != nil {
				if err := onToken(token); err != nil {
					return GenerateOutput{}, fmt.Errorf("handle streaming token: %w", err)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return GenerateOutput{}, fmt.Errorf("read streaming chat completion: %w", err)
	}

	content := strings.TrimSpace(builder.String())
	if content == "" {
		return GenerateOutput{}, fmt.Errorf("streaming chat completion response is empty")
	}

	return GenerateOutput{
		Content: content,
		Usage:   usage,
	}, nil
}

func (c *OpenAIClient) validateAndConvertMessages(input []Message) ([]chatCompletionMessage, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if c.model == "" {
		return nil, fmt.Errorf("chat model is required")
	}
	if len(input) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	messages := make([]chatCompletionMessage, 0, len(input))
	for i, message := range input {
		if strings.TrimSpace(message.Role) == "" {
			return nil, fmt.Errorf("message role at index %d is required", i)
		}
		if strings.TrimSpace(message.Content) == "" {
			return nil, fmt.Errorf("message content at index %d is required", i)
		}
		messages = append(messages, chatCompletionMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}

	return messages, nil
}

type createChatCompletionRequest struct {
	Model         string                       `json:"model"`
	Messages      []chatCompletionMessage      `json:"messages"`
	Temperature   float64                      `json:"temperature,omitempty"`
	Stream        bool                         `json:"stream,omitempty"`
	StreamOptions *chatCompletionStreamOptions `json:"stream_options,omitempty"`
}

type chatCompletionStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
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

type chatCompletionStreamResponse struct {
	Choices []chatCompletionStreamChoice `json:"choices"`
	Usage   *chatCompletionUsage         `json:"usage"`
}

type chatCompletionStreamChoice struct {
	Delta chatCompletionMessage `json:"delta"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}
