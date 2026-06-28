package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/llm"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
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

func (f *fakeAnswerGenerator) StreamGenerate(ctx context.Context, input llm.GenerateInput, onToken llm.TokenHandler) (llm.GenerateOutput, error) {
	f.input = input
	for _, token := range []string{"Grounded ", "answer"} {
		if err := onToken(token); err != nil {
			return llm.GenerateOutput{}, err
		}
	}
	return f.output, f.err
}

type fakeChatStore struct {
	session         model.ChatSession
	history         []model.ChatMessage
	createdSessions []repository.CreateChatSessionInput
	createdMessages []repository.CreateChatMessageInput
	touchedSessions []uuid.UUID
}

func (f *fakeChatStore) CreateSession(ctx context.Context, input repository.CreateChatSessionInput) (model.ChatSession, error) {
	f.createdSessions = append(f.createdSessions, input)
	if f.session.ID == uuid.Nil {
		f.session.ID = uuid.MustParse("33333333-3333-3333-3333-333333333333")
		f.session.CreatedAt = time.Now()
		f.session.UpdatedAt = f.session.CreatedAt
	}
	f.session.Title = input.Title
	return f.session, nil
}

func (f *fakeChatStore) GetSession(ctx context.Context, id uuid.UUID) (model.ChatSession, error) {
	if f.session.ID == uuid.Nil {
		f.session.ID = id
	}
	return f.session, nil
}

func (f *fakeChatStore) CreateMessage(ctx context.Context, input repository.CreateChatMessageInput) (model.ChatMessage, error) {
	f.createdMessages = append(f.createdMessages, input)
	return model.ChatMessage{
		ID:        uuid.New(),
		SessionID: input.SessionID,
		Role:      input.Role,
		Content:   input.Content,
	}, nil
}

func (f *fakeChatStore) ListRecentMessagesBySession(ctx context.Context, sessionID uuid.UUID, limit int) ([]model.ChatMessage, error) {
	return f.history, nil
}

func (f *fakeChatStore) TouchSession(ctx context.Context, id uuid.UUID) error {
	f.touchedSessions = append(f.touchedSessions, id)
	return nil
}

func TestRAGChatServiceChat(t *testing.T) {
	chunkID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	documentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	page := int32(2)
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
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
	store := &fakeChatStore{
		session: model.ChatSession{ID: sessionID},
		history: []model.ChatMessage{
			{Role: "user", Content: "Tôi đang hỏi về giáo dục."},
			{Role: "assistant", Content: "Bạn muốn hỏi phần nào?"},
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

	output, err := NewRAGChatService(searcher, generator, store).Chat(context.Background(), ChatInput{
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
	if !strings.Contains(generator.input.Messages[1].Content, "Tôi đang hỏi về giáo dục.") {
		t.Fatalf("expected prompt to include recent history, got %q", generator.input.Messages[1].Content)
	}
	if output.Answer == "" || len(output.Citations) != 1 {
		t.Fatalf("unexpected output: %+v", output)
	}
	if output.SessionID != sessionID {
		t.Fatalf("expected session_id %s, got %s", sessionID, output.SessionID)
	}
	if output.TokenUsage.TotalTokens != 120 {
		t.Fatalf("unexpected usage: %+v", output.TokenUsage)
	}
	if len(store.createdSessions) != 1 {
		t.Fatalf("expected one created session, got %d", len(store.createdSessions))
	}
	if len(store.createdMessages) != 2 {
		t.Fatalf("expected user and assistant messages to be saved, got %d", len(store.createdMessages))
	}
	if store.createdMessages[0].Role != "user" || store.createdMessages[1].Role != "assistant" {
		t.Fatalf("unexpected saved messages: %+v", store.createdMessages)
	}
	if store.createdMessages[1].Citations == nil {
		t.Fatal("expected assistant citations to be saved")
	}
	var citations []Citation
	if err := json.Unmarshal(*store.createdMessages[1].Citations, &citations); err != nil {
		t.Fatalf("unmarshal saved citations: %v", err)
	}
	if len(citations) != 1 {
		t.Fatalf("expected one saved citation, got %d", len(citations))
	}
}

func TestRAGChatServiceReturnsInsufficientAnswerWhenNoContext(t *testing.T) {
	searcher := &fakeChatSearcher{
		output: SearchOutput{Query: "unknown", Results: []SearchMatch{}},
	}
	generator := &fakeAnswerGenerator{}
	store := &fakeChatStore{}

	output, err := NewRAGChatService(searcher, generator, store).Chat(context.Background(), ChatInput{
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
	if output.SessionID == uuid.Nil {
		t.Fatal("expected session_id to be created")
	}
	if len(store.createdMessages) != 2 {
		t.Fatalf("expected insufficient exchange to be saved, got %d messages", len(store.createdMessages))
	}
}

func TestRAGChatServiceStreamChat(t *testing.T) {
	page := int32(2)
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	searcher := &fakeChatSearcher{
		output: SearchOutput{
			Query: "Công nghệ có thay thế giáo viên không?",
			Results: []SearchMatch{
				{
					ChunkID:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
					DocumentID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
					Filename:   "giao-duc-10-topic.pdf",
					PageNumber: &page,
					ChunkIndex: 2,
					Text:       "Công nghệ hỗ trợ giáo viên.",
					Score:      0.42,
				},
			},
		},
	}
	store := &fakeChatStore{session: model.ChatSession{ID: sessionID}}
	generator := &fakeAnswerGenerator{
		output: llm.GenerateOutput{
			Content: "Grounded answer",
			Usage:   llm.TokenUsage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
		},
	}

	var events []StreamEvent
	err := NewRAGChatService(searcher, generator, store).StreamChat(context.Background(), ChatInput{
		Question: "Công nghệ có thay thế giáo viên không?",
	}, func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("stream chat: %v", err)
	}

	if len(events) < 4 {
		t.Fatalf("expected token, citations, done events, got %+v", events)
	}
	if events[0].Type != "token" || events[1].Type != "token" {
		t.Fatalf("expected token events first, got %+v", events[:2])
	}
	if events[len(events)-2].Type != "citations" || events[len(events)-1].Type != "done" {
		t.Fatalf("expected final citations/done events, got %+v", events)
	}
	if len(store.createdMessages) != 2 {
		t.Fatalf("expected stream exchange to be saved, got %d messages", len(store.createdMessages))
	}
}
