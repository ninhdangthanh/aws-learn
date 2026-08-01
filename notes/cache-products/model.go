package main

import (
	"errors"
	"strings"
	"time"
)

var ErrProductNotFound = errors.New("product not found")

type Product struct {
	ID       int64  `json:"id" gorm:"primaryKey"`
	ClientID string `json:"client_id" gorm:"not null;index:idx_products_client_id"`
	Name     string `json:"name" gorm:"not null"`
	Price    int64  `json:"price" gorm:"not null;check:price > 0"`
	// Không đặt `default:true`: GORM sẽ bỏ field zero-value ra khỏi INSERT khi có
	// default tag, làm cho `active:false` lúc create bị DB ghi đè thành true.
	Active    bool      `json:"active" gorm:"not null"`
	CreatedAt time.Time `json:"-" gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `json:"-" gorm:"not null;autoUpdateTime"`
}

func (Product) TableName() string {
	return "products"
}

type ProductInput struct {
	Name   string `json:"name"`
	Price  int64  `json:"price"`
	Active *bool  `json:"active,omitempty"`
}

func (in ProductInput) validate() error {
	if strings.TrimSpace(in.Name) == "" || in.Price <= 0 {
		return errors.New("name is required and price must be positive")
	}
	return nil
}

func (in ProductInput) isActive() bool {
	if in.Active == nil {
		return true
	}
	return *in.Active
}

func (in ProductInput) trimmedName() string {
	return strings.TrimSpace(in.Name)
}

type OrderItemInput struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type CreateOrderRequest struct {
	Items []OrderItemInput `json:"items"`
}

type OrderLine struct {
	ProductID  int64  `json:"product_id"`
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	UnitPrice  int64  `json:"unit_price"`
	LineAmount int64  `json:"line_amount"`
}
