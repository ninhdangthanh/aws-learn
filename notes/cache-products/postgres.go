package main

import (
	"context"
	"errors"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// CatalogRepo là lớp duy nhất chạm tới Postgres (qua GORM).
type CatalogRepo struct {
	db *gorm.DB
}

func NewCatalogRepo(dsn string) (*CatalogRepo, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
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
	return &CatalogRepo{db: db}, nil
}

func (r *CatalogRepo) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (r *CatalogRepo) Migrate(ctx context.Context) error {
	if err := r.db.WithContext(ctx).AutoMigrate(&Client{}, &Category{}, &Product{}, &ProductSize{}); err != nil {
		return err
	}
	return r.seed(ctx)
}

// ---------------------------------------------------------------------------
// Read
// ---------------------------------------------------------------------------

func (r *CatalogRepo) ListClientIDs(ctx context.Context) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&Client{}).Order("id").Pluck("id", &ids).Error
	return ids, err
}

// LoadCatalog dựng read model của một client từ source of truth.
// Chỉ lấy category active + product active.
func (r *CatalogRepo) LoadCatalog(ctx context.Context, clientID string) (Catalog, error) {
	var client Client
	err := r.db.WithContext(ctx).Where("id = ?", clientID).Take(&client).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Catalog{}, ErrClientNotFound
	}
	if err != nil {
		return Catalog{}, err
	}

	var categories []Category
	if err := r.db.WithContext(ctx).
		Where("client_id = ? AND active", clientID).
		Order("sort_order, id").
		Find(&categories).Error; err != nil {
		return Catalog{}, err
	}

	var products []Product
	if err := r.db.WithContext(ctx).
		Preload("Sizes", func(db *gorm.DB) *gorm.DB {
			return db.Order("product_sizes.sort_order, product_sizes.id")
		}).
		Where("client_id = ? AND active", clientID).
		Order("category_id, id").
		Find(&products).Error; err != nil {
		return Catalog{}, err
	}

	byCategory := make(map[int64][]CatalogProduct, len(categories))
	for _, product := range products {
		sizes := make([]CatalogSize, 0, len(product.Sizes))
		for _, size := range product.Sizes {
			sizes = append(sizes, CatalogSize{ID: size.ID, Name: size.Name, Price: size.Price})
		}
		byCategory[product.CategoryID] = append(byCategory[product.CategoryID], CatalogProduct{
			ID:    product.ID,
			Name:  product.Name,
			Sizes: sizes,
		})
	}

	catalog := Catalog{
		ClientID:   client.ID,
		ClientName: client.Name,
		CachedAt:   time.Now().UTC(),
		Categories: make([]CatalogCategory, 0, len(categories)),
	}
	for _, category := range categories {
		items := byCategory[category.ID]
		if items == nil {
			items = []CatalogProduct{}
		}
		catalog.Categories = append(catalog.Categories, CatalogCategory{
			ID:       category.ID,
			Name:     category.Name,
			Products: items,
		})
	}
	return catalog, nil
}

func (r *CatalogRepo) GetProduct(ctx context.Context, clientID string, productID int64) (Product, error) {
	var product Product
	err := r.db.WithContext(ctx).
		Preload("Sizes", func(db *gorm.DB) *gorm.DB {
			return db.Order("product_sizes.sort_order, product_sizes.id")
		}).
		Where("id = ? AND client_id = ?", productID, clientID).
		Take(&product).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Product{}, ErrProductNotFound
	}
	if err != nil {
		return Product{}, err
	}
	return product, nil
}

// GetSizeForOrder đọc giá tại thời điểm checkout — luôn từ Postgres, không qua Redis.
func (r *CatalogRepo) GetSizeForOrder(ctx context.Context, clientID string, sizeID int64) (OrderSize, error) {
	var rows []OrderSize
	err := r.db.WithContext(ctx).
		Table("product_sizes AS s").
		Select("s.id AS size_id, s.name AS size_name, s.price, "+
			"p.id AS product_id, p.name AS product_name, p.active AS product_active").
		Joins("JOIN products AS p ON p.id = s.product_id").
		Where("s.id = ? AND p.client_id = ?", sizeID, clientID).
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return OrderSize{}, err
	}
	if len(rows) == 0 {
		return OrderSize{}, ErrSizeNotFound
	}
	return rows[0], nil
}

