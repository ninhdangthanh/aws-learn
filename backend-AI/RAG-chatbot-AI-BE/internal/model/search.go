package model

type SearchRequest struct {
	Query          string  `json:"query" binding:"required"`
	TopK           int     `json:"top_k"`
	ScoreThreshold float64 `json:"score_threshold,omitempty"`
}

type SearchResponse struct {
	Query     string         `json:"query"`
	Results   []SearchResult `json:"results"`
	LatencyMs int64          `json:"latency_ms"`
}

type ChatRequest struct {
	Question  string `json:"question" binding:"required"`
	SessionID string `json:"session_id,omitempty"`
	TopK      int    `json:"top_k,omitempty"`
	Stream    bool   `json:"stream,omitempty"`
}

type ChatResponse struct {
	Answer     string         `json:"answer,omitempty"`
	Citations  []Citation     `json:"citations,omitempty"`
	SessionID  string         `json:"session_id,omitempty"`
	TokenUsage map[string]int `json:"token_usage,omitempty"`
	LatencyMs  int64          `json:"latency_ms,omitempty"`
}
