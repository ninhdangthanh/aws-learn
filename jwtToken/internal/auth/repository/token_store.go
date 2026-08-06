package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ninhdang/jwt-auth/pkg/redis"
)

// ErrRefreshNotFound nghĩa là refresh token không còn trong Redis: đã logout,
// đã hết TTL, hoặc bị thu hồi. Chữ ký JWT vẫn hợp lệ nhưng token coi như chết.
var ErrRefreshNotFound = errors.New("repository: refresh token not found")

const (
	refreshKeyPrefix = "refresh:" // refresh:{jti} -> hash metadata
	userSetKeyPrefix = "user:"    // user:{id}:refresh -> set các jti của user
	userSetKeySuffix = ":refresh"
)

// RefreshSession là payload lưu kèm mỗi jti trong Redis.
type RefreshSession struct {
	UserID    string
	DeviceID  string
	UserAgent string
	IP        string
	IssuedAt  time.Time
}

// TokenStore quản lý vòng đời refresh token trong Redis.
// Redis là nguồn sự thật: còn key = token còn hiệu lực.
type TokenStore struct {
	client *redis.Client
}

func NewTokenStore(client *redis.Client) *TokenStore {
	return &TokenStore{client: client}
}

func refreshKey(jti string) string { return refreshKeyPrefix + jti }

func userSetKey(userID string) string { return userSetKeyPrefix + userID + userSetKeySuffix }

// SaveRefresh ghi refresh:{jti} với TTL đúng bằng hạn của refresh token,
// nên Redis tự dọn rác khi token hết hạn.
func (s *TokenStore) SaveRefresh(ctx context.Context, jti string, sess RefreshSession, ttl time.Duration) error {
	key := refreshKey(jti)

	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"user_id":    sess.UserID,
		"device_id":  sess.DeviceID,
		"user_agent": sess.UserAgent,
		"ip":         sess.IP,
		"issued_at":  sess.IssuedAt.UTC().Format(time.RFC3339),
	})
	pipe.Expire(ctx, key, ttl)

	// Set phụ để sau này logout-all / liệt kê thiết bị không phải SCAN toàn bộ Redis.
	pipe.SAdd(ctx, userSetKey(sess.UserID), jti)
	pipe.Expire(ctx, userSetKey(sess.UserID), ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("repository: save refresh token: %w", err)
	}
	return nil
}

// GetRefresh trả về session gắn với jti, hoặc ErrRefreshNotFound nếu key đã biến mất.
func (s *TokenStore) GetRefresh(ctx context.Context, jti string) (*RefreshSession, error) {
	data, err := s.client.HGetAll(ctx, refreshKey(jti)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("repository: get refresh token: %w", err)
	}
	if len(data) == 0 {
		return nil, ErrRefreshNotFound
	}

	issuedAt, _ := time.Parse(time.RFC3339, data["issued_at"])
	return &RefreshSession{
		UserID:    data["user_id"],
		DeviceID:  data["device_id"],
		UserAgent: data["user_agent"],
		IP:        data["ip"],
		IssuedAt:  issuedAt,
	}, nil
}

// DeleteRefresh xoá một refresh token (logout một thiết bị).
func (s *TokenStore) DeleteRefresh(ctx context.Context, jti, userID string) error {
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, refreshKey(jti))
	pipe.SRem(ctx, userSetKey(userID), jti)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("repository: delete refresh token: %w", err)
	}
	return nil
}
