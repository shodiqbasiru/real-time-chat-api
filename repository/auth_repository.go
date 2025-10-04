package repository

import (
	"gorm.io/gorm"
	"real-time-chat-app/entity"
)

type AuthRepository struct {
	Repository[entity.Account]
}

func NewAuthRepository() *AuthRepository {
	return &AuthRepository{}
}

func (repository AuthRepository) FindByUsername(db *gorm.DB, username string) (entity.Account, error) {
	user := &entity.Account{}
	err := db.Preload("User").Where("user_name = ?", username).First(user).Error
	if err != nil {
		return *user, err
	}
	return *user, err
}
