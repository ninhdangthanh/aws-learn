package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/service"
)

type SemanticSearcher interface {
	Search(ctx context.Context, input service.SearchInput) (service.SearchOutput, error)
}

type SearchHandler struct {
	searcher SemanticSearcher
}

type searchRequest struct {
	// Query is the natural-language question or search phrase to embed.
	// Example: "What is the refund policy?"
	Query string `json:"query"`

	// TopK is the maximum number of most relevant chunks to return.
	// Example: 5 returns up to five matching chunks. If omitted or 0, the service default is used.
	TopK int `json:"top_k"`

	// ScoreThreshold filters out results below the similarity score.
	// Example: 0.7 returns only chunks with score >= 0.7. If omitted, no threshold is applied.
	ScoreThreshold *float64 `json:"score_threshold"`
}

func NewSearchHandler(searcher SemanticSearcher) *SearchHandler {
	return &SearchHandler{
		searcher: searcher,
	}
}

func (h *SearchHandler) Search(c *gin.Context) {
	var request searchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	request.Query = strings.TrimSpace(request.Query)
	if request.Query == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "query is required")
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

	response, err := h.searcher.Search(c.Request.Context(), service.SearchInput{
		Query:          request.Query,
		TopK:           request.TopK,
		ScoreThreshold: request.ScoreThreshold,
	})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "search_error", "failed to search documents")
		return
	}

	c.JSON(http.StatusOK, response)
}