// ---------------------------------------------------------------------------
// Write
// ---------------------------------------------------------------------------

func (r *CatalogRepo) CreateProduct(ctx context.Context, clientID string, input ProductInput) (Product, error) {
	if err := input.validate(); err != nil {
		return Product{}, err
	}
	if err := r.assertCategoryBelongsToClient(ctx, clientID, input.CategoryID); err != nil {
		return Product{}, err
	}

	// GORM tự insert product + sizes trong một transaction và gán ProductID.
	product := Product{
		ClientID:   clientID,
		CategoryID: input.CategoryID,
		Name:       input.trimmedName(),
		Active:     input.isActive(),
		Sizes:      input.sizeRows(0),
	}
	if err := r.db.WithContext(ctx).Create(&product).Error; err != nil {
		return Product{}, err
	}
	return product, nil
}

// UpdateProduct thay toàn bộ sizes bằng danh sách mới. Size cũ bị xoá nên id size
// thay đổi — muốn đổi giá mà giữ nguyên id thì dùng UpdateSize.
func (r *CatalogRepo) UpdateProduct(ctx context.Context, clientID string, productID int64, input ProductInput) (Product, error) {
	if err := input.validate(); err != nil {
		return Product{}, err
	}
	if err := r.assertCategoryBelongsToClient(ctx, clientID, input.CategoryID); err != nil {
		return Product{}, err
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Product{}).
			Where("id = ? AND client_id = ?", productID, clientID).
			Updates(map[string]any{
				"category_id": input.CategoryID,
				"name":        input.trimmedName(),
				"active":      input.isActive(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrProductNotFound
		}

		if err := tx.Where("product_id = ?", productID).Delete(&ProductSize{}).Error; err != nil {
			return err
		}
		sizes := input.sizeRows(productID)
		return tx.Create(&sizes).Error
	})
	if err != nil {
		return Product{}, err
	}
	return r.GetProduct(ctx, clientID, productID)
}

func (r *CatalogRepo) DeleteProduct(ctx context.Context, clientID string, productID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Xoá size trước để không vướng foreign key; nếu product không tồn tại thì
		// transaction rollback nên các size vừa xoá được trả lại.
		if err := tx.Where("product_id = ?", productID).Delete(&ProductSize{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND client_id = ?", productID, clientID).Delete(&Product{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrProductNotFound
		}
		return nil
	})
}

func (r *CatalogRepo) UpdateSize(ctx context.Context, clientID string, sizeID int64, input SizeInput) (ProductSize, error) {
	if err := input.validate(); err != nil {
		return ProductSize{}, err
	}

	ownedByClient := r.db.Model(&Product{}).Select("id").Where("client_id = ?", clientID)
	result := r.db.WithContext(ctx).
		Model(&ProductSize{}).
		Where("id = ? AND product_id IN (?)", sizeID, ownedByClient).
		Updates(map[string]any{"name": input.trimmedName(), "price": input.Price})
	if result.Error != nil {
		return ProductSize{}, result.Error
	}
	if result.RowsAffected == 0 {
		return ProductSize{}, ErrSizeNotFound
	}

	var size ProductSize
	if err := r.db.WithContext(ctx).Where("id = ?", sizeID).Take(&size).Error; err != nil {
		return ProductSize{}, err
	}
	return size, nil
}

func (r *CatalogRepo) assertCategoryBelongsToClient(ctx context.Context, clientID string, categoryID int64) error {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Category{}).
		Where("id = ? AND client_id = ?", categoryID, clientID).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrCategoryNotFound
	}
	return nil
}
