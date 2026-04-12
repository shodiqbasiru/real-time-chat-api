package repository

import (
	"context"
	"real-time-chat-app/entity"

	"gorm.io/gorm"
)

type UserRepository struct {
	Repository[entity.User]
}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (repository UserRepository) FindUserByID(ctx context.Context, db *gorm.DB, id string) (*entity.User, error) {
	var user entity.User
	err := db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
