// Package middleware chứa Gin middleware xác thực.
package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ninhdang/jwt-auth/internal/auth/service"
	"github.com/ninhdang/jwt-auth/pkg/jwt"
)

// Key lưu thông tin user vào gin.Context sau khi xác thực thành công.
const (
	ContextUserID = "auth.user_id"
	ContextEmail  = "auth.email"
	ContextJTI    = "auth.jti"
	ContextClaims = "auth.claims"
)

// Guard là phần kiểm tra stateful chạy sau khi chữ ký hợp lệ (blacklist,
// token_version). Tách thành interface để middleware không cần biết state nằm ở đâu.
type Guard interface {
	Authorize(ctx context.Context, claims *jwt.Claims) error
}

// RequireAuth xác thực access token qua ba lớp:
//
//  1. Verify chữ ký + hạn      — stateless, không chạm DB (Bài 1)
//  2. Check blacklist:{jti}    — token này đã bị logout chưa (Bài 5)
//  3. So ver với token_version — user có logout-all không (Bài 4)
//
// Lớp 1 lọc sạch token rác trước, nên lớp 2-3 chỉ chạy trên token thật.
func RequireAuth(manager *jwt.Manager, guard Guard) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := bearerToken(c.GetHeader("Authorization"))
		if err != nil {
			abort(c, "missing_token", "Authorization header phải có dạng: Bearer <token>")
			return
		}

		claims, err := manager.Parse(raw, jwt.TokenTypeAccess)
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenExpired):
				// FE dựa vào code này để biết khi nào cần gọi /refresh.
				abort(c, "token_expired", "Access token đã hết hạn")
			case errors.Is(err, jwt.ErrUnexpectedTyp):
				abort(c, "invalid_token", "Không thể dùng refresh token như access token")
			default:
				abort(c, "invalid_token", "Access token không hợp lệ")
			}
			return
		}

		if err := guard.Authorize(c.Request.Context(), claims); err != nil {
			switch {
			case errors.Is(err, service.ErrTokenRevoked):
				abort(c, "token_revoked", "Token đã bị thu hồi, vui lòng đăng nhập lại")
			case errors.Is(err, service.ErrTokenVersionMismatch):
				abort(c, "token_version_mismatch", "Phiên đã bị vô hiệu hoá, vui lòng đăng nhập lại")
			default:
				// Redis/DB chết thì từ chối chứ không cho qua: fail closed.
				slog.Error("token guard failed", "err", err)
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"error":   "auth_unavailable",
					"message": "Không xác thực được, vui lòng thử lại",
				})
			}
			return
		}

		c.Set(ContextUserID, claims.Subject)
		c.Set(ContextEmail, claims.Email)
		c.Set(ContextJTI, claims.ID)
		c.Set(ContextClaims, claims)
		c.Next()
	}
}

// Claims lấy nguyên claims của access token hiện tại (logout cần jti + exp).
func Claims(c *gin.Context) *jwt.Claims {
	v, _ := c.Get(ContextClaims)
	claims, _ := v.(*jwt.Claims)
	return claims
}

// UserID lấy id user đã xác thực ra khỏi context.
func UserID(c *gin.Context) string {
	v, _ := c.Get(ContextUserID)
	id, _ := v.(string)
	return id
}

func bearerToken(header string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("middleware: malformed authorization header")
	}
	return strings.TrimSpace(parts[1]), nil
}

func abort(c *gin.Context, code, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error":   code,
		"message": message,
	})
}
