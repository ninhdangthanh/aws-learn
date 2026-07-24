package main

import (
	"context"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"multipart-s3-upload/internal/config"
	"multipart-s3-upload/internal/handler"
	"multipart-s3-upload/internal/s3svc"
)

func main() {
	// .env is optional — real deployments use real env vars / IAM roles.
	_ = godotenv.Load()

	cfg := config.Load()
	if cfg.Bucket == "" {
		log.Fatal("S3_BUCKET is required")
	}

	svc, err := s3svc.New(context.Background(), cfg.AWSRegion, cfg.Bucket, cfg.PresignExpiry)
	if err != nil {
		log.Fatalf("init s3: %v", err)
	}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Content-Type"},
	}))

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	h := handler.New(svc, cfg.KeyPrefix, cfg.PartSize)
	h.Register(r.Group("/"))

	log.Printf("listening on %s (bucket=%s region=%s)", cfg.Port, cfg.Bucket, cfg.AWSRegion)
	if err := r.Run(cfg.Port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
