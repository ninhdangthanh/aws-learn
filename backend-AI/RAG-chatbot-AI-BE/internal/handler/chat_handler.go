package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/service"
)

type RAGChatter interface {
	Chat(ctx context.Context, input service.ChatInput) (service.ChatOutput, error)
	StreamChat(ctx context.Context, input service.ChatInput, onEvent service.StreamEventHandler) error
}

type ChatHandler struct {
	chatter RAGChatter
}

type chatRequest struct {
	// Question is the user's natural-language question.
	// Example: "Công nghệ có thay thế giáo viên không?"
	Question string `json:"question"`

	// SessionID is optional. If omitted, the service creates a new chat session.
	SessionID *string `json:"session_id"`

	// TopK is the maximum number of retrieved chunks used as context.
	// Example: 5 retrieves up to five chunks. If omitted or 0, the service default is used.
	TopK int `json:"top_k"`

	// ScoreThreshold filters out retrieved chunks below the similarity score.
	// Example: 0.3 keeps chunks with score >= 0.3. If omitted, no threshold is applied.
	ScoreThreshold *float64 `json:"score_threshold"`

	// Stream switches the endpoint to Server-Sent Events token streaming.
	Stream bool `json:"stream"`
}

func NewChatHandler(chatter RAGChatter) *ChatHandler {
	return &ChatHandler{
		chatter: chatter,
	}
}

func (h *ChatHandler) Chat(c *gin.Context) {
	var request chatRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	request.Question = strings.TrimSpace(request.Question)
	if request.Question == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "question is required")
		return
	}

	if request.TopK < 0 {
		writeError(c, http.StatusBadRequest, "invalid_request", "top_k must be greater than or equal to 0")
		return
	}

	if request.ScoreThreshold != nil && (*request.ScoreThreshold < 0 || *request.ScoreThreshold > 1) {
		writeError(c, http.StatusBadRequest, "invalid_request", "score_threshold must be between 0 and 1")
		return
	}

	var sessionID *uuid.UUID
	if request.SessionID != nil && strings.TrimSpace(*request.SessionID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*request.SessionID))
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "session_id must be a valid UUID")
			return
		}
		sessionID = &parsed
	}

	input := service.ChatInput{
		Question:       request.Question,
		SessionID:      sessionID,
		TopK:           request.TopK,
		ScoreThreshold: request.ScoreThreshold,
	}

	if request.Stream {
		h.streamChat(c, input)
		return
	}

	response, err := h.chatter.Chat(c.Request.Context(), input)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "chat_error", "failed to answer question")
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *ChatHandler) streamChat(c *gin.Context, input service.ChatInput) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	writeEvent := func(event service.StreamEvent) error {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}

	if err := h.chatter.StreamChat(c.Request.Context(), input, writeEvent); err != nil {
		_ = writeEvent(service.StreamEvent{
			Type:    "error",
			Content: "failed to stream answer",
		})
	}
}
