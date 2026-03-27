package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-template/models"
	"github.com/redis/go-redis/v9"
)

func CacheUser(user *models.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("user:%d", user.ID)
	return GetInstance().Set(context.Background(), key, data, 24*time.Hour).Err()
}

func GetCachedUser(id uint) (*models.User, error) {
	key := fmt.Sprintf("user:%d", id)
	val, err := GetInstance().Get(context.Background(), key).Result()
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var user models.User
	err = json.Unmarshal([]byte(val), &user)
	return &user, err
}

// Session Management for Stateful JWT
func SetSession(userID uint, jti string, expiration time.Duration) error {
	key := fmt.Sprintf("session:%d:%s", userID, jti)
	return GetInstance().Set(context.Background(), key, "valid", expiration).Err()
}

func IsSessionValid(userID uint, jti string) bool {
	key := fmt.Sprintf("session:%d:%s", userID, jti)
	val, err := GetInstance().Get(context.Background(), key).Result()
	return err == nil && val == "valid"
}

func RevokeSession(userID uint, jti string) error {
	key := fmt.Sprintf("session:%d:%s", userID, jti)
	return GetInstance().Del(context.Background(), key).Err()
}

func EvictUser(userID uint) error {
	pattern := fmt.Sprintf("session:%d:*", userID)
	iter := GetInstance().Scan(context.Background(), 0, pattern, 0).Iterator()
	for iter.Next(context.Background()) {
		if err := GetInstance().Del(context.Background(), iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}
