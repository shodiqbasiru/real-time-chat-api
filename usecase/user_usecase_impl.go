package usecase

import (
	"context"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
	"real-time-chat-app/repository"
	"real-time-chat-app/security"
)

type UserUsecaseImpl struct {
	*repository.UserRepository
	*validator.Validate
	*gorm.DB
	*logrus.Logger
	*security.JWT
}

func NewUserUsecase(userRepository *repository.UserRepository, validate *validator.Validate, DB *gorm.DB, logger *logrus.Logger, JWT *security.JWT) UserUsecase {
	return &UserUsecaseImpl{UserRepository: userRepository, Validate: validate, DB: DB, Logger: logger, JWT: JWT}
}

func (uc *UserUsecaseImpl) GetUserByID(ctx context.Context, token string) (res.UserResponse, error) {
	uc.Logger.Infof("GetUserByID.token <=====> %v", token)

	// get id by token user
	userIdFromToken, err := uc.JWT.GetUserIdFromToken(token)
	if err != nil {
		uc.Logger.WithError(err).Errorf("Failed to find user id by token")
		return res.UserResponse{}, err
	}

	// find user by id
	var user entity.User
	if err := uc.UserRepository.FindById(ctx, uc.DB, &user, userIdFromToken); err != nil {
		uc.Logger.WithError(err).Errorf("Failed to find user = %v", err)
		return res.UserResponse{}, err
	}

	// mapping user response
	return res.UserResponse{
		ID:          user.ID,
		Name:        user.Name,
		Email:       user.Email,
		PhoneNumber: user.PhoneNumber,
		CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}
