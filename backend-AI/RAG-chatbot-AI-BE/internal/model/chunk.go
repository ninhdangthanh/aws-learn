package model

type Chunk struct {
	ID         string `json:"id"`
	DocumentID string `json:"document_id"`
	ChunkIndex int    `json:"chunk_index"`
	Content    string `json:"content"`
	PageNumber int    `json:"page_number"`
	TokenCount int    `json:"token_count"`
	QdrantID   string `json:"qdrant_id,omitempty"`
}

type SearchResult struct {
	DocumentID string  `json:"document_id"`
	Filename   string  `json:"filename"`
	PageNumber int     `json:"page_number"`
	ChunkIndex int     `json:"chunk_index"`
	Score      float64 `json:"score"`
	Text       string  `json:"text"`
}
