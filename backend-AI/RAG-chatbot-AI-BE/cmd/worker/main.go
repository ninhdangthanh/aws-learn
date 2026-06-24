package main

import (
	"log"

	"github.com/ninhdangthanh/rag-chatbot-ai-be/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	log.Printf(
		"worker bootstrap ready (redis=%s, postgres=%s:%d, qdrant=%s)",
		cfg.Redis.Addr,
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Qdrant.URL,
	)
}
