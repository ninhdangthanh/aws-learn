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
	refreshKeyPrefix   = "refresh:"   // refresh:{jti} -> hash metadata
	blacklistKeyPrefix = "blacklist:" // blacklist:{jti} -> access token bị thu hồi sớm
	userSetKeyPrefix   = "user:"      // user:{id}:refresh -> set các jti của user
	userSetKeySuffix   = ":refresh"
	userVerKeySuffix   = ":ver" // user:{id}:ver -> cache token_version
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

func blacklistKey(jti string) string { return blacklistKeyPrefix + jti }

func userSetKey(userID string) string { return userSetKeyPrefix + userID + userSetKeySuffix }

func userVerKey(userID string) string { return userSetKeyPrefix + userID + userVerKeySuffix }

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

// DeleteAllRefreshByUser xoá toàn bộ refresh token của user (Logout All - Bài 9).
// Nhờ set user:{id}:refresh nên không cần SCAN toàn bộ keyspace của Redis.
// Trả về danh sách jti đã xoá để tầng trên đánh dấu revoked trong PostgreSQL.
func (s *TokenStore) DeleteAllRefreshByUser(ctx context.Context, userID string) ([]string, error) {
	setKey := userSetKey(userID)

	jtis, err := s.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, fmt.Errorf("repository: list refresh jtis: %w", err)
	}
	if len(jtis) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(jtis)+1)
	for _, jti := range jtis {
		keys = append(keys, refreshKey(jti))
	}
	keys = append(keys, setKey)

	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return nil, fmt.Errorf("repository: delete all refresh tokens: %w", err)
	}
	return jtis, nil
}

// ---- Blacklist access token (Bài 5) ----

// BlacklistAccess chặn một access token trước khi nó hết hạn tự nhiên.
// TTL đặt đúng bằng thời gian sống còn lại: token hết hạn thì key cũng biến mất,
// nên blacklist không bao giờ phình to vô hạn.
func (s *TokenStore) BlacklistAccess(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		// Token đã hết hạn rồi, chữ ký tự nó bị từ chối — không cần chặn thêm.
		return nil
	}
	if err := s.client.Set(ctx, blacklistKey(jti), "1", ttl).Err(); err != nil {
		return fmt.Errorf("repository: blacklist access token: %w", err)
	}
	return nil
}

// IsBlacklisted kiểm tra access token đã bị thu hồi hay chưa.
func (s *TokenStore) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	n, err := s.client.Exists(ctx, blacklistKey(jti)).Result()
	if err != nil {
		return false, fmt.Errorf("repository: check blacklist: %w", err)
	}
	return n > 0, nil
}

// ---- Cache token_version (Bài 4) ----

// ErrTokenVersionMiss nghĩa là cache chưa có, phải đọc từ PostgreSQL.
var ErrTokenVersionMiss = errors.New("repository: token version not cached")

// GetTokenVersion đọc token_version từ cache.
func (s *TokenStore) GetTokenVersion(ctx context.Context, userID string) (int, error) {
	v, err := s.client.Get(ctx, userVerKey(userID)).Int()
	if errors.Is(err, redis.Nil) {
		return 0, ErrTokenVersionMiss
	}
	if err != nil {
		return 0, fmt.Errorf("repository: get token version: %w", err)
	}
	return v, nil
}

// SetTokenVersion ghi token_version vào cache. Khi logout-all / đổi mật khẩu,
// tầng service gọi hàm này với giá trị mới nên mọi request kế tiếp bị chặn ngay,
// không phải chờ cache hết hạn.
func (s *TokenStore) SetTokenVersion(ctx context.Context, userID string, version int, ttl time.Duration) error {
	if err := s.client.Set(ctx, userVerKey(userID), version, ttl).Err(); err != nil {
		return fmt.Errorf("repository: set token version: %w", err)
	}
	return nil
}
