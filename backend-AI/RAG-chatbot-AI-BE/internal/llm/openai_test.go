package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestOpenAIClientGenerate(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.test/chat/completions" {
			t.Fatalf("unexpected url: %s", req.URL.String())
		}
		if got, want := req.Header.Get("Authorization"), "Bearer test-key"; got != want {
			t.Fatalf("expected authorization %q, got %q", want, got)
		}

		var request createChatCompletionRequest
		if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "gpt-test" {
			t.Fatalf("expected model gpt-test, got %q", request.Model)
		}
		if len(request.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(request.Messages))
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(bytes.NewBufferString(`{
			"choices": [
				{"message": {"role": "assistant", "content": "Grounded answer"}}
			],
			"usage": {
				"prompt_tokens": 12,
				"completion_tokens": 5,
				"total_tokens": 17
			}
		}`)),
		}, nil
	})

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "test-key",
		Model:   "gpt-test",
		BaseURL: "https://api.test",
	})
	client.httpClient = &http.Client{Transport: transport}

	output, err := client.Generate(context.Background(), GenerateInput{
		Messages: []Message{
			{Role: "system", Content: "Use context only."},
			{Role: "user", Content: "Question"},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if output.Content != "Grounded answer" {
		t.Fatalf("unexpected content: %q", output.Content)
	}
	if output.Usage.PromptTokens != 12 || output.Usage.CompletionTokens != 5 || output.Usage.TotalTokens != 17 {
		t.Fatalf("unexpected usage: %+v", output.Usage)
	}
}
