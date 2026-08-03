package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// CatalogCache là lớp duy nhất chạm tới Redis (cache-aside cho catalog của client).
//
// Cache ở đây là optional: mọi lỗi Redis chỉ được log chứ không đẩy ngược lên
// handler. Cache hỏng phải làm chậm, không được làm sai — đường lui luôn là Postgres.
type CatalogCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewCatalogCache(addr string, ttl time.Duration) (*CatalogCache, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		rdb.Close()
		return nil, err
	}
	return &CatalogCache{rdb: rdb, ttl: ttl}, nil
}

func (c *CatalogCache) Close() error { return c.rdb.Close() }

func (c *CatalogCache) TTL() time.Duration { return c.ttl }

func (c *CatalogCache) Key(clientID string) string {
	return "client:" + clientID + ":catalog"
}

// Get trả (catalog, true) khi cache hit. Redis down hoặc payload hỏng đều coi như
// miss — caller cứ đọc Postgres như bình thường.
func (c *CatalogCache) Get(ctx context.Context, clientID string) (Catalog, bool) {
	key := c.Key(clientID)

	cached, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			log.Println("redis get:", err)
		}
		return Catalog{}, false
	}

	var catalog Catalog
	if err := json.Unmarshal(cached, &catalog); err != nil {
		log.Println("redis payload corrupted, dropping key:", key)
		c.Del(ctx, clientID)
		return Catalog{}, false
	}
	return catalog, true
}

func (c *CatalogCache) Set(ctx context.Context, clientID string, catalog Catalog) {
	body, err := json.Marshal(catalog)
	if err != nil {
		log.Println("cache encode:", err)
		return
	}
	if err := c.rdb.Set(ctx, c.Key(clientID), body, c.ttl).Err(); err != nil {
		log.Println("redis set:", err)
	}
}

func (c *CatalogCache) Del(ctx context.Context, clientID string) {
	if err := c.rdb.Del(ctx, c.Key(clientID)).Err(); err != nil {
		log.Println("redis del:", err)
	}
}
