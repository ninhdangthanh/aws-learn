// Package middleware chứa Gin middleware xác thực.
package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ninhdang/jwt-auth/pkg/jwt"
)

// Key lưu thông tin user vào gin.Context sau khi xác thực thành công.
const (
	ContextUserID = "auth.user_id"
	ContextEmail  = "auth.email"
	ContextJTI    = "auth.jti"
)

// RequireAuth là middleware của Bài 1: chỉ verify chữ ký + hạn của access token,
// KHÔNG query database. Đây chính là điểm khác biệt so với session.
func RequireAuth(manager *jwt.Manager) gin.HandlerFunc {
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

		// Phase 2 sẽ chèn thêm ở đây: check blacklist:{jti} và so token_version với DB.

		c.Set(ContextUserID, claims.Subject)
		c.Set(ContextEmail, claims.Email)
		c.Set(ContextJTI, claims.ID)
		c.Next()
	}
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
