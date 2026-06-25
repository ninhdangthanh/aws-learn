package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// Gemini: nên dùng "gemini-embedding-2" cho stack Google/Gemini mới.
// Nếu cần text-only batch cũ với task_type, cân nhắc "gemini-embedding-001".
type GeminiClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

type GeminiConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

func NewGeminiClient(config GeminiConfig) *GeminiClient {
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}

	return &GeminiClient{
		apiKey:     config.APIKey,
		model:      config.Model,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *GeminiClient) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}
	if c.model == "" {
		return nil, fmt.Errorf("gemini embedding model is required")
	}
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	vectors := make([][]float32, 0, len(texts))
	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("embedding input at index %d is empty", i)
		}

		vector, err := c.embedText(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text at index %d: %w", i, err)
		}
		vectors = append(vectors, vector)
	}

	return vectors, nil
}

func (c *GeminiClient) embedText(ctx context.Context, text string) ([]float32, error) {
	requestBody := geminiEmbeddingRequest{
		Model: "models/" + c.model,
		Content: geminiContent{
			Parts: []geminiPart{{Text: text}},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini embedding request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/models/%s:embedContent", c.baseURL, c.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create gemini embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call gemini embeddings API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var apiErr geminiErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("gemini embeddings API returned %s: %s", resp.Status, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("gemini embeddings API returned %s", resp.Status)
	}

	var result geminiEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode gemini embedding response: %w", err)
	}

	if len(result.Embedding.Values) == 0 {
		return nil, fmt.Errorf("gemini embedding response is empty")
	}

	return result.Embedding.Values, nil
}

type geminiEmbeddingRequest struct {
	Model   string        `json:"model"`
	Content geminiContent `json:"content"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiEmbeddingResponse struct {
	Embedding geminiEmbedding `json:"embedding"`
}

type geminiEmbedding struct {
	Values []float32 `json:"values"`
}

type geminiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}
