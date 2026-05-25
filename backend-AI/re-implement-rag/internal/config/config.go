package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Port                 int
	OpenAIAPIKey         string
	OpenAIEmbeddingModel string
	OpenAILLMModel       string
	PostgresDSN          string
	RedisAddr            string
	QdrantAddr           string
	QdrantCollection     string
	ChunkSize            int
	ChunkOverlap         int
	SearchTopK           int
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config: %w", err)
	}

	viper.SetDefault("PORT", 8080)
	viper.SetDefault("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
	viper.SetDefault("OPENAI_LLM_MODEL", "gpt-4.1-mini")
	viper.SetDefault("REDIS_ADDR", "localhost:6379")
	viper.SetDefault("QDRANT_ADDR", "localhost:6334")
	viper.SetDefault("QDRANT_COLLECTION", "documents")
	viper.SetDefault("CHUNK_SIZE", 500)
	viper.SetDefault("CHUNK_OVERLAP", 100)
	viper.SetDefault("SEARCH_TOP_K", 5)

	cfg := &Config{
		Port:                 viper.GetInt("PORT"),
		OpenAIAPIKey:         viper.GetString("OPENAI_API_KEY"),
		OpenAIEmbeddingModel: viper.GetString("OPENAI_EMBEDDING_MODEL"),
		OpenAILLMModel:       viper.GetString("OPENAI_LLM_MODEL"),
		PostgresDSN:          viper.GetString("POSTGRES_DSN"),
		RedisAddr:            viper.GetString("REDIS_ADDR"),
		QdrantAddr:           viper.GetString("QDRANT_ADDR"),
		QdrantCollection:     viper.GetString("QDRANT_COLLECTION"),
		ChunkSize:            viper.GetInt("CHUNK_SIZE"),
		ChunkOverlap:         viper.GetInt("CHUNK_OVERLAP"),
		SearchTopK:           viper.GetInt("SEARCH_TOP_K"),
	}

	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if cfg.PostgresDSN == "" {
		return nil, fmt.Errorf("POSTGRES_DSN is required")
	}
	return cfg, nil
}
