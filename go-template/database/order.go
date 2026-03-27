package database

import "github.com/go-template/models"

func CreateOrder(order *models.Order) error {
	return GetInstance().Create(order).Error
}

func GetOrder(id uint) (*models.Order, error) {
	var order models.Order
	err := GetInstance().First(&order, id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}
