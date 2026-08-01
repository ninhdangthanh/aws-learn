package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// CatalogCache là lớp duy nhất chạm tới Redis. Redis ở đây không phải cache lazy
// mà là read model: mỗi client có đúng một key chứa toàn bộ catalog đang bán.
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

// Get phân biệt rõ ba trạng thái, vì read path đối xử với chúng khác nhau:
//
//	hit = true             -> trả thẳng
//	hit = false, err = nil -> miss, caller rebuild từ Postgres
//	err != nil             -> Redis down, caller trả 503
//
// Payload hỏng được coi như miss: xoá key rồi để rebuild dựng lại.
func (c *CatalogCache) Get(ctx context.Context, clientID string) (Catalog, bool, error) {
	key := c.Key(clientID)

	cached, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return Catalog{}, false, nil
	}
	if err != nil {
		return Catalog{}, false, err
	}

	var catalog Catalog
	if err := json.Unmarshal(cached, &catalog); err != nil {
		log.Println("redis payload corrupted, dropping key:", key)
		c.Del(ctx, clientID)
		return Catalog{}, false, nil
	}
	return catalog, true, nil
}

func (c *CatalogCache) Set(ctx context.Context, clientID string, catalog Catalog) error {
	body, err := json.Marshal(catalog)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.Key(clientID), body, c.ttl).Err()
}

func (c *CatalogCache) Del(ctx context.Context, clientID string) error {
	return c.rdb.Del(ctx, c.Key(clientID)).Err()
}
