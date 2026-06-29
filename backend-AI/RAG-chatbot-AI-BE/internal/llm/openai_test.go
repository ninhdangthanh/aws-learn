package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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

func TestOpenAIClientStreamGenerate(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var request createChatCompletionRequest
		if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !request.Stream {
			t.Fatal("expected stream=true")
		}
		if request.StreamOptions == nil || !request.StreamOptions.IncludeUsage {
			t.Fatalf("expected include_usage stream option, got %+v", request.StreamOptions)
		}

		body := strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"Hello"}}],"usage":null}`,
			`data: {"choices":[{"delta":{"content":" world"}}],"usage":null}`,
			`data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
			`data: [DONE]`,
			``,
		}, "\n\n")

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	})

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "test-key",
		Model:   "gpt-test",
		BaseURL: "https://api.test",
	})
	client.httpClient = &http.Client{Transport: transport}

	var tokens []string
	output, err := client.StreamGenerate(context.Background(), GenerateInput{
		Messages: []Message{
			{Role: "system", Content: "Use context only."},
			{Role: "user", Content: "Question"},
		},
	}, func(token string) error {
		tokens = append(tokens, token)
		return nil
	})
	if err != nil {
		t.Fatalf("stream generate: %v", err)
	}

	if output.Content != "Hello world" {
		t.Fatalf("unexpected content: %q", output.Content)
	}
	if strings.Join(tokens, "") != "Hello world" {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
	if output.Usage.TotalTokens != 12 {
		t.Fatalf("unexpected usage: %+v", output.Usage)
	}
}
