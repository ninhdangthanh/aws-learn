package postgres

import (
	"fmt"
	"io/fs"
	"sort"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// New mở connection pool tới PostgreSQL qua GORM.
func New(dsn string, debug bool) (*gorm.DB, error) {
	logLevel := logger.Warn
	if debug {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("postgres: sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}

	return db, nil
}

// Migrate chạy tuần tự các file *.up.sql theo thứ tự tên file.
// Mỗi file chỉ chạy một lần, được ghi nhận trong bảng schema_migrations.
func Migrate(db *gorm.DB, migrations fs.FS) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version     TEXT PRIMARY KEY,
		applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
	)`).Error; err != nil {
		return fmt.Errorf("postgres: create schema_migrations: %w", err)
	}

	files, err := fs.Glob(migrations, "*.up.sql")
	if err != nil {
		return fmt.Errorf("postgres: glob migrations: %w", err)
	}
	sort.Strings(files)

	for _, name := range files {
		var count int64
		if err := db.Raw("SELECT count(1) FROM schema_migrations WHERE version = ?", name).
			Scan(&count).Error; err != nil {
			return fmt.Errorf("postgres: check migration %s: %w", name, err)
		}
		if count > 0 {
			continue
		}

		content, err := fs.ReadFile(migrations, name)
		if err != nil {
			return fmt.Errorf("postgres: read migration %s: %w", name, err)
		}

		// Mỗi migration chạy trong một transaction để không rơi vào trạng thái nửa vời.
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(string(content)).Error; err != nil {
				return err
			}
			return tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name).Error
		})
		if err != nil {
			return fmt.Errorf("postgres: apply migration %s: %w", name, err)
		}
	}

	return nil
}
