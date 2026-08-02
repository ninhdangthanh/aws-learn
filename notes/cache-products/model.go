package main

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrClientNotFound   = errors.New("client not found")
	ErrCategoryNotFound = errors.New("category not found")
	ErrProductNotFound  = errors.New("product not found")
	ErrSizeNotFound     = errors.New("size not found")
)

// ---------------------------------------------------------------------------
// Source of truth: Postgres
// ---------------------------------------------------------------------------

type Client struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name      string    `json:"name" gorm:"not null"`
	CreatedAt time.Time `json:"-" gorm:"not null;autoCreateTime"`
}

func (Client) TableName() string { return "clients" }

type Category struct {
	ID       int64  `json:"id" gorm:"primaryKey"`
	ClientID string `json:"client_id" gorm:"not null;type:varchar(64);index:idx_categories_client_id"`
	Name     string `json:"name" gorm:"not null"`
	// SortOrder quyết định thứ tự category hiển thị trên menu.
	SortOrder int       `json:"sort_order" gorm:"not null"`
	Active    bool      `json:"active" gorm:"not null"`
	CreatedAt time.Time `json:"-" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"-" gorm:"not null;autoUpdateTime"`
}

func (Category) TableName() string { return "categories" }

type Product struct {
	ID         int64  `json:"id" gorm:"primaryKey"`
	ClientID   string `json:"client_id" gorm:"not null;type:varchar(64);index:idx_products_client_id"`
	CategoryID int64  `json:"category_id" gorm:"not null;index:idx_products_category_id"`
	Name       string `json:"name" gorm:"not null"`
	// Không đặt `default:true`: GORM sẽ bỏ field zero-value ra khỏi INSERT khi có
	// default tag, làm cho `active:false` lúc create bị DB ghi đè thành true.
	Active    bool          `json:"active" gorm:"not null"`
	CreatedAt time.Time     `json:"-" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time     `json:"-" gorm:"not null;autoUpdateTime"`
	Sizes     []ProductSize `json:"sizes" gorm:"foreignKey:ProductID;constraint:OnDelete:CASCADE"`
}

func (Product) TableName() string { return "products" }

// ProductSize giữ giá. Product không còn cột price — mọi món đều bán theo size,
// món một giá thì có đúng một size tên "Mặc định".
type ProductSize struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	ProductID int64     `json:"product_id" gorm:"not null;index:idx_product_sizes_product_id"`
	Name      string    `json:"name" gorm:"not null"`
	Price     int64     `json:"price" gorm:"not null;check:price > 0"`
	SortOrder int       `json:"sort_order" gorm:"not null"`
	CreatedAt time.Time `json:"-" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"-" gorm:"not null;autoUpdateTime"`
}

func (ProductSize) TableName() string { return "product_sizes" }

// ---------------------------------------------------------------------------
// Read model: payload nằm trong Redis
// ---------------------------------------------------------------------------

// Catalog là toàn bộ menu đang bán của một client, đúng shape frontend cần:
// load một lần rồi filter theo category ở phía client.
//
// Chỉ chứa category active và product active — xem docs/implement-plan-1.md.
type Catalog struct {
	ClientID   string `json:"client_id"`
	ClientName string `json:"client_name"`
	// CachedAt là lúc entry được build từ Postgres, cho thấy cache đã cũ bao lâu.
	CachedAt   time.Time         `json:"cached_at"`
	Categories []CatalogCategory `json:"categories"`
}

type CatalogCategory struct {
	ID       int64            `json:"id"`
	Name     string           `json:"name"`
	Products []CatalogProduct `json:"products"`
}

type CatalogProduct struct {
	ID    int64         `json:"id"`
	Name  string        `json:"name"`
	Sizes []CatalogSize `json:"sizes"`
}

type CatalogSize struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

// ---------------------------------------------------------------------------
// Request DTO
// ---------------------------------------------------------------------------

type SizeInput struct {
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

func (in SizeInput) trimmedName() string { return strings.TrimSpace(in.Name) }

func (in SizeInput) validate() error {
	if in.trimmedName() == "" || in.Price <= 0 {
		return errors.New("size name is required and price must be positive")
	}
	return nil
}

type ProductInput struct {
	CategoryID int64       `json:"category_id"`
	Name       string      `json:"name"`
	Active     *bool       `json:"active,omitempty"`
	Sizes      []SizeInput `json:"sizes"`
}

func (in ProductInput) validate() error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("name is required")
	}
	if in.CategoryID <= 0 {
		return errors.New("category_id is required")
	}
	if len(in.Sizes) == 0 {
		return errors.New("at least one size is required")
	}
	for _, size := range in.Sizes {
		if err := size.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (in ProductInput) trimmedName() string { return strings.TrimSpace(in.Name) }

func (in ProductInput) isActive() bool {
	if in.Active == nil {
		return true
	}
	return *in.Active
}

// sizeRows dựng các row product_sizes theo đúng thứ tự client gửi lên.
func (in ProductInput) sizeRows(productID int64) []ProductSize {
	rows := make([]ProductSize, 0, len(in.Sizes))
	for i, size := range in.Sizes {
		rows = append(rows, ProductSize{
			ProductID: productID,
			Name:      size.trimmedName(),
			Price:     size.Price,
			SortOrder: i,
		})
	}
	return rows
}

// ---------------------------------------------------------------------------
// Order DTO — command path, luôn đọc Postgres
// ---------------------------------------------------------------------------

type OrderItemInput struct {
	SizeID   int64 `json:"size_id"`
	Quantity int   `json:"quantity"`
}

type CreateOrderRequest struct {
	Items []OrderItemInput `json:"items"`
}

type OrderLine struct {
	SizeID     int64  `json:"size_id"`
	ProductID  int64  `json:"product_id"`
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	UnitPrice  int64  `json:"unit_price"`
	LineAmount int64  `json:"line_amount"`
}

// OrderSize là kết quả join product_sizes + products dùng riêng cho checkout.
type OrderSize struct {
	SizeID        int64
	SizeName      string
	Price         int64
	ProductID     int64
	ProductName   string
	ProductActive bool
}
