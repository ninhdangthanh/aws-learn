package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ChatSession struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Title     *string   `gorm:"type:varchar(500)" json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ChatSession) TableName() string {
	return "chat_sessions"
}

type ChatMessage struct {
	ID         uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SessionID  uuid.UUID       `gorm:"type:uuid;not null;index" json:"session_id"`
	Role       string          `gorm:"type:varchar(20);not null" json:"role"`
	Content    string          `gorm:"type:text;not null" json:"content"`
	Citations  json.RawMessage `gorm:"type:jsonb" json:"citations"`
	TokenUsage json.RawMessage `gorm:"type:jsonb" json:"token_usage"`
	LatencyMs  *int32          `json:"latency_ms"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (ChatMessage) TableName() string {
	return "chat_messages"
}
