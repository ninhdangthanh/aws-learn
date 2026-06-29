package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
)

type ChatRepository struct {
	db *gorm.DB
}

type CreateChatSessionInput struct {
	Title *string
}

type CreateChatMessageInput struct {
	SessionID  uuid.UUID
	Role       string
	Content    string
	Citations  *json.RawMessage
	TokenUsage *json.RawMessage
	LatencyMs  *int32
}

func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{
		db: db,
	}
}

func (r *ChatRepository) CreateSession(ctx context.Context, input CreateChatSessionInput) (model.ChatSession, error) {
	session := model.ChatSession{Title: input.Title}
	err := r.db.WithContext(ctx).Create(&session).Error
	return session, err
}

func (r *ChatRepository) GetSession(ctx context.Context, id uuid.UUID) (model.ChatSession, error) {
	var session model.ChatSession
	err := r.db.WithContext(ctx).First(&session, "id = ?", id).Error
	return session, err
}

func (r *ChatRepository) ListSessions(ctx context.Context, limit, offset int) ([]model.ChatSession, error) {
	var sessions []model.ChatSession
	query := r.db.WithContext(ctx).Order("updated_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Find(&sessions).Error
	return sessions, err
}

func (r *ChatRepository) CreateMessage(ctx context.Context, input CreateChatMessageInput) (model.ChatMessage, error) {
	message := model.ChatMessage{
		SessionID:  input.SessionID,
		Role:       input.Role,
		Content:    input.Content,
		Citations:  derefRawMessage(input.Citations),
		TokenUsage: derefRawMessage(input.TokenUsage),
		LatencyMs:  input.LatencyMs,
	}

	err := r.db.WithContext(ctx).Create(&message).Error
	return message, err
}

func (r *ChatRepository) ListMessagesBySession(ctx context.Context, sessionID uuid.UUID, limit int) ([]model.ChatMessage, error) {
	var messages []model.ChatMessage
	query := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&messages).Error
	return messages, err
}

func (r *ChatRepository) ListRecentMessagesBySession(ctx context.Context, sessionID uuid.UUID, limit int) ([]model.ChatMessage, error) {
	var messages []model.ChatMessage
	query := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&messages).Error; err != nil {
		return nil, err
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (r *ChatRepository) TouchSession(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&model.ChatSession{}).
		Where("id = ?", id).
		Update("updated_at", time.Now()).Error
}
