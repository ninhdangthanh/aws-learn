package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ninhdangthanh/rag-chatbot/internal/config"
	"github.com/ninhdangthanh/rag-chatbot/internal/service"
)

type Handler struct {
	docService  *service.DocumentService
	retrieval   *service.RetrievalService
	chatService *service.ChatService
	cfg         *config.Config
}

func NewHandler(docService *service.DocumentService, retrieval *service.RetrievalService, chatService *service.ChatService, cfg *config.Config) *Handler {
	return &Handler{docService: docService, retrieval: retrieval, chatService: chatService, cfg: cfg}
}

func (h *Handler) UploadDocument(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	doc, _, err := h.docService.UploadDocument(c.Request.Context(), file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, doc)
}

func (h *Handler) ListDocuments(c *gin.Context) {
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	docs, err := h.docService.ListDocuments(c.Request.Context(), status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": docs})
}

func (h *Handler) GetDocument(c *gin.Context) {
	id := c.Param("id")
	doc, err := h.docService.GetDocument(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if doc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *Handler) DeleteDocument(c *gin.Context) {
	id := c.Param("id")
	if err := h.docService.DeleteDocument(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
