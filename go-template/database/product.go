package database

import "github.com/go-template/models"

func CreateProduct(product *models.Product) error {
	return GetInstance().Create(product).Error
}

func GetProduct(id uint) (*models.Product, error) {
	var product models.Product
	err := GetInstance().First(&product, id).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}
