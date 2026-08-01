package main

import (
	"context"
	"errors"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ProductRepo là lớp duy nhất chạm tới Postgres (qua GORM).
type ProductRepo struct {
	db *gorm.DB
}

func NewProductRepo(dsn string) (*ProductRepo, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return &ProductRepo{db: db}, nil
}

func (r *ProductRepo) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Migrate tạo bảng và seed data mẫu để demo chạy được ngay, không cần file migration.
func (r *ProductRepo) Migrate(ctx context.Context) error {
	if err := r.db.WithContext(ctx).AutoMigrate(&Product{}); err != nil {
		return err
	}

	var count int64
	if err := r.db.WithContext(ctx).Model(&Product{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	seed := []Product{
		{ClientID: "1001", Name: "Pizza", Price: 100000, Active: true},
		{ClientID: "1001", Name: "Burger", Price: 75000, Active: true},
		{ClientID: "1002", Name: "Coffee", Price: 45000, Active: true},
	}
	return r.db.WithContext(ctx).Create(&seed).Error
}

func (r *ProductRepo) List(ctx context.Context, clientID string) ([]Product, error) {
	products := make([]Product, 0)
	err := r.db.WithContext(ctx).
		Where("client_id = ?", clientID).
		Order("id").
		Find(&products).Error
	if err != nil {
		return nil, err
	}
	return products, nil
}

func (r *ProductRepo) Get(ctx context.Context, clientID string, productID int64) (Product, error) {
	var product Product
	err := r.db.WithContext(ctx).
		Where("client_id = ? AND id = ?", clientID, productID).
		First(&product).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Product{}, ErrProductNotFound
	}
	if err != nil {
		return Product{}, err
	}
	return product, nil
}

func (r *ProductRepo) Create(ctx context.Context, clientID string, input ProductInput) (Product, error) {
	if err := input.validate(); err != nil {
		return Product{}, err
	}

	product := Product{
		ClientID: clientID,
		Name:     input.trimmedName(),
		Price:    input.Price,
		Active:   input.isActive(),
	}
	if err := r.db.WithContext(ctx).Create(&product).Error; err != nil {
		return Product{}, err
	}
	return product, nil
}

func (r *ProductRepo) Update(ctx context.Context, clientID string, productID int64, input ProductInput) (Product, error) {
	if err := input.validate(); err != nil {
		return Product{}, err
	}

	result := r.db.WithContext(ctx).
		Model(&Product{}).
		Where("client_id = ? AND id = ?", clientID, productID).
		Updates(map[string]any{
			"name":   input.trimmedName(),
			"price":  input.Price,
			"active": input.isActive(),
		})
	if result.Error != nil {
		return Product{}, result.Error
	}
	if result.RowsAffected == 0 {
		return Product{}, ErrProductNotFound
	}
	return r.Get(ctx, clientID, productID)
}

func (r *ProductRepo) Delete(ctx context.Context, clientID string, productID int64) error {
	result := r.db.WithContext(ctx).
		Where("client_id = ? AND id = ?", clientID, productID).
		Delete(&Product{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrProductNotFound
	}
	return nil
}
