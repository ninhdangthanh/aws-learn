package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ninhdangthanh/rag-chatbot/internal/model"
)

func (h *Handler) Search(c *gin.Context) {
	var req model.SearchRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TopK == 0 {
		req.TopK = h.cfg.SearchTopK
	}
	results, err := h.retrieval.Search(c.Request.Context(), req.Query, req.TopK, req.ScoreThreshold)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.SearchResponse{Query: req.Query, Results: results, LatencyMs: time.Now().UnixMilli()})
}
