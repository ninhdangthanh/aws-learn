package redis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-template/models"
	"github.com/redis/go-redis/v9"
)

// CacheOrder caching logic
func CacheOrder(order *models.Order) error {
	data, err := json.Marshal(order)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("order:%d", order.ID)
	// Cache for 1 hour
	return GetInstance().Set(ctx, key, data, 1*time.Hour).Err()
}

// GetCachedOrder retrieves logic
func GetCachedOrder(id uint) (*models.Order, error) {
	key := fmt.Sprintf("order:%d", id)
	val, err := GetInstance().Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // cache miss
	} else if err != nil {
		return nil, err
	}

	var order models.Order
	err = json.Unmarshal([]byte(val), &order)
	return &order, err
}
