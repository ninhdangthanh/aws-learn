package config

import (
	"os"
)

type Config struct {
	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisHost string
	RedisPort string

	// Kafka
	KafkaBrokers []string

	// Server
	ServerPort string
}

func Load() *Config {
	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "kafka_user"),
		DBPassword: getEnv("DB_PASSWORD", "kafka_pass"),
		DBName:     getEnv("DB_NAME", "commerce_db"),
		RedisHost:  getEnv("REDIS_HOST", "localhost"),
		RedisPort:  getEnv("REDIS_PORT", "6379"),
		KafkaBrokers: []string{
			getEnv("KAFKA_BROKER", "localhost:9092"),
		},
		ServerPort: getEnv("SERVER_PORT", "8000"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
