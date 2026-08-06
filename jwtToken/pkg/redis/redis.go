package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client alias để tầng repository không import trực tiếp go-redis.
type Client = redis.Client

// Nil được re-export để caller phân biệt "key không tồn tại" với lỗi thật.
const Nil = redis.Nil

// New tạo Redis client và ping để fail-fast lúc khởi động.
func New(addr, pwd string, db int) (*Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pwd,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping %s: %w", addr, err)
	}

	return client, nil
}
