package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Upload   UploadConfig
	Chunking ChunkingConfig
	Asynq    AsynqConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	Qdrant   QdrantConfig
	OpenAI   OpenAIConfig
}

type AppConfig struct {
	Name            string
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type UploadConfig struct {
	Dir              string
	MaxFileSizeBytes int64
}

type ChunkingConfig struct {
	ChunkSizeTokens    int
	ChunkOverlapTokens int
}

type AsynqConfig struct {
	QueueName   string
	Concurrency int
}

func (c AppConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type PostgresConfig struct {
	Host     string
	Port     int
	DB       string
	User     string
	Password string
	SSLMode  string
	DSN      string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type QdrantConfig struct {
	URL        string
	Host       string
	GRPCPort   int
	APIKey     string
	Collection string
}

type OpenAIConfig struct {
	APIKey              string
	EmbeddingModel      string
	EmbeddingDimensions int
	ChatModel           string
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("read .env: %w", err)
		}
	}

	cfg := Config{
		App: AppConfig{
			Name:            v.GetString("APP_NAME"),
			Host:            v.GetString("APP_HOST"),
			Port:            v.GetInt("APP_PORT"),
			ReadTimeout:     v.GetDuration("APP_READ_TIMEOUT"),
			WriteTimeout:    v.GetDuration("APP_WRITE_TIMEOUT"),
			ShutdownTimeout: v.GetDuration("APP_SHUTDOWN_TIMEOUT"),
		},
		Upload: UploadConfig{
			Dir:              v.GetString("UPLOAD_DIR"),
			MaxFileSizeBytes: v.GetInt64("UPLOAD_MAX_FILE_SIZE_BYTES"),
		},
		Chunking: ChunkingConfig{
			ChunkSizeTokens:    v.GetInt("CHUNK_SIZE"),
			ChunkOverlapTokens: v.GetInt("CHUNK_OVERLAP"),
		},
		Asynq: AsynqConfig{
			QueueName:   v.GetString("ASYNQ_QUEUE_NAME"),
			Concurrency: v.GetInt("ASYNQ_CONCURRENCY"),
		},
		Postgres: PostgresConfig{
			Host:     v.GetString("POSTGRES_HOST"),
			Port:     v.GetInt("POSTGRES_PORT"),
			DB:       v.GetString("POSTGRES_DB"),
			User:     v.GetString("POSTGRES_USER"),
			Password: v.GetString("POSTGRES_PASSWORD"),
			SSLMode:  v.GetString("POSTGRES_SSLMODE"),
			DSN:      v.GetString("POSTGRES_DSN"),
		},
		Redis: RedisConfig{
			Addr:     v.GetString("REDIS_ADDR"),
			Password: v.GetString("REDIS_PASSWORD"),
			DB:       v.GetInt("REDIS_DB"),
		},
		Qdrant: QdrantConfig{
			URL:        v.GetString("QDRANT_URL"),
			Host:       v.GetString("QDRANT_HOST"),
			GRPCPort:   v.GetInt("QDRANT_GRPC_PORT"),
			APIKey:     v.GetString("QDRANT_API_KEY"),
			Collection: v.GetString("QDRANT_COLLECTION"),
		},
		OpenAI: OpenAIConfig{
			APIKey:              v.GetString("OPENAI_API_KEY"),
			EmbeddingModel:      v.GetString("OPENAI_EMBEDDING_MODEL"),
			EmbeddingDimensions: v.GetInt("OPENAI_EMBEDDING_DIMENSIONS"),
			ChatModel:           v.GetString("OPENAI_CHAT_MODEL"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_NAME", "rag-chatbot-ai-backend")
	v.SetDefault("APP_HOST", "0.0.0.0")
	v.SetDefault("APP_PORT", 8099)
	v.SetDefault("APP_READ_TIMEOUT", "10s")
	v.SetDefault("APP_WRITE_TIMEOUT", "10s")
	v.SetDefault("APP_SHUTDOWN_TIMEOUT", "10s")

	v.SetDefault("UPLOAD_DIR", "storage/uploads")
	v.SetDefault("UPLOAD_MAX_FILE_SIZE_BYTES", 10485760)
	v.SetDefault("CHUNK_SIZE", 200)
	v.SetDefault("CHUNK_OVERLAP", 40)
	v.SetDefault("ASYNQ_QUEUE_NAME", "default")
	v.SetDefault("ASYNQ_CONCURRENCY", 10)

	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", 5432)
	v.SetDefault("POSTGRES_DB", "ragchat")
	v.SetDefault("POSTGRES_USER", "postgres")
	v.SetDefault("POSTGRES_PASSWORD", "postgres")
	v.SetDefault("POSTGRES_SSLMODE", "disable")
	v.SetDefault("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/ragchat?sslmode=disable")

	v.SetDefault("REDIS_ADDR", "localhost:6379")
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_DB", 0)

	v.SetDefault("QDRANT_URL", "http://localhost:6333")
	v.SetDefault("QDRANT_HOST", "localhost")
	v.SetDefault("QDRANT_GRPC_PORT", 6334)
	v.SetDefault("QDRANT_API_KEY", "")
	v.SetDefault("QDRANT_COLLECTION", "documents")

	v.SetDefault("OPENAI_API_KEY", "")
	v.SetDefault("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
	v.SetDefault("OPENAI_EMBEDDING_DIMENSIONS", 1536)
	v.SetDefault("OPENAI_CHAT_MODEL", "gpt-4.1-mini")
}

func (c Config) Validate() error {
	if c.App.Port <= 0 {
		return fmt.Errorf("APP_PORT must be greater than 0")
	}

	if c.App.Host == "" {
		return fmt.Errorf("APP_HOST is required")
	}

	if c.Redis.Addr == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}

	if c.Upload.Dir == "" {
		return fmt.Errorf("UPLOAD_DIR is required")
	}

	if c.Upload.MaxFileSizeBytes <= 0 {
		return fmt.Errorf("UPLOAD_MAX_FILE_SIZE_BYTES must be greater than 0")
	}

	if c.Chunking.ChunkSizeTokens <= 0 {
		return fmt.Errorf("CHUNK_SIZE must be greater than 0")
	}

	if c.Chunking.ChunkOverlapTokens < 0 {
		return fmt.Errorf("CHUNK_OVERLAP must be greater than or equal to 0")
	}

	if c.Chunking.ChunkOverlapTokens >= c.Chunking.ChunkSizeTokens {
		return fmt.Errorf("CHUNK_OVERLAP must be smaller than CHUNK_SIZE")
	}

	if c.Asynq.QueueName == "" {
		return fmt.Errorf("ASYNQ_QUEUE_NAME is required")
	}

	if c.Asynq.Concurrency <= 0 {
		return fmt.Errorf("ASYNQ_CONCURRENCY must be greater than 0")
	}

	if c.Qdrant.URL == "" {
		return fmt.Errorf("QDRANT_URL is required")
	}

	if c.Qdrant.Host == "" {
		return fmt.Errorf("QDRANT_HOST is required")
	}

	if c.Qdrant.GRPCPort <= 0 {
		return fmt.Errorf("QDRANT_GRPC_PORT must be greater than 0")
	}

	if c.Qdrant.Collection == "" {
		return fmt.Errorf("QDRANT_COLLECTION is required")
	}

	if c.OpenAI.EmbeddingModel == "" {
		return fmt.Errorf("OPENAI_EMBEDDING_MODEL is required")
	}

	if c.OpenAI.EmbeddingDimensions <= 0 {
		return fmt.Errorf("OPENAI_EMBEDDING_DIMENSIONS must be greater than 0")
	}

	return nil
}
