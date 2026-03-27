package service

import (
	"context"

	"github.com/go-template/database"
	"github.com/go-template/elastic"
	"github.com/go-template/messaging"
	"github.com/go-template/models"
	"github.com/go-template/redis"
)

type OrderService interface {
	CreateOrder(ctx context.Context, order *models.Order) error
	GetOrder(ctx context.Context, id uint) (*models.Order, error)
}

type orderService struct{}

func NewOrderService() OrderService {
	return &orderService{}
}

func (s *orderService) CreateOrder(ctx context.Context, order *models.Order) error {
	if err := database.CreateOrder(order); err != nil {
		return err
	}
	_ = elastic.IndexOrder(order)
	_ = redis.CacheOrder(order)
	_ = messaging.PublishOrderCreatedEvent(order)
	return nil
}

func (s *orderService) GetOrder(ctx context.Context, id uint) (*models.Order, error) {
	if order, err := redis.GetCachedOrder(id); err == nil && order != nil {
		return order, nil
	}
	order, err := database.GetOrder(id)
	if err != nil {
		return nil, err
	}
	_ = redis.CacheOrder(order)
	return order, nil
}
