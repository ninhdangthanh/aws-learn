package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
)

const (
	defaultChatListLimit = 20
	maxChatListLimit     = 100
)

type ChatHistoryStore interface {
	ListSessions(ctx context.Context, limit, offset int) ([]model.ChatSession, error)
	GetSession(ctx context.Context, id uuid.UUID) (model.ChatSession, error)
	ListMessagesBySession(ctx context.Context, sessionID uuid.UUID, limit int) ([]model.ChatMessage, error)
}

type ChatSessionHandler struct {
	store ChatHistoryStore
}

func NewChatSessionHandler(store ChatHistoryStore) *ChatSessionHandler {
	return &ChatSessionHandler{
		store: store,
	}
}

func (h *ChatSessionHandler) ListSessions(c *gin.Context) {
	page, limit, ok := parsePageLimit(c)
	if !ok {
		return
	}

	sessions, err := h.store.ListSessions(c.Request.Context(), limit, (page-1)*limit)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "database_error", "failed to list chat sessions")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"page":     page,
		"limit":    limit,
	})
}

func (h *ChatSessionHandler) ListMessages(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "session id must be a valid UUID")
		return
	}

	if _, err := h.store.GetSession(c.Request.Context(), sessionID); err != nil {
		writeError(c, http.StatusNotFound, "not_found", "chat session not found")
		return
	}

	_, limit, ok := parsePageLimit(c)
	if !ok {
		return
	}

	messages, err := h.store.ListMessagesBySession(c.Request.Context(), sessionID, limit)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "database_error", "failed to list chat messages")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"messages":   messages,
		"limit":      limit,
	})
}

func parsePageLimit(c *gin.Context) (int, int, bool) {
	page := 1
	if value := c.Query("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(c, http.StatusBadRequest, "invalid_request", "page must be a positive integer")
			return 0, 0, false
		}
		page = parsed
	}

	limit := defaultChatListLimit
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(c, http.StatusBadRequest, "invalid_request", "limit must be a positive integer")
			return 0, 0, false
		}
		limit = parsed
	}
	if limit > maxChatListLimit {
		limit = maxChatListLimit
	}

	return page, limit, true
}
