package service

import (
	"context"

	"github.com/go-template/database"
	"github.com/go-template/elastic"
	"github.com/go-template/messaging"
	"github.com/go-template/models"
	"github.com/go-template/redis"
)

type ProductService interface {
	CreateProduct(ctx context.Context, product *models.Product) error
	GetProduct(ctx context.Context, id uint) (*models.Product, error)
}

type productService struct{}

func NewProductService() ProductService {
	return &productService{}
}

func (s *productService) CreateProduct(ctx context.Context, product *models.Product) error {
	if err := database.CreateProduct(product); err != nil {
		return err
	}
	_ = elastic.IndexProduct(product)
	_ = redis.CacheProduct(product)
	_ = messaging.PublishProductCreatedEvent(product)
	return nil
}

func (s *productService) GetProduct(ctx context.Context, id uint) (*models.Product, error) {
	if product, err := redis.GetCachedProduct(id); err == nil && product != nil {
		return product, nil
	}
	product, err := database.GetProduct(id)
	if err != nil {
		return nil, err
	}
	_ = redis.CacheProduct(product)
	return product, nil
}
