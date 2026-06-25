package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// OpenAI: nên dùng "text-embedding-3-small" cho RAG MVP vì rẻ, nhanh, vector 1536 chiều.
// Nếu cần chất lượng cao hơn và chấp nhận vector lớn hơn, cân nhắc "text-embedding-3-large".
type OpenAIClient struct {
	apiKey     string
	model      string
	dimensions int
	baseURL    string
	httpClient *http.Client
}

type OpenAIConfig struct {
	APIKey     string
	Model      string
	Dimensions int
	BaseURL    string
}

func NewOpenAIClient(config OpenAIConfig) *OpenAIClient {
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}

	return &OpenAIClient{
		apiKey:     config.APIKey,
		model:      config.Model,
		dimensions: config.Dimensions,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *OpenAIClient) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if c.model == "" {
		return nil, fmt.Errorf("embedding model is required")
	}
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("embedding input at index %d is empty", i)
		}
	}

	requestBody := createEmbeddingRequest{
		Model:          c.model,
		Input:          texts,
		EncodingFormat: "float",
	}
	if c.dimensions > 0 {
		requestBody.Dimensions = c.dimensions
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call embeddings API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var apiErr openAIErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("embeddings API returned %s: %s", resp.Status, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("embeddings API returned %s", resp.Status)
	}

	var result createEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(result.Data) != len(texts) {
		return nil, fmt.Errorf("embedding response count mismatch: got %d, want %d", len(result.Data), len(texts))
	}

	sort.Slice(result.Data, func(i, j int) bool {
		return result.Data[i].Index < result.Data[j].Index
	})

	vectors := make([][]float32, 0, len(result.Data))
	for _, item := range result.Data {
		vector := make([]float32, 0, len(item.Embedding))
		for _, value := range item.Embedding {
			vector = append(vector, float32(value))
		}
		vectors = append(vectors, vector)
	}

	return vectors, nil
}

type createEmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format"`
	Dimensions     int      `json:"dimensions,omitempty"`
}

type createEmbeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}
