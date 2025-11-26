package usecase

import (
	"context"
	"real-time-chat-app/dto/res"
)

type UserUsecase interface {
	GetUserIDByToken(ctx context.Context, token string) (res.UserResponse, error)
	GetUserByID(ctx context.Context, userId string) (res.UserResponse, error)
	GetAllUser(ctx context.Context) ([]res.UserResponse, error)
}
