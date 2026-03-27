package middleware

import (
	"net/http"
	"strings"

	"github.com/go-template/redis"
	"github.com/go-template/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be 'Bearer <token>'"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := utils.ValidateToken(tokenString, secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Stateful Enforcement: Check if session ID (JTI) exists in Redis
		if !redis.IsSessionValid(claims.UserID, claims.JTI) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session has been revoked or evicted"})
			c.Abort()
			return
		}

		// Store claims in context
		c.Set("userID", claims.UserID)
		c.Set("jti", claims.JTI)
		c.Next()
	}
}
