package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ninhdang/jwt-auth/internal/auth/model"
)

// RefreshRepository lưu metadata refresh token trong PostgreSQL để audit và
// quản lý thiết bị. Nguồn sự thật cho việc "token còn hiệu lực hay không" là
// Redis (xem TokenStore) — bảng này chỉ để tra cứu.
type RefreshRepository struct {
	db *gorm.DB
}

func NewRefreshRepository(db *gorm.DB) *RefreshRepository {
	return &RefreshRepository{db: db}
}

func (r *RefreshRepository) Create(ctx context.Context, rt *model.RefreshToken) error {
	if err := r.db.WithContext(ctx).Create(rt).Error; err != nil {
		return fmt.Errorf("repository: create refresh token: %w", err)
	}
	return nil
}

// Revoke đánh dấu một jti đã bị thu hồi (logout).
func (r *RefreshRepository) Revoke(ctx context.Context, jti string) error {
	now := time.Now()
	err := r.db.WithContext(ctx).
		Model(&model.RefreshToken{}).
		Where("jti = ? AND revoked_at IS NULL", jti).
		Update("revoked_at", now).Error
	if err != nil {
		return fmt.Errorf("repository: revoke refresh token: %w", err)
	}
	return nil
}

// ListActiveByUser trả về các phiên còn sống của user (dùng cho màn "thiết bị đăng nhập").
func (r *RefreshRepository) ListActiveByUser(ctx context.Context, userID string) ([]model.RefreshToken, error) {
	var tokens []model.RefreshToken
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, time.Now()).
		Order("created_at DESC").
		Find(&tokens).Error
	if err != nil {
		return nil, fmt.Errorf("repository: list refresh tokens: %w", err)
	}
	return tokens, nil
}

// RevokeByUserAndDevice thu hồi refresh token cũ của cùng một thiết bị, tránh
// mỗi lần login lại tạo thêm một hàng rác cho cùng device.
func (r *RefreshRepository) RevokeByUserAndDevice(ctx context.Context, userID, deviceID string) error {
	err := r.db.WithContext(ctx).
		Model(&model.RefreshToken{}).
		Where("user_id = ? AND device_id = ? AND revoked_at IS NULL", userID, deviceID).
		Update("revoked_at", time.Now()).Error
	if err != nil {
		return fmt.Errorf("repository: revoke refresh tokens by device: %w", err)
	}
	return nil
}
