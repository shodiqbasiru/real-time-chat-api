package usecase

import (
	"context"
	"real-time-chat-app/dto/res"
)

type UserUsecase interface {
	GetUserByID(ctx context.Context, token string) (res.UserResponse, error)
}
