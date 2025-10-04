package repository

import "real-time-chat-app/entity"

type UserRepository struct {
	Repository[entity.User]
}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}
