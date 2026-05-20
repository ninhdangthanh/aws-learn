package model

type ChatSession struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ChatMessage struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"session_id"`
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	Citations  []Citation     `json:"citations,omitempty"`
	TokenUsage map[string]int `json:"token_usage,omitempty"`
	LatencyMs  int            `json:"latency_ms,omitempty"`
	CreatedAt  string         `json:"created_at"`
}

type Citation struct {
	Filename   string `json:"filename"`
	PageNumber int    `json:"page_number"`
	Snippet    string `json:"snippet,omitempty"`
}
