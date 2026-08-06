package model

import (
	"time"

	"github.com/ninhdang/jwt-auth/internal/user"
)

// RefreshToken là metadata của refresh token lưu trong PostgreSQL.
// Giá trị token thật KHÔNG được lưu ở đây — chỉ có jti để audit và thu hồi.
type RefreshToken struct {
	ID        string     `gorm:"column:id;primaryKey"`
	UserID    string     `gorm:"column:user_id;index"`
	JTI       string     `gorm:"column:jti;uniqueIndex"`
	DeviceID  string     `gorm:"column:device_id"`
	UserAgent string     `gorm:"column:user_agent"`
	IP        string     `gorm:"column:ip"`
	ExpiresAt time.Time  `gorm:"column:expires_at"`
	RevokedAt *time.Time `gorm:"column:revoked_at"`
	CreatedAt time.Time  `gorm:"column:created_at"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

// ---- Request DTOs ----

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	FullName string `json:"full_name" binding:"max=120"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	// DeviceID cho phép mỗi thiết bị giữ một refresh token riêng (Bài 7).
	// Bỏ trống thì server tự sinh.
	DeviceID string `json:"device_id" binding:"max=64"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=72"`
	// DeviceID của thiết bị hiện tại, để cấp lại token sau khi thu hồi tất cả.
	DeviceID string `json:"device_id" binding:"max=64"`
}

// ---- Response DTOs ----

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	// ExpiresIn là số giây còn lại của access token.
	ExpiresIn int    `json:"expires_in"`
	DeviceID  string `json:"device_id"`
}

type AuthResponse struct {
	User   user.PublicUser `json:"user"`
	Tokens TokenPair       `json:"tokens"`
}

// SessionInfo mô tả một phiên (một refresh token đang sống) của user.
type SessionInfo struct {
	JTI       string    `json:"jti"`
	DeviceID  string    `json:"device_id"`
	UserAgent string    `json:"user_agent"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
