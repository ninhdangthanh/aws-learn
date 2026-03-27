package redis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-template/models"
	"github.com/redis/go-redis/v9"
)

func CacheProduct(product *models.Product) error {
	data, err := json.Marshal(product)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("product:%d", product.ID)
	return GetInstance().Set(ctx, key, data, 12*time.Hour).Err()
}

func GetCachedProduct(id uint) (*models.Product, error) {
	key := fmt.Sprintf("product:%d", id)
	val, err := GetInstance().Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var product models.Product
	err = json.Unmarshal([]byte(val), &product)
	return &product, err
}
