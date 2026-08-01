package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// ProductCache là lớp duy nhất chạm tới Redis (cache-aside cho list products).
type ProductCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewProductCache(addr string, ttl time.Duration) (*ProductCache, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		rdb.Close()
		return nil, err
	}
	return &ProductCache{rdb: rdb, ttl: ttl}, nil
}

func (c *ProductCache) Close() error {
	return c.rdb.Close()
}

func (c *ProductCache) TTL() time.Duration {
	return c.ttl
}

func (c *ProductCache) Key(clientID string) string {
	return "client:" + clientID + ":products"
}

// Get trả về (products, true) khi cache hit. Cache hỏng thì tự xoá và coi như miss.
func (c *ProductCache) Get(ctx context.Context, clientID string) ([]Product, bool) {
	key := c.Key(clientID)

	cached, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			log.Println("redis get:", err)
		}
		return nil, false
	}

	var products []Product
	if err := json.Unmarshal(cached, &products); err != nil {
		log.Println("redis payload corrupted, dropping key:", key)
		c.Del(ctx, clientID)
		return nil, false
	}
	return products, true
}

func (c *ProductCache) Set(ctx context.Context, clientID string, products []Product) {
	body, err := json.Marshal(products)
	if err != nil {
		log.Println("cache encode:", err)
		return
	}
	if err := c.rdb.Set(ctx, c.Key(clientID), body, c.ttl).Err(); err != nil {
		log.Println("redis set:", err)
	}
}

func (c *ProductCache) Del(ctx context.Context, clientID string) {
	if err := c.rdb.Del(ctx, c.Key(clientID)).Err(); err != nil {
		log.Println("redis del:", err)
	}
}
