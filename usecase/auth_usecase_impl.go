package usecase

import (
	"context"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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
	*logrus.Logger
	*security.JWT
}

func NewAuthUsecase(authRepository *repository.AuthRepository, validate *validator.Validate, DB *gorm.DB, logger *logrus.Logger, JWT *security.JWT) AuthUsecase {
	return &AuthUsecaseImpl{AuthRepository: authRepository, Validate: validate, DB: DB, Logger: logger, JWT: JWT}
}

func (uc *AuthUsecaseImpl) LoginUser(ctx context.Context, req *req.LoginRequest) (res.LoginResponse, error) {
	uc.Logger.Infof("New Request = %v", req)

	// validate request
	if err := uc.Validate.Struct(req); err != nil {
		uc.Logger.WithError(err).Errorf("failed to validete request : %v", err)
		return res.LoginResponse{}, err
	}
	// start transaction
	trx := uc.DB.WithContext(ctx).Begin()
	defer trx.Rollback()

	// find BY Username
	currentAccount, err := uc.AuthRepository.FindByUsername(trx, req.Username)
	if err != nil {
		uc.Logger.WithError(err).Errorf("Failed to find username = %v", err)
		return res.LoginResponse{}, err
	}
	// compare the password
	if matchPassword := auth.ComparePassword(currentAccount.Password, req.Password); !matchPassword {
		uc.Logger.WithError(err).Errorf("Failed to compare password = %v", err)
		return res.LoginResponse{}, err
	}
	// generate token
	token, err := uc.JWT.GenerateToken(&currentAccount.User)
	if err != nil {
		uc.Logger.WithError(err).Errorf("failed to generate token = %v", err)
	}
	// mapping response
	return res.LoginResponse{
		Token: token,
	}, nil
}

func (uc *AuthUsecaseImpl) RegisterUser(ctx context.Context, req *req.RegisterRequest) (res.RegisterResponse, error) {
	// validate request
	if err := uc.Validate.Struct(req); err != nil {
		uc.Logger.WithError(err).Errorf("failed to validete request : %v", err)
		return res.RegisterResponse{}, err
	}
	// start transaction
	trx := uc.DB.WithContext(ctx).Begin()
	defer trx.Rollback()
	// mapping request to entity
	hashPassword, _ := auth.HashPassword(req.Password)

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
	// save to db
	if err := uc.AuthRepository.Save(ctx, trx, newAccount); err != nil {
		uc.Logger.WithError(err).Errorf("failed to save user : %v", err)
		return res.RegisterResponse{}, err
	}
	// if success commit else rollback
	if err := trx.Commit().Error; err != nil {
		uc.Logger.WithError(err).Errorf("failed to commit user : %v", err)
		return res.RegisterResponse{}, err
	}
	// mapping response
	return res.RegisterResponse{
		ID:       newAccount.ID,
		Username: newAccount.UserName,
		Email:    newUser.Email,
	}, nil
}
