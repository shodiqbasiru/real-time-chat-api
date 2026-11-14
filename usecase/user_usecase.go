package usecase

import (
	"context"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
)

type UserUsecase interface {
	GetUserIDByToken(ctx context.Context, token string) (res.UserResponse, error)
	GetUserByID(ctx context.Context, userId string) (res.UserResponse, error)
	GetAllUser(ctx context.Context) ([]res.UserResponse, error)
	FindUser(ctx context.Context, userID string) (*entity.User, error)
	UpdateProfile(ctx context.Context, token string, request *req.EditProfileRequest) (res.UserResponse, error)
}
