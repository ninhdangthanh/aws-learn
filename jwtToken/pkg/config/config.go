package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config gom toàn bộ cấu hình runtime, đọc từ biến môi trường (.env).
type Config struct {
	AppPort string
	AppEnv  string

	DBDsn string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	JWTSecret      string
	JWTIssuer      string
	AccessTokenTTL time.Duration
	RefreshTTL     time.Duration

	CORSOrigins []string
}

// Load đọc .env (nếu có) rồi build Config. Trả lỗi nếu thiếu config bắt buộc.
func Load() (*Config, error) {
	// .env là optional: khi chạy docker/CI thì env đã được inject sẵn.
	_ = godotenv.Load()

	cfg := &Config{
		AppPort:        env("APP_PORT", "8080"),
		AppEnv:         env("APP_ENV", "development"),
		DBDsn:          env("DB_DSN", ""),
		RedisAddr:      env("REDIS_ADDR", "localhost:6380"),
		RedisPassword:  env("REDIS_PASSWORD", ""),
		RedisDB:        envInt("REDIS_DB", 0),
		JWTSecret:      env("JWT_SECRET", ""),
		JWTIssuer:      env("JWT_ISSUER", "jwt-auth"),
		AccessTokenTTL: envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTTL:     envDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		CORSOrigins:    strings.Split(env("CORS_ORIGINS", "http://localhost:5173"), ","),
	}

	if cfg.DBDsn == "" {
		return nil, fmt.Errorf("config: DB_DSN is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("config: JWT_SECRET is required and must be at least 32 chars")
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(env(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(env(key, ""))
	if err != nil {
		return fallback
	}
	return d
}
