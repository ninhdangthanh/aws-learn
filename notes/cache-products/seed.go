package main

import (
	"context"

	"gorm.io/gorm"
)

// Seed data mô phỏng menu thật của hai quán F&B để demo chạy được ngay.
// Có chủ đích cài sẵn hai case đặc biệt:
//   - product "Trà Sữa Ngưng Bán" (active = false) -> không vào catalog, order bị reject
//   - category "Khuyến Mãi Hè" (active = false)    -> cả nhánh không vào catalog
type seedSize struct {
	name  string
	price int64
}

type seedProduct struct {
	name   string
	active bool
	sizes  []seedSize
}

type seedCategory struct {
	name     string
	active   bool
	products []seedProduct
}

type seedClient struct {
	id         string
	name       string
	categories []seedCategory
}

// oneSize là món bán một giá — vẫn phải có size, đặt tên "Mặc định".
func oneSize(price int64) []seedSize {
	return []seedSize{{name: "Mặc định", price: price}}
}

var seedData = []seedClient{
	{
		id:   "1001",
		name: "Trà Sữa Nhà Làm",
		categories: []seedCategory{
			{
				name:   "Đồ Uống",
				active: true,
				products: []seedProduct{
					{name: "Trà Đào Cam Sả", active: true, sizes: []seedSize{
						{name: "S", price: 30000}, {name: "M", price: 40000}, {name: "L", price: 50000},
					}},
					{name: "Trà Sữa Trân Châu Đường Đen", active: true, sizes: []seedSize{
						{name: "M", price: 45000}, {name: "L", price: 55000},
					}},
					{name: "Trà Vải Hoa Hồng", active: true, sizes: []seedSize{
						{name: "S", price: 32000}, {name: "M", price: 42000}, {name: "L", price: 52000},
					}},
					{name: "Hồng Trà Sữa", active: true, sizes: []seedSize{
						{name: "M", price: 40000}, {name: "L", price: 48000},
					}},
					{name: "Sữa Tươi Trân Châu Đường Đen", active: true, sizes: []seedSize{
						{name: "M", price: 45000}, {name: "L", price: 55000},
					}},
					{name: "Matcha Latte", active: true, sizes: []seedSize{
						{name: "M", price: 50000}, {name: "L", price: 60000},
					}},
					{name: "Cà Phê Sữa Đá", active: true, sizes: []seedSize{
						{name: "M", price: 35000}, {name: "L", price: 45000},
					}},
					{name: "Nước Ép Ổi", active: true, sizes: oneSize(45000)},
					{name: "Trà Sữa Ngưng Bán", active: false, sizes: []seedSize{
						{name: "M", price: 40000},
					}},
				},
			},
			{
				name:   "Topping",
				active: true,
				products: []seedProduct{
					{name: "Trân Châu Đen", active: true, sizes: oneSize(8000)},
					{name: "Thạch Phô Mai", active: true, sizes: oneSize(10000)},
					{name: "Pudding Trứng", active: true, sizes: oneSize(10000)},
					{name: "Kem Cheese", active: true, sizes: oneSize(12000)},
				},
			},
			{
				name:   "Ăn Vặt",
				active: true,
				products: []seedProduct{
					{name: "Bánh Tráng Trộn", active: true, sizes: oneSize(35000)},
					{name: "Khoai Tây Lắc Phô Mai", active: true, sizes: []seedSize{
						{name: "Vừa", price: 30000}, {name: "Lớn", price: 45000},
					}},
					{name: "Gà Rán", active: true, sizes: []seedSize{
						{name: "3 miếng", price: 55000}, {name: "6 miếng", price: 99000},
					}},
				},
			},
		},
	},
	{
		id:   "1002",
		name: "Pizza Ba Miền",
		categories: []seedCategory{
			{
				name:   "Pizza",
				active: true,
				products: []seedProduct{
					{name: "Pizza Hải Sản", active: true, sizes: []seedSize{
						{name: "S", price: 149000}, {name: "M", price: 199000}, {name: "L", price: 259000},
					}},
					{name: "Pizza Bò Nướng Tiêu", active: true, sizes: []seedSize{
						{name: "S", price: 139000}, {name: "M", price: 189000}, {name: "L", price: 249000},
					}},
					{name: "Pizza Phô Mai", active: true, sizes: []seedSize{
						{name: "S", price: 129000}, {name: "M", price: 169000}, {name: "L", price: 219000},
					}},
				},
			},
			{
				name:   "Mì Ý",
				active: true,
				products: []seedProduct{
					{name: "Mì Ý Sốt Bò Bằm", active: true, sizes: oneSize(89000)},
					{name: "Mì Ý Sốt Kem Nấm", active: true, sizes: oneSize(95000)},
				},
			},
			{
				name:   "Nước",
				active: true,
				products: []seedProduct{
					{name: "Coca-Cola", active: true, sizes: []seedSize{
						{name: "Lon 330ml", price: 20000}, {name: "Chai 1.5L", price: 35000},
					}},
					{name: "Nước Suối", active: true, sizes: oneSize(12000)},
				},
			},
			{
				name:   "Khuyến Mãi Hè",
				active: false,
				products: []seedProduct{
					{name: "Combo Hè Mát Lạnh", active: true, sizes: oneSize(99000)},
				},
			},
		},
	},
}

// seed chỉ chạy khi bảng clients còn rỗng.
func (r *CatalogRepo) seed(ctx context.Context) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Client{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, client := range seedData {
			row := Client{ID: client.id, Name: client.name}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}

			for categoryIndex, category := range client.categories {
				categoryRow := Category{
					ClientID:  client.id,
					Name:      category.name,
					SortOrder: categoryIndex,
					Active:    category.active,
				}
				if err := tx.Create(&categoryRow).Error; err != nil {
					return err
				}

				for _, product := range category.products {
					sizes := make([]ProductSize, 0, len(product.sizes))
					for sizeIndex, size := range product.sizes {
						sizes = append(sizes, ProductSize{
							Name:      size.name,
							Price:     size.price,
							SortOrder: sizeIndex,
						})
					}
					productRow := Product{
						ClientID:   client.id,
						CategoryID: categoryRow.ID,
						Name:       product.name,
						Active:     product.active,
						Sizes:      sizes,
					}
					if err := tx.Create(&productRow).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}
