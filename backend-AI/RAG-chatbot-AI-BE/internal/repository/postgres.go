package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninhdangthanh/rag-chatbot/internal/model"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close(ctx context.Context) {
	p.pool.Close()
}

func (p *Postgres) CreateDocument(ctx context.Context, filename string, size int64, fileType string) (*model.Document, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := p.pool.Exec(ctx, `INSERT INTO documents (id, filename, file_size, file_type, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`, id, filename, size, fileType, model.StatusPending, now, now)
	if err != nil {
		return nil, err
	}
	return &model.Document{ID: id, Filename: filename, FileSize: size, FileType: fileType, Status: model.StatusPending, CreatedAt: now, UpdatedAt: now}, nil
}

func (p *Postgres) UpdateDocumentStatus(ctx context.Context, id string, status model.DocumentStatus, errMsg string) error {
	query := `UPDATE documents SET status=$1, error_msg=$2, updated_at=$3 WHERE id=$4`
	_, err := p.pool.Exec(ctx, query, status, sql.NullString{String: errMsg, Valid: errMsg != ""}, time.Now().UTC(), id)
	return err
}

func (p *Postgres) SetDocumentChunkCount(ctx context.Context, id string, chunkCount int) error {
	_, err := p.pool.Exec(ctx, `UPDATE documents SET chunk_count=$1, updated_at=$2 WHERE id=$3`, chunkCount, time.Now().UTC(), id)
	return err
}

func (p *Postgres) GetDocumentByID(ctx context.Context, id string) (*model.Document, error) {
	doc := &model.Document{}
	row := p.pool.QueryRow(ctx, `SELECT id, filename, file_size, file_type, status, chunk_count, error_msg, created_at, updated_at FROM documents WHERE id=$1`, id)
	var errMsg sql.NullString
	if err := row.Scan(&doc.ID, &doc.Filename, &doc.FileSize, &doc.FileType, &doc.Status, &doc.ChunkCount, &errMsg, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if errMsg.Valid {
		doc.ErrorMsg = errMsg.String
	}
	return doc, nil
}

func (p *Postgres) ListDocuments(ctx context.Context, status string, limit, offset int) ([]*model.Document, error) {
	builder := strings.Builder{}
	builder.WriteString(`SELECT id, filename, file_size, file_type, status, chunk_count, error_msg, created_at, updated_at FROM documents`)
	args := []interface{}{}
	if status != "" {
		builder.WriteString(` WHERE status=$1`)
		args = append(args, status)
	}
	builder.WriteString(` ORDER BY created_at DESC LIMIT $2 OFFSET $3`)
	args = append(args, limit, offset)

	rows, err := p.pool.Query(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*model.Document
	for rows.Next() {
		doc := &model.Document{}
		var errMsg sql.NullString
		if err := rows.Scan(&doc.ID, &doc.Filename, &doc.FileSize, &doc.FileType, &doc.Status, &doc.ChunkCount, &errMsg, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, err
		}
		if errMsg.Valid {
			doc.ErrorMsg = errMsg.String
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (p *Postgres) DeleteDocument(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM documents WHERE id=$1`, id)
	return err
}

func (p *Postgres) CreateChunks(ctx context.Context, chunks []model.Chunk) error {
	batch := &pgx.Batch{}
	for _, c := range chunks {
		batch.Queue(`INSERT INTO chunks (id, document_id, chunk_index, content, page_number, token_count, qdrant_id, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, c.ID, c.DocumentID, c.ChunkIndex, c.Content, c.PageNumber, c.TokenCount, c.QdrantID, time.Now().UTC())
	}
	br := p.pool.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < len(chunks); i++ {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Postgres) FindChunksToEmbed(ctx context.Context, documentID string) ([]model.Chunk, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, document_id, chunk_index, content, page_number, token_count FROM chunks WHERE document_id=$1 AND qdrant_id IS NULL ORDER BY chunk_index`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.Chunk
	for rows.Next() {
		c := model.Chunk{}
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.ChunkIndex, &c.Content, &c.PageNumber, &c.TokenCount); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}

func (p *Postgres) UpdateChunkQdrantID(ctx context.Context, chunkID, qdrantID string) error {
	_, err := p.pool.Exec(ctx, `UPDATE chunks SET qdrant_id=$1 WHERE id=$2`, qdrantID, chunkID)
	return err
}

func (p *Postgres) GetSearchChunks(ctx context.Context, ids []string) (map[string]model.Chunk, error) {
	if len(ids) == 0 {
		return map[string]model.Chunk{}, nil
	}
	params := []string{}
	args := []interface{}{}
	for i, id := range ids {
		params = append(params, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}
	query := fmt.Sprintf(`SELECT id, document_id, chunk_index, content, page_number FROM chunks WHERE qdrant_id IN (%s)`, strings.Join(params, ","))
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]model.Chunk{}
	for rows.Next() {
		c := model.Chunk{}
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.ChunkIndex, &c.Content, &c.PageNumber); err != nil {
			return nil, err
		}
		result[c.ID] = c
	}
	return result, nil
}

func (p *Postgres) CreateChatSession(ctx context.Context, title string) (string, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := p.pool.Exec(ctx, `INSERT INTO chat_sessions (id, title, created_at, updated_at) VALUES ($1, $2, $3, $4)`, id, title, now, now)
	return id, err
}

func (p *Postgres) CreateChatMessage(ctx context.Context, message model.ChatMessage) error {
	citationsJSON, err := json.Marshal(message.Citations)
	if err != nil {
		return err
	}
	tokenUsageJSON, err := json.Marshal(message.TokenUsage)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `INSERT INTO chat_messages (id, session_id, role, content, citations, token_usage, latency_ms, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, message.ID, message.SessionID, message.Role, message.Content, citationsJSON, tokenUsageJSON, message.LatencyMs, time.Now().UTC())
	return err
}

func (p *Postgres) GetChatSessionMessages(ctx context.Context, sessionID string) ([]model.ChatMessage, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, session_id, role, content, citations, token_usage, latency_ms, created_at FROM chat_messages WHERE session_id=$1 ORDER BY created_at`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []model.ChatMessage
	for rows.Next() {
		var citationsData []byte
		var tokenUsageData []byte
		msg := model.ChatMessage{}
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &citationsData, &tokenUsageData, &msg.LatencyMs, &msg.CreatedAt); err != nil {
			return nil, err
		}
		if len(citationsData) > 0 {
			_ = json.Unmarshal(citationsData, &msg.Citations)
		}
		if len(tokenUsageData) > 0 {
			_ = json.Unmarshal(tokenUsageData, &msg.TokenUsage)
		}
		messages = append(messages, msg)
	}
	return messages, nil
}
