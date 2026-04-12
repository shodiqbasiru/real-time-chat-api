package usecase

import (
	"context"
	"errors"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
	"real-time-chat-app/repository"
	"real-time-chat-app/security"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type UserUsecaseImpl struct {
	*repository.UserRepository
	*validator.Validate
	*gorm.DB
	Log *logger.AppLogger
	*security.JWT
}

func NewUserUsecase(userRepository *repository.UserRepository, validate *validator.Validate, DB *gorm.DB, logger *logger.AppLogger, JWT *security.JWT) UserUsecase {
	return &UserUsecaseImpl{UserRepository: userRepository, Validate: validate, DB: DB, Log: logger, JWT: JWT}
}

func (uc *UserUsecaseImpl) GetUserIDByToken(ctx context.Context, token string) (res.UserResponse, error) {
	uc.Log.Http.Info.Info().Msg("GetUserByID started")
	uc.Log.Http.Trace.Trace().Msg("Extracting user ID from token")

	// get id by token user
	userIdFromToken, err := uc.JWT.GetUserIdFromToken(token)
	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Msg("Failed to extract user ID from token")
		return res.UserResponse{}, errors.New("invalid token")
	}

	uc.Log.Http.Trace.Trace().
		Str("userId", userIdFromToken).
		Msg("Finding user by ID")

	return getUserId(ctx, uc, userIdFromToken)
}

func (uc *UserUsecaseImpl) GetUserByID(ctx context.Context, userId string) (res.UserResponse, error) {
	return getUserId(ctx, uc, userId)
}

func (uc *UserUsecaseImpl) GetAllUser(ctx context.Context) ([]res.UserResponse, error) {
	uc.Log.Http.Info.Info().Msg("GetAllUser started")

	uc.Log.Http.Trace.Trace().Msg("Fetching all users from database")

	// Find all users
	var users []entity.User
	if err := uc.UserRepository.FindAll(ctx, uc.DB, &users); err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Msg("Failed to get all users")
		return nil, err
	}

	uc.Log.Http.Trace.Trace().
		Int("userCount", len(users)).
		Msg("Mapping user entities to responses")

	var userResponses []res.UserResponse
	for _, user := range users {
		userResponses = append(userResponses, res.UserResponse{
			ID:          user.ID,
			Name:        user.Name,
			Email:       user.Email,
			PhoneNumber: user.PhoneNumber,
			CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	uc.Log.Http.Info.Info().
		Int("userCount", len(userResponses)).
		Msg("Successfully retrieved all users")

	return userResponses, nil
}

func (uc *UserUsecaseImpl) FindUser(ctx context.Context, userId string) (*entity.User, error) {
	user, err := uc.UserRepository.FindUserByID(ctx, uc.DB, userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.Log.Http.Warning.Warn().
				Str("userId", userId).
				Msg("User not found")
		} else {
			uc.Log.Http.Error.Error().
				Err(err).
				Str("userId", userId).
				Msg("Failed to find user")
		}
		return nil, err
	}
	return user, nil
}

func getUserId(ctx context.Context, uc *UserUsecaseImpl, userId string) (res.UserResponse, error) {
	// find user by id
	var user entity.User
	if err := uc.UserRepository.FindById(ctx, uc.DB, &user, userId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.Log.Http.Warning.Warn().
				Str("userId", userId).
				Msg("User not found")
		} else {
			uc.Log.Http.Error.Error().
				Err(err).
				Str("userId", userId).
				Msg("Failed to find user")
		}
		return res.UserResponse{}, err
	}

	uc.Log.Http.Info.Info().
		Str("userId", user.ID).
		Str("userName", user.Name).
		Str("email", user.Email).
		Msg("Successfully retrieved user")

	// mapping user response
	return res.UserResponse{
		ID:          user.ID,
		Name:        user.Name,
		Email:       user.Email,
		PhoneNumber: user.PhoneNumber,
		CreatedAt:   user.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}
