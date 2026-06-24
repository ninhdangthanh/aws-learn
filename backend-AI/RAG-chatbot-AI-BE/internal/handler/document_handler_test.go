package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/model"
	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/repository"
)

type fakeDocumentStore struct {
	createFn func(ctx context.Context, input repository.CreateDocumentInput) (model.Document, error)
	getFn    func(ctx context.Context, id uuid.UUID) (model.Document, error)
}

func (f *fakeDocumentStore) Create(ctx context.Context, input repository.CreateDocumentInput) (model.Document, error) {
	return f.createFn(ctx, input)
}

func (f *fakeDocumentStore) Get(ctx context.Context, id uuid.UUID) (model.Document, error) {
	return f.getFn(ctx, id)
}

func TestUploadDocumentSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	handler := NewDocumentHandler(&fakeDocumentStore{
		createFn: func(ctx context.Context, input repository.CreateDocumentInput) (model.Document, error) {
			return model.Document{
				ID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				Filename: input.Filename,
				Status:   input.Status,
			}, nil
		},
		getFn: func(ctx context.Context, id uuid.UUID) (model.Document, error) {
			return model.Document{}, nil
		},
	}, tempDir, 1024*1024)

	router := gin.New()
	router.POST("/api/v1/documents", handler.Upload)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "sample.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	if _, err := part.Write([]byte("%PDF-1.4 sample")); err != nil {
		t.Fatalf("write multipart: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}

	if len(files) != 1 || !strings.HasSuffix(files[0].Name(), ".pdf") {
		t.Fatalf("expected one saved pdf file, got %v", files)
	}
}

func TestUploadDocumentRejectsNonPDF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	handler := NewDocumentHandler(&fakeDocumentStore{
		createFn: func(ctx context.Context, input repository.CreateDocumentInput) (model.Document, error) {
			return model.Document{}, nil
		},
		getFn: func(ctx context.Context, id uuid.UUID) (model.Document, error) {
			return model.Document{}, nil
		},
	}, tempDir, 1024*1024)

	router := gin.New()
	router.POST("/api/v1/documents", handler.Upload)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "sample.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	if _, err := part.Write([]byte("hello")); err != nil {
		t.Fatalf("write multipart: %v", err)
	}

	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	expectedID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	handler := NewDocumentHandler(&fakeDocumentStore{
		createFn: func(ctx context.Context, input repository.CreateDocumentInput) (model.Document, error) {
			return model.Document{}, nil
		},
		getFn: func(ctx context.Context, id uuid.UUID) (model.Document, error) {
			return model.Document{
				ID:       id,
				Filename: "demo.pdf",
				Status:   "pending",
			}, nil
		},
	}, t.TempDir(), 1024*1024)

	router := gin.New()
	router.GET("/api/v1/documents/:id", handler.GetStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+expectedID.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var document model.Document
	if err := json.Unmarshal(rec.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode document: %v", err)
	}

	if document.ID != expectedID {
		t.Fatalf("expected id %s, got %s", expectedID, document.ID)
	}
}

func TestGetStatusNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewDocumentHandler(&fakeDocumentStore{
		createFn: func(ctx context.Context, input repository.CreateDocumentInput) (model.Document, error) {
			return model.Document{}, nil
		},
		getFn: func(ctx context.Context, id uuid.UUID) (model.Document, error) {
			return model.Document{}, gorm.ErrRecordNotFound
		},
	}, t.TempDir(), 1024*1024)

	router := gin.New()
	router.GET("/api/v1/documents/:id", handler.GetStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestUploadRemovesFileWhenCreateFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	handler := NewDocumentHandler(&fakeDocumentStore{
		createFn: func(ctx context.Context, input repository.CreateDocumentInput) (model.Document, error) {
			return model.Document{}, errors.New("db down")
		},
		getFn: func(ctx context.Context, id uuid.UUID) (model.Document, error) {
			return model.Document{}, nil
		},
	}, tempDir, 1024*1024)

	router := gin.New()
	router.POST("/api/v1/documents", handler.Upload)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "sample.pdf")
	_, _ = part.Write([]byte("%PDF-1.4 sample"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	matches, err := filepath.Glob(filepath.Join(tempDir, "*.pdf"))
	if err != nil {
		t.Fatalf("glob uploads: %v", err)
	}

	if len(matches) != 0 {
		t.Fatalf("expected uploaded file to be removed on db error, got %v", matches)
	}
}
