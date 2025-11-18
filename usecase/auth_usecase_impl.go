package usecase

import (
	"context"
	"errors"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
	"real-time-chat-app/repository"
	"real-time-chat-app/security"
	auth "real-time-chat-app/util"
)

type AuthUsecaseImpl struct {
	*repository.AuthRepository
	*validator.Validate
	*gorm.DB
	Log *logger.AppLogger
	*security.JWT
}

func NewAuthUsecase(authRepository *repository.AuthRepository, validate *validator.Validate, DB *gorm.DB, logger *logger.AppLogger, JWT *security.JWT) AuthUsecase {
	return &AuthUsecaseImpl{AuthRepository: authRepository, Validate: validate, DB: DB, Log: logger, JWT: JWT}
}

func (uc *AuthUsecaseImpl) LoginUser(ctx context.Context, req *req.LoginRequest) (res.LoginResponse, error) {
	uc.Log.Http.Info.Info().
		Str("username", req.Username).
		Msg("LoginUser usecase started")

	// validate request
	if err := uc.Validate.Struct(req); err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("username", req.Username).
			Msg("Validation failed for login request")
		return res.LoginResponse{}, errors.New("invalid request data")
	}

	uc.Log.Http.Trace.Trace().
		Str("username", req.Username).
		Msg("Validation passed, starting database transaction")

	// start transaction
	trx := uc.DB.WithContext(ctx).Begin()
	defer trx.Rollback()

	uc.Log.Http.Trace.Trace().
		Str("username", req.Username).
		Msg("Finding user by username")

	// find BY Username
	currentAccount, err := uc.AuthRepository.FindByUsername(trx, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.Log.Http.Warning.Warn().
				Str("username", req.Username).
				Msg("User not found")
			return res.LoginResponse{}, errors.New("invalid username or password")
		}

		uc.Log.Http.Error.Error().
			Err(err).
			Str("username", req.Username).
			Msg("Database error while finding user")
		return res.LoginResponse{}, errors.New("failed to process login")
	}

	uc.Log.Http.Trace.Trace().
		Str("username", req.Username).
		Str("userId", currentAccount.User.ID).
		Msg("User found, verifying password")

	// compare the password
	if matchPassword := auth.ComparePassword(currentAccount.Password, req.Password); !matchPassword {
		uc.Log.Http.Warning.Warn().
			Str("username", req.Username).
			Msg("Invalid password attempt")
		return res.LoginResponse{}, errors.New("invalid username or password")
	}

	uc.Log.Http.Trace.Trace().
		Str("username", req.Username).
		Str("userId", currentAccount.User.ID).
		Msg("Password verified, generating JWT token")

	// generate token
	token, err := uc.JWT.GenerateToken(&currentAccount.User)
	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("username", req.Username).
			Str("userId", currentAccount.User.ID).
			Msg("Failed to generate JWT token")
		return res.LoginResponse{}, errors.New("failed to generate authentication token")
	}

	uc.Log.Http.Info.Info().
		Str("username", req.Username).
		Str("userId", currentAccount.User.ID).
		Msg("Login successful, token generated")

	// mapping response
	return res.LoginResponse{
		Token: token,
	}, nil
}

func (uc *AuthUsecaseImpl) RegisterUser(ctx context.Context, req *req.RegisterRequest) (res.RegisterResponse, error) {
	uc.Log.Http.Info.Info().
		Str("username", req.Username).
		Str("email", req.Email).
		Msg("RegisterUser usecase started")

	// Validate request
	if err := uc.Validate.Struct(req); err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("username", req.Username).
			Msg("Validation failed for register request")
		return res.RegisterResponse{}, errors.New("invalid request data")
	}
	uc.Log.Http.Trace.Trace().
		Str("username", req.Username).
		Str("email", req.Email).
		Msg("Validation passed, starting database transaction")

	// Start transaction
	trx := uc.DB.WithContext(ctx).Begin()
	defer trx.Rollback()

	uc.Log.Http.Trace.Trace().
		Str("username", req.Username).
		Msg("Hashing password")

	// Hash password
	hashPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("username", req.Username).
			Msg("Failed to hash password")
		return res.RegisterResponse{}, errors.New("failed to process password")
	}

	uc.Log.Http.Trace.Trace().
		Str("username", req.Username).
		Msg("Creating user entity")

	newUser := &entity.User{
		Name:        req.Username,
		Email:       req.Email,
		PhoneNumber: req.PhoneNumber,
	}

	newAccount := &entity.Account{
		UserName: req.Username,
		Password: hashPassword,
		User:     *newUser,
	}

	uc.Log.Http.Trace.Trace().
		Str("username", req.Username).
		Str("email", req.Email).
		Msg("Saving user to database")

	// save to db
	if err := uc.AuthRepository.Save(ctx, trx, newAccount); err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("username", req.Username).
			Str("email", req.Email).
			Msg("Failed to save user to database")

		return res.RegisterResponse{}, errors.New("failed to register user")
	}

	uc.Log.Http.Trace.Trace().
		Str("username", req.Username).
		Str("userId", newAccount.ID).
		Msg("Committing transaction")

	// Commit transaction
	if err := trx.Commit().Error; err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("username", req.Username).
			Str("userId", newAccount.ID).
			Msg("Failed to commit transaction")
		return res.RegisterResponse{}, errors.New("failed to complete registration")
	}

	uc.Log.Http.Info.Info().
		Str("userId", newAccount.ID).
		Str("username", newAccount.UserName).
		Str("email", newUser.Email).
		Msg("User registered successfully")

	// mapping response
	return res.RegisterResponse{
		ID:       newAccount.ID,
		Username: newAccount.UserName,
		Email:    newUser.Email,
	}, nil
}
