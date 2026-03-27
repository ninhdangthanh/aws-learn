package main

import (
	"log"

	"github.com/go-template/api"
	"github.com/go-template/config"
	"github.com/go-template/database"
	"github.com/go-template/elastic"
	"github.com/go-template/grpc/server"
	"github.com/go-template/messaging"
	"github.com/go-template/redis"
)

func main() {
	// Load configuration struct
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Postgres (Gorm)
	log.Println("Initializing Postgres singleton...")
	database.InitDB(cfg.DB_DSN)

	// Initialize Redis
	log.Println("Initializing Redis singleton...")
	redis.InitRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)

	// Initialize Elastic Search
	log.Println("Initializing Elastic Search singleton...")
	elastic.InitElasticSearch(cfg.ElasticSearchURL)

	// Initialize RabbitMQ
	log.Println("Initializing RabbitMQ singleton...")
	messaging.InitRabbitMQ(cfg.RabbitMQURL)
	defer messaging.CloseRabbitMQ()

	// Start gRPC Server in a goroutine
	go func() {
		server.StartGRPCServer(cfg.GRPCPort)
	}()

	// Start Gin HTTP Server
	log.Printf("Starting Gin server on %s...", cfg.GinPort)
	r := api.SetupRouter(cfg)
	if err := r.Run(cfg.GinPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
