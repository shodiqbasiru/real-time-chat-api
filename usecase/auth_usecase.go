package usecase

import (
	"context"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/dto/res"
)

type AuthUsecase interface {
	RegisterUser(ctx context.Context, request *req.RegisterRequest) (res.RegisterResponse, error)
	LoginUser(ctx context.Context, request *req.LoginRequest) (res.LoginResponse, error)
}
