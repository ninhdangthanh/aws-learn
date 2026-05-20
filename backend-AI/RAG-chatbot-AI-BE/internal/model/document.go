package model

import "time"

type DocumentStatus string

const (
	StatusPending   DocumentStatus = "pending"
	StatusParsing   DocumentStatus = "parsing"
	StatusChunked   DocumentStatus = "chunked"
	StatusEmbedding DocumentStatus = "embedding"
	StatusReady     DocumentStatus = "ready"
	StatusFailed    DocumentStatus = "failed"
)

type Document struct {
	ID         string         `json:"id"`
	Filename   string         `json:"filename"`
	FileSize   int64          `json:"file_size"`
	FileType   string         `json:"file_type"`
	Status     DocumentStatus `json:"status"`
	ChunkCount int            `json:"chunk_count"`
	ErrorMsg   string         `json:"error_msg,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}
