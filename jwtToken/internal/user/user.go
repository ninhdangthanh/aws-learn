package user

import "time"

// User là entity gốc của hệ thống auth.
type User struct {
	ID           string `gorm:"column:id;primaryKey"`
	Email        string `gorm:"column:email;uniqueIndex"`
	PasswordHash string `gorm:"column:password_hash"`
	FullName     string `gorm:"column:full_name"`
	// TokenVersion tăng lên mỗi lần "logout all" / đổi mật khẩu (Bài 4, Bài 9).
	TokenVersion int       `gorm:"column:token_version;default:1"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (User) TableName() string { return "users" }

// PublicUser là bản chiếu an toàn để trả ra API (không lộ password_hash).
type PublicUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) Public() PublicUser {
	return PublicUser{
		ID:        u.ID,
		Email:     u.Email,
		FullName:  u.FullName,
		CreatedAt: u.CreatedAt,
	}
}
