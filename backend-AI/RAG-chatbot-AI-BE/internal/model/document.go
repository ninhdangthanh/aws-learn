package model

import (
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Filename   string    `gorm:"type:varchar(500);not null" json:"filename"`
	StoragePath *string   `gorm:"type:text" json:"-"`
	FileSize   int64     `gorm:"not null" json:"file_size"`
	FileType   string    `gorm:"type:varchar(20);not null" json:"file_type"`
	Status     string    `gorm:"type:varchar(20);not null;default:pending" json:"status"`
	ChunkCount int32     `gorm:"not null;default:0" json:"chunk_count"`
	ErrorMsg   *string   `gorm:"type:text" json:"error_msg"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Document) TableName() string {
	return "documents"
}
