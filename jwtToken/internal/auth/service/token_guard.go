package service

import (
	"context"
	"errors"
	"time"

	"github.com/ninhdang/jwt-auth/internal/auth/repository"
	"github.com/ninhdang/jwt-auth/pkg/jwt"
)

var (
	// ErrTokenRevoked: access token nằm trong blacklist (Bài 5) — đã logout.
	ErrTokenRevoked = errors.New("service: access token revoked")
	// ErrTokenVersionMismatch: token mang ver cũ (Bài 4) — user đã logout-all
	// hoặc vừa đổi mật khẩu.
	ErrTokenVersionMismatch = errors.New("service: token version mismatch")
)

// tokenVersionCacheTTL: cache ngắn để giảm tải DB mà vẫn tự lành nếu có sai lệch.
// Không phụ thuộc vào TTL này để thu hồi kịp thời — mọi thao tác đổi
// token_version đều xoá cache ngay lập tức.
const tokenVersionCacheTTL = 10 * time.Minute

// TokenGuard là phần kiểm tra *stateful* chạy sau khi chữ ký JWT đã hợp lệ.
//
// Nó là cái giá phải trả cho khả năng thu hồi token: Bài 1 nói middleware không
// cần chạm DB, nhưng muốn logout/logout-all có hiệu lực tức thì thì phải tra
// một nguồn state nào đó. Ở đây state nằm ở Redis (1 lần GET blacklist +
// 1 lần GET version), PostgreSQL chỉ bị đụng tới khi cache miss.
type TokenGuard struct {
	users  *repository.UserRepository
	tokens *repository.TokenStore
}

func NewTokenGuard(users *repository.UserRepository, tokens *repository.TokenStore) *TokenGuard {
	return &TokenGuard{users: users, tokens: tokens}
}

// Authorize chạy hai chốt chặn của Phase 2 trên một access token đã verify chữ ký.
func (g *TokenGuard) Authorize(ctx context.Context, claims *jwt.Claims) error {
	// Bài 5: token này có bị thu hồi riêng lẻ không?
	blacklisted, err := g.tokens.IsBlacklisted(ctx, claims.ID)
	if err != nil {
		return err
	}
	if blacklisted {
		return ErrTokenRevoked
	}

	// Bài 4: toàn bộ token của user có bị vô hiệu hoá hàng loạt không?
	currentVersion, err := g.tokenVersion(ctx, claims.Subject)
	if err != nil {
		return err
	}
	if claims.TokenVersion != currentVersion {
		return ErrTokenVersionMismatch
	}

	return nil
}

// tokenVersion đọc token_version từ Redis, fallback về PostgreSQL khi cache miss.
func (g *TokenGuard) tokenVersion(ctx context.Context, userID string) (int, error) {
	version, err := g.tokens.GetTokenVersion(ctx, userID)
	if err == nil {
		return version, nil
	}
	if !errors.Is(err, repository.ErrTokenVersionMiss) {
		return 0, err
	}

	u, err := g.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// User đã bị xoá nhưng token vẫn còn hạn — coi như đã bị thu hồi.
			return 0, ErrTokenRevoked
		}
		return 0, err
	}

	// Cache lỗi thì không chặn request: chỉ mất lợi ích về hiệu năng.
	_ = g.tokens.SetTokenVersion(ctx, userID, u.TokenVersion, tokenVersionCacheTTL)
	return u.TokenVersion, nil
}
