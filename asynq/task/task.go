package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueLow      = "low"
)

const (
	TypeEmailDelivery = "email:deliver"
	TypeImageResize   = "image:resize"
)

type EmailDeliveryPayload struct {
	UserID      int    `json:"user_id"`
	Email       string `json:"email"`
	TemplateID  string `json:"template_id"`
	Subject     string `json:"subject"`
	RequestedBy string `json:"requested_by"`
}

type ImageResizePayload struct {
	SourceURL string `json:"source_url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Format    string `json:"format"`
	OwnerID   int    `json:"owner_id"`
}

func NewEmailDeliveryTask(p EmailDeliveryPayload) (*asynq.Task, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeEmailDelivery, payload), nil
}

func NewImageResizeTask(p ImageResizePayload) (*asynq.Task, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeImageResize, payload, asynq.MaxRetry(5), asynq.Timeout(20*time.Minute)), nil
}

func HandleEmailDeliveryTask(ctx context.Context, t *asynq.Task) error {
	_ = ctx

	var p EmailDeliveryPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("could not decode email payload: %v: %w", err, asynq.SkipRetry)
	}
	if err := p.Validate(); err != nil {
		return fmt.Errorf("invalid email payload: %v: %w", err, asynq.SkipRetry)
	}

	log.Printf(
		"send email: user_id=%d email=%s template=%s subject=%q requested_by=%s",
		p.UserID,
		p.Email,
		p.TemplateID,
		p.Subject,
		p.RequestedBy,
	)
	return nil
}

type ImageProcessorConfig struct {
	OutputDir string
}

type ImageProcessor struct {
	config ImageProcessorConfig
}

func (processor *ImageProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	_ = ctx

	var p ImageResizePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("could not decode image payload: %v: %w", err, asynq.SkipRetry)
	}
	if err := p.Validate(); err != nil {
		return fmt.Errorf("invalid image payload: %v: %w", err, asynq.SkipRetry)
	}

	log.Printf(
		"resize image: src=%s owner_id=%d size=%dx%d format=%s output_dir=%s",
		p.SourceURL,
		p.OwnerID,
		p.Width,
		p.Height,
		p.Format,
		processor.config.OutputDir,
	)
	return nil
}

func NewImageProcessor(config ImageProcessorConfig) *ImageProcessor {
	if config.OutputDir == "" {
		config.OutputDir = "./tmp/resized"
	}
	return &ImageProcessor{config: config}
}

func (p EmailDeliveryPayload) Validate() error {
	if p.UserID <= 0 {
		return errors.New("user_id must be greater than 0")
	}
	if strings.TrimSpace(p.Email) == "" {
		return errors.New("email is required")
	}
	if strings.TrimSpace(p.TemplateID) == "" {
		return errors.New("template_id is required")
	}
	if strings.TrimSpace(p.Subject) == "" {
		return errors.New("subject is required")
	}
	return nil
}

func (p ImageResizePayload) Validate() error {
	if strings.TrimSpace(p.SourceURL) == "" {
		return errors.New("source_url is required")
	}
	if p.Width <= 0 || p.Height <= 0 {
		return errors.New("width and height must be greater than 0")
	}
	switch strings.ToLower(strings.TrimSpace(p.Format)) {
	case "jpg", "jpeg", "png", "webp":
		return nil
	default:
		return fmt.Errorf("format %q is not supported", p.Format)
	}
}
