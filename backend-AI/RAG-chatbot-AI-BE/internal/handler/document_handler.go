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
	UpdateStatus(ctx context.Context, input repository.UpdateDocumentStatusInput) (model.Document, error)
}

type DocumentTaskDistributor interface {
	EnqueueParseDocument(ctx context.Context, documentID uuid.UUID, filePath string) error
}

type DocumentHandler struct {
	repo            DocumentStore
	taskDistributor DocumentTaskDistributor
	uploadDir       string
	maxFileSizeByte int64
}

func NewDocumentHandler(repo DocumentStore, taskDistributor DocumentTaskDistributor, uploadDir string, maxFileSizeByte int64) *DocumentHandler {
	return &DocumentHandler{
		repo:            repo,
		taskDistributor: taskDistributor,
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
		Status:      "pending",
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
			Status:   "failed",
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
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
