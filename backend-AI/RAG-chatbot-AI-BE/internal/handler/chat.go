package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ninhdangthanh/rag-chatbot/internal/model"
)

func (h *Handler) Chat(c *gin.Context) {
	var req model.ChatRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TopK == 0 {
		req.TopK = h.cfg.SearchTopK
	}
	start := time.Now()
	resp, err := h.chatService.Chat(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.LatencyMs = time.Now().Sub(start).Milliseconds()
	c.JSON(http.StatusOK, resp)
}
