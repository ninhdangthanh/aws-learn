package tasks

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	TypeDocumentParse = "document:parse"
	TypeDocumentEmbed = "document:embed"
)

type DocumentParsePayload struct {
	DocumentID string `json:"document_id"`
	FilePath   string `json:"file_path"`
}

type DocumentEmbedPayload struct {
	DocumentID string `json:"document_id"`
}

func NewDocumentParseTask(documentID uuid.UUID, filePath string) (*asynq.Task, error) {
	payload, err := json.Marshal(DocumentParsePayload{
		DocumentID: documentID.String(),
		FilePath:   filePath,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal parse payload: %w", err)
	}

	return asynq.NewTask(TypeDocumentParse, payload), nil
}

func NewDocumentEmbedTask(documentID uuid.UUID) (*asynq.Task, error) {
	payload, err := json.Marshal(DocumentEmbedPayload{
		DocumentID: documentID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embed payload: %w", err)
	}

	return asynq.NewTask(TypeDocumentEmbed, payload), nil
}

func ParseDocumentID(value string) (uuid.UUID, error) {
	return uuid.Parse(value)
}

func EnqueueOptions(queueName string) []asynq.Option {
	return []asynq.Option{
		asynq.Queue(queueName),
		asynq.Timeout(5 * time.Minute),
		asynq.MaxRetry(10),
	}
}
