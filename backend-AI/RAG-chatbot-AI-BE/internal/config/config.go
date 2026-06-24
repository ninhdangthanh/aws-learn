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
	Postgres PostgresConfig
	Redis    RedisConfig
	Qdrant   QdrantConfig
	OpenAI   OpenAIConfig
}

type AppConfig struct {
	Name            string
	Env             string
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
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
	APIKey     string
	Collection string
}

type OpenAIConfig struct {
	APIKey         string
	EmbeddingModel string
	ChatModel      string
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
			Env:             v.GetString("APP_ENV"),
			Host:            v.GetString("APP_HOST"),
			Port:            v.GetInt("APP_PORT"),
			ReadTimeout:     v.GetDuration("APP_READ_TIMEOUT"),
			WriteTimeout:    v.GetDuration("APP_WRITE_TIMEOUT"),
			ShutdownTimeout: v.GetDuration("APP_SHUTDOWN_TIMEOUT"),
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
			APIKey:     v.GetString("QDRANT_API_KEY"),
			Collection: v.GetString("QDRANT_COLLECTION"),
		},
		OpenAI: OpenAIConfig{
			APIKey:         v.GetString("OPENAI_API_KEY"),
			EmbeddingModel: v.GetString("OPENAI_EMBEDDING_MODEL"),
			ChatModel:      v.GetString("OPENAI_CHAT_MODEL"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_NAME", "rag-chatbot-ai-backend")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_HOST", "0.0.0.0")
	v.SetDefault("APP_PORT", 8099)
	v.SetDefault("APP_READ_TIMEOUT", "10s")
	v.SetDefault("APP_WRITE_TIMEOUT", "10s")
	v.SetDefault("APP_SHUTDOWN_TIMEOUT", "10s")

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
	v.SetDefault("QDRANT_API_KEY", "")
	v.SetDefault("QDRANT_COLLECTION", "documents")

	v.SetDefault("OPENAI_API_KEY", "")
	v.SetDefault("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
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

	if c.Qdrant.URL == "" {
		return fmt.Errorf("QDRANT_URL is required")
	}

	return nil
}
