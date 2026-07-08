package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"multipart-s3-upload/internal/s3svc"
)

// Handler holds the dependencies for the upload HTTP endpoints.
type Handler struct {
	s3        *s3svc.Service
	keyPrefix string
	partSize  int64
}

func New(s3 *s3svc.Service, keyPrefix string, partSize int64) *Handler {
	return &Handler{s3: s3, keyPrefix: keyPrefix, partSize: partSize}
}

// Register wires the four multipart endpoints onto the router group.
func (h *Handler) Register(r *gin.RouterGroup) {
	r.POST("/uploads/init", h.Init)
	r.POST("/uploads/presign-parts", h.PresignParts)
	r.POST("/uploads/complete", h.Complete)
	r.POST("/uploads/abort", h.Abort)
}

// ---- POST /uploads/init ------------------------------------------------------

type initRequest struct {
	Filename    string `json:"filename" binding:"required"`
	Size        int64  `json:"size" binding:"required"`
	ContentType string `json:"contentType"`
}

type initResponse struct {
	Key      string `json:"key"`
	UploadID string `json:"uploadId"`
	PartSize int64  `json:"partSize"`
}

// Init validates the request, generates an object key and starts the multipart
// upload. It returns the uploadId + suggested partSize the client will use to
// slice the file.
func (h *Handler) Init(c *gin.Context) {
	var req initRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	key := h.buildKey(req.Filename)
	uploadID, err := h.s3.CreateMultipartUpload(c.Request.Context(), key, contentType)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "create multipart upload failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, initResponse{Key: key, UploadID: uploadID, PartSize: h.partSize})
}

// ---- POST /uploads/presign-parts --------------------------------------------

type presignRequest struct {
	Key         string  `json:"key" binding:"required"`
	UploadID    string  `json:"uploadId" binding:"required"`
	PartNumbers []int32 `json:"partNumbers" binding:"required"`
}

type presignedPart struct {
	PartNumber int32  `json:"partNumber"`
	URL        string `json:"url"`
}

// PresignParts returns a presigned URL for each requested part number. The
// client can ask for all parts at once or in batches (e.g. as it retries).
func (h *Handler) PresignParts(c *gin.Context) {
	var req presignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	urls := make([]presignedPart, 0, len(req.PartNumbers))
	for _, n := range req.PartNumbers {
		if n < 1 || n > 10000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "partNumber must be between 1 and 10000"})
			return
		}
		url, err := h.s3.PresignUploadPart(c.Request.Context(), req.Key, req.UploadID, n)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "presign failed: " + err.Error()})
			return
		}
		urls = append(urls, presignedPart{PartNumber: n, URL: url})
	}

	c.JSON(http.StatusOK, gin.H{"parts": urls})
}

// ---- POST /uploads/complete --------------------------------------------------

type completeRequest struct {
	Key      string `json:"key" binding:"required"`
	UploadID string `json:"uploadId" binding:"required"`
	Parts    []struct {
		PartNumber int32  `json:"partNumber"`
		ETag       string `json:"etag"`
	} `json:"parts" binding:"required"`
}

// Complete assembles the uploaded parts into the final object. Parts are sorted
// by part number because S3 requires ascending order.
func (h *Handler) Complete(c *gin.Context) {
	var req completeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Parts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parts must not be empty"})
		return
	}

	parts := make([]s3svc.CompletedPart, len(req.Parts))
	for i, p := range req.Parts {
		parts[i] = s3svc.CompletedPart{PartNumber: p.PartNumber, ETag: p.ETag}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })

	location, err := h.s3.CompleteMultipartUpload(c.Request.Context(), req.Key, req.UploadID, parts)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "complete failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"key": req.Key, "location": location})
}

// ---- POST /uploads/abort -----------------------------------------------------

type abortRequest struct {
	Key      string `json:"key" binding:"required"`
	UploadID string `json:"uploadId" binding:"required"`
}

// Abort cancels an in-progress upload so S3 stops charging for orphaned parts.
func (h *Handler) Abort(c *gin.Context) {
	var req abortRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.s3.AbortMultipartUpload(c.Request.Context(), req.Key, req.UploadID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "abort failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "aborted"})
}

// buildKey produces a collision-resistant object key: prefix + date + random +
// sanitized original filename.
func (h *Handler) buildKey(filename string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	safe := path.Base(strings.ReplaceAll(filename, "\\", "/"))
	safe = strings.ReplaceAll(safe, " ", "_")
	return h.keyPrefix + time.Now().UTC().Format("2006/01/02/") + hex.EncodeToString(buf) + "-" + safe
}
