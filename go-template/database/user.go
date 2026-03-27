package database

import "github.com/go-template/models"

func CreateUser(user *models.User) error {
	return GetInstance().Create(user).Error
}

func GetUser(id uint) (*models.User, error) {
	var user models.User
	err := GetInstance().First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := GetInstance().Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
