package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
)

type DocumentStore interface {
	Create(ctx context.Context, input repository.CreateDocumentInput) (model.Document, error)
	Get(ctx context.Context, id uuid.UUID) (model.Document, error)
	List(ctx context.Context, input repository.ListDocumentsInput) ([]model.Document, error)
	UpdateStatus(ctx context.Context, input repository.UpdateDocumentStatusInput) (model.Document, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type DocumentTaskDistributor interface {
	EnqueueParseDocument(ctx context.Context, documentID uuid.UUID, filePath string) error
}

type DocumentVectorStore interface {
	DeleteByDocument(ctx context.Context, documentID uuid.UUID) error
}

type DocumentHandler struct {
	repo            DocumentStore
	taskDistributor DocumentTaskDistributor
	vectorStore     DocumentVectorStore
	uploadDir       string
	maxFileSizeByte int64
}

func NewDocumentHandler(repo DocumentStore, taskDistributor DocumentTaskDistributor, vectorStore DocumentVectorStore, uploadDir string, maxFileSizeByte int64) *DocumentHandler {
	return &DocumentHandler{
		repo:            repo,
		taskDistributor: taskDistributor,
		vectorStore:     vectorStore,
		uploadDir:       uploadDir,
		maxFileSizeByte: maxFileSizeByte,
	}
}

func (h *DocumentHandler) Upload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "file is required")
		return
	}

	if fileHeader.Size <= 0 {
		writeError(c, http.StatusBadRequest, "invalid_file", "file must not be empty")
		return
	}

	if fileHeader.Size > h.maxFileSizeByte {
		writeError(c, http.StatusBadRequest, "file_too_large", fmt.Sprintf("file size must be <= %d bytes", h.maxFileSizeByte))
		return
	}

	if err := validatePDF(fileHeader); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_file_type", err.Error())
		return
	}

	if err := os.MkdirAll(h.uploadDir, 0o755); err != nil {
		writeError(c, http.StatusInternalServerError, "storage_error", "failed to prepare upload directory")
		return
	}

	filename := sanitizeFilename(fileHeader.Filename)
	storedName := fmt.Sprintf("%s_%s", uuid.NewString(), filename)
	storedPath := filepath.Join(h.uploadDir, storedName)

	if err := c.SaveUploadedFile(fileHeader, storedPath); err != nil {
		writeError(c, http.StatusInternalServerError, "storage_error", "failed to save uploaded file")
		return
	}

	document, err := h.repo.Create(c.Request.Context(), repository.CreateDocumentInput{
		Filename:    filename,
		StoragePath: &storedPath,
		FileSize:    fileHeader.Size,
		FileType:    "pdf",
		Status:      model.DocumentStatusPending,
	})
	if err != nil {
		_ = os.Remove(storedPath)
		writeError(c, http.StatusInternalServerError, "database_error", "failed to create document")
		return
	}

	if err := h.taskDistributor.EnqueueParseDocument(c.Request.Context(), document.ID, storedPath); err != nil {
		_ = os.Remove(storedPath)
		message := err.Error()
		_, _ = h.repo.UpdateStatus(c.Request.Context(), repository.UpdateDocumentStatusInput{
			ID:       document.ID,
			Status:   model.DocumentStatusFailed,
			ErrorMsg: &message,
		})
		writeError(c, http.StatusInternalServerError, "queue_error", "failed to enqueue parse job")
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"id":       document.ID,
		"filename": document.Filename,
		"status":   document.Status,
	})
}

func (h *DocumentHandler) GetStatus(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "document id must be a valid UUID")
		return
	}

	document, err := h.repo.Get(c.Request.Context(), documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "document not found")
			return
		}

		writeError(c, http.StatusInternalServerError, "database_error", "failed to load document")
		return
	}

	c.JSON(http.StatusOK, document)
}

func (h *DocumentHandler) List(c *gin.Context) {
	page, limit, ok := parsePagination(c)
	if !ok {
		return
	}

	status := model.DocumentStatus(strings.TrimSpace(c.Query("status")))
	documents, err := h.repo.List(c.Request.Context(), repository.ListDocumentsInput{
		Status: status,
		Limit:  limit,
		Offset: (page - 1) * limit,
	})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "database_error", "failed to list documents")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": documents,
		"page":      page,
		"limit":     limit,
		"status":    status,
	})
}

func (h *DocumentHandler) Delete(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "document id must be a valid UUID")
		return
	}

	document, err := h.repo.Get(c.Request.Context(), documentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "document not found")
			return
		}
		writeError(c, http.StatusInternalServerError, "database_error", "failed to load document")
		return
	}

	if h.vectorStore != nil {
		if err := h.vectorStore.DeleteByDocument(c.Request.Context(), documentID); err != nil {
			writeError(c, http.StatusInternalServerError, "vector_error", "failed to delete document vectors")
			return
		}
	}

	if err := h.repo.Delete(c.Request.Context(), documentID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "document not found")
			return
		}
		writeError(c, http.StatusInternalServerError, "database_error", "failed to delete document")
		return
	}

	if document.StoragePath != nil {
		_ = os.Remove(*document.StoragePath)
	}

	c.Status(http.StatusNoContent)
}

func validatePDF(fileHeader *multipart.FileHeader) error {
	if strings.ToLower(filepath.Ext(fileHeader.Filename)) != ".pdf" {
		return fmt.Errorf("only PDF files are supported")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to inspect uploaded file")
	}
	defer file.Close()

	header := make([]byte, 512)
	n, err := file.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to inspect uploaded file")
	}

	if !bytes.HasPrefix(header[:n], []byte("%PDF")) {
		return fmt.Errorf("only PDF files are supported")
	}

	return nil
}

func sanitizeFilename(name string) string {
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, " ", "_")
	if base == "." || base == "" {
		return "document.pdf"
	}

	return base
}

func writeError(c *gin.Context, status int, code, message string) {
	errorBody := gin.H{
		"code":    code,
		"message": message,
	}
	if requestID, exists := c.Get("request_id"); exists {
		errorBody["request_id"] = requestID
	}

	c.JSON(status, gin.H{
		"error": errorBody,
	})
}

func parsePagination(c *gin.Context) (int, int, bool) {
	page := 1
	if value := c.Query("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(c, http.StatusBadRequest, "invalid_request", "page must be a positive integer")
			return 0, 0, false
		}
		page = parsed
	}

	limit := 20
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(c, http.StatusBadRequest, "invalid_request", "limit must be a positive integer")
			return 0, 0, false
		}
		limit = parsed
	}
	if limit > 100 {
		limit = 100
	}

	return page, limit, true
}
