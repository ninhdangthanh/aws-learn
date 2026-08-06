// Package config đọc cấu hình từ biến môi trường.
package config

import "os"

// Env đọc biến môi trường, trả về fallback nếu chưa đặt hoặc rỗng.
func Env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
