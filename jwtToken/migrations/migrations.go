// Package migrations nhúng các file SQL vào binary để app tự chạy migration
// lúc khởi động, không cần cài thêm CLI ngoài.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
