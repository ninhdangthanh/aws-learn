package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	Port           string        // HTTP port for the Gin server, e.g. ":8080"
	AWSRegion      string        // AWS region of the bucket, e.g. "ap-southeast-1"
	Bucket         string        // Target S3 bucket name
	KeyPrefix      string        // Prefix applied to every generated object key
	PartSize       int64         // Suggested part size (bytes) sent back to the client
	PresignExpiry  time.Duration // Lifetime of each presigned part URL
	AllowedOrigins []string      // CORS origins allowed to call the backend
}

// Load reads configuration from the environment, applying sensible defaults so
// the demo runs with a minimal .env.
func Load() *Config {
	return &Config{
		Port:           getenv("PORT", ":8080"),
		AWSRegion:      getenv("AWS_REGION", "ap-southeast-1"),
		Bucket:         getenv("S3_BUCKET", ""),
		KeyPrefix:      getenv("S3_KEY_PREFIX", "uploads/"),
		PartSize:       getenvInt64("PART_SIZE_BYTES", 10*1024*1024), // 10 MiB
		PresignExpiry:  time.Duration(getenvInt64("PRESIGN_EXPIRY_SECONDS", 900)) * time.Second,
		AllowedOrigins: []string{getenv("CORS_ORIGIN", "http://localhost:5173")},
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}
