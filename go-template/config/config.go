package config

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DB_DSN            string `envconfig:"DB_DSN" default:"host=localhost user=gorm password=gorm dbname=gorm port=5920 sslmode=disable TimeZone=Asia/Shanghai"`
	RedisAddr         string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	RedisPassword     string `envconfig:"REDIS_PASSWORD" default:""`
	RedisDB           int    `envconfig:"REDIS_DB" default:"0"`
	ElasticSearchURL  string `envconfig:"ELASTICSEARCH_URL" default:"http://localhost:9200"`
	RabbitMQURL       string `envconfig:"RABBITMQ_URL" default:"amqp://guest:guest@localhost:5672/"`
	GRPCPort          string `envconfig:"GRPC_PORT" default:":50051"`
	GinPort           string `envconfig:"GIN_PORT" default:":8080"`
	JWTSecret         string `envconfig:"JWT_SECRET" default:"your-secret-key"`
	JWTExpirationHours int   `envconfig:"JWT_EXPIRATION_HOURS" default:"1"` // Short-lived access
	RefreshExpirationDays int `envconfig:"REFRESH_EXPIRATION_DAYS" default:"7"` // Longer-lived refresh
}

func LoadConfig() (*Config, error) {
	// Attempt to load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables defaults")
	}

	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
