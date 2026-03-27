package redis

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	rdbInstance *redis.Client
	rdbOnce     sync.Once
	ctx         = context.Background()
)

// InitRedis initializes the singleton redis client instance
func InitRedis(addr string, password string, db int) {
	rdbOnce.Do(func() {
		var err error
		for i := 0; i < 5; i++ {
			rdbInstance = redis.NewClient(&redis.Options{
				Addr:     addr,
				Password: password,
				DB:       db,
			})

			if _, err = rdbInstance.Ping(ctx).Result(); err == nil {
				log.Println("Connected to Redis successfully")
				return
			}
			log.Printf("Waiting for Redis (attempt %d/5)... error: %v", i+1, err)
			time.Sleep(5 * time.Second)
		}
		log.Fatalf("Could not connect to Redis after 5 attempts: %v", err)
	})
}

// GetInstance retrieves the singleton redis client connection
func GetInstance() *redis.Client {
	if rdbInstance == nil {
		log.Println("Redis instance is nil. Did you call InitRedis?")
	}
	return rdbInstance
}
