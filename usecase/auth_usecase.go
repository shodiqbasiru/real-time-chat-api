package usecase

import (
	"context"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/dto/res"
)

type AuthUsecase interface {
	RegisterUser(ctx context.Context, request *req.RegisterRequest) (res.RegisterResponse, error)
	LoginUser(ctx context.Context, request *req.LoginRequest) (res.LoginResponse, error)
	// RefreshSession exchanges a refresh token for a new access token and a
	// new refresh token. The old refresh token stops working.
	RefreshSession(ctx context.Context, request *req.RefreshTokenRequest) (res.LoginResponse, error)
	// Logout revokes the refresh token and every token rotated from the
	// same login. Unknown tokens are ignored.
	Logout(ctx context.Context, request *req.RefreshTokenRequest) error
}
