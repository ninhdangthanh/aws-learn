package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ninhdang/jwt-auth/internal/user"
)

// ErrUserNotFound tách lỗi domain khỏi lỗi của GORM.
var ErrUserNotFound = errors.New("repository: user not found")

// UserRepository truy cập bảng users.
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		return fmt.Errorf("repository: create user: %w", err)
	}
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: find user by email: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*user.User, error) {
	var u user.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository: find user by id: %w", err)
	}
	return &u, nil
}

// IncrementTokenVersion tăng token_version lên 1 và trả về giá trị mới.
// Đây là công tắc "Logout All" (Bài 9): mọi access token đang lưu hành mang
// ver cũ sẽ bị middleware từ chối ở lần kiểm tra kế tiếp.
// UPDATE ... RETURNING chạy nguyên tử, không bị race giữa read và write.
func (r *UserRepository) IncrementTokenVersion(ctx context.Context, userID string) (int, error) {
	var newVersion int
	err := r.db.WithContext(ctx).Raw(
		`UPDATE users SET token_version = token_version + 1, updated_at = now()
		 WHERE id = ? RETURNING token_version`, userID,
	).Scan(&newVersion).Error
	if err != nil {
		return 0, fmt.Errorf("repository: increment token version: %w", err)
	}
	if newVersion == 0 {
		return 0, ErrUserNotFound
	}
	return newVersion, nil
}

// UpdatePassword đổi password_hash của user.
func (r *UserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	res := r.db.WithContext(ctx).
		Model(&user.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{"password_hash": passwordHash, "updated_at": time.Now()})
	if res.Error != nil {
		return fmt.Errorf("repository: update password: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// ExistsByEmail dùng cho pre-check lúc đăng ký; unique index vẫn là chốt chặn cuối.
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&user.User{}).Where("email = ?", email).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("repository: count user by email: %w", err)
	}
	return count > 0, nil
}
