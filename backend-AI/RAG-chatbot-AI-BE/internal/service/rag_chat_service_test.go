package service

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/llm"
)

type fakeChatSearcher struct {
	input  SearchInput
	output SearchOutput
	err    error
}

func (f *fakeChatSearcher) Search(ctx context.Context, input SearchInput) (SearchOutput, error) {
	f.input = input
	return f.output, f.err
}

type fakeAnswerGenerator struct {
	input  llm.GenerateInput
	output llm.GenerateOutput
	err    error
}

func (f *fakeAnswerGenerator) Generate(ctx context.Context, input llm.GenerateInput) (llm.GenerateOutput, error) {
	f.input = input
	return f.output, f.err
}

func TestRAGChatServiceChat(t *testing.T) {
	chunkID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	documentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	page := int32(2)
	searcher := &fakeChatSearcher{
		output: SearchOutput{
			Query: "Công nghệ có thay thế giáo viên không?",
			Results: []SearchMatch{
				{
					ChunkID:    chunkID,
					DocumentID: documentID,
					Filename:   "giao-duc-10-topic.pdf",
					PageNumber: &page,
					ChunkIndex: 2,
					Text:       "Công nghệ không thay thế giáo viên mà hỗ trợ phản hồi cá nhân.",
					Score:      0.42,
				},
			},
		},
	}
	generator := &fakeAnswerGenerator{
		output: llm.GenerateOutput{
			Content: "Công nghệ không thay thế giáo viên [giao-duc-10-topic.pdf, page 2].",
			Usage: llm.TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 20,
				TotalTokens:      120,
			},
		},
	}

	output, err := NewRAGChatService(searcher, generator).Chat(context.Background(), ChatInput{
		Question: " Công nghệ có thay thế giáo viên không? ",
		TopK:     3,
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if searcher.input.Query != "Công nghệ có thay thế giáo viên không?" {
		t.Fatalf("expected trimmed question, got %q", searcher.input.Query)
	}
	if searcher.input.TopK != 3 {
		t.Fatalf("expected top_k 3, got %d", searcher.input.TopK)
	}
	if len(generator.input.Messages) != 2 {
		t.Fatalf("expected 2 prompt messages, got %d", len(generator.input.Messages))
	}
	if !strings.Contains(generator.input.Messages[1].Content, "giao-duc-10-topic.pdf") {
		t.Fatalf("expected context to include filename, got %q", generator.input.Messages[1].Content)
	}
	if output.Answer == "" || len(output.Citations) != 1 {
		t.Fatalf("unexpected output: %+v", output)
	}
	if output.TokenUsage.TotalTokens != 120 {
		t.Fatalf("unexpected usage: %+v", output.TokenUsage)
	}
}

func TestRAGChatServiceReturnsInsufficientAnswerWhenNoContext(t *testing.T) {
	searcher := &fakeChatSearcher{
		output: SearchOutput{Query: "unknown", Results: []SearchMatch{}},
	}
	generator := &fakeAnswerGenerator{}

	output, err := NewRAGChatService(searcher, generator).Chat(context.Background(), ChatInput{
		Question: "unknown",
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if !strings.Contains(output.Answer, "cannot answer") {
		t.Fatalf("expected insufficient answer, got %q", output.Answer)
	}
	if len(generator.input.Messages) != 0 {
		t.Fatalf("expected generator not to be called")
	}
}
