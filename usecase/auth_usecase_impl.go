package usecase

import (
	"context"
	"errors"
	"time"

	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
	"real-time-chat-app/repository"
	"real-time-chat-app/security"
	auth "real-time-chat-app/util"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const refreshReuseGrace = 30 * time.Second

type AuthUsecaseImpl struct {
	*repository.AuthRepository
	RefreshTokens *repository.RefreshTokenRepository
	*validator.Validate
	*gorm.DB
	Log *logger.AppLogger
	*security.JWT
}

func NewAuthUsecase(authRepository *repository.AuthRepository, refreshTokens *repository.RefreshTokenRepository, validate *validator.Validate, DB *gorm.DB, logger *logger.AppLogger, JWT *security.JWT) AuthUsecase {
	return &AuthUsecaseImpl{AuthRepository: authRepository, RefreshTokens: refreshTokens, Validate: validate, DB: DB, Log: logger, JWT: JWT}
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
		Msg("Password verified, generating tokens")

	if err := uc.RefreshTokens.DeleteExpired(trx, currentAccount.User.ID, time.Now()); err != nil {
		uc.Log.Http.Warning.Warn().
			Err(err).
			Str("userId", currentAccount.User.ID).
			Msg("Failed to delete expired refresh tokens")
	}

	// a new login starts a new token family
	session, _, err := uc.issueSession(trx, currentAccount.User.ID, "")
	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("username", req.Username).
			Str("userId", currentAccount.User.ID).
			Msg("Failed to generate tokens")
		return res.LoginResponse{}, errors.New("failed to generate authentication token")
	}

	if err := trx.Commit().Error; err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("userId", currentAccount.User.ID).
			Msg("Failed to commit login transaction")
		return res.LoginResponse{}, errors.New("failed to process login")
	}

	uc.Log.Http.Info.Info().
		Str("username", req.Username).
		Str("userId", currentAccount.User.ID).
		Msg("Login successful, tokens generated")

	return session, nil
}

func (uc *AuthUsecaseImpl) RefreshSession(ctx context.Context, req *req.RefreshTokenRequest) (res.LoginResponse, error) {
	if err := uc.Validate.Struct(req); err != nil {
		return res.LoginResponse{}, ErrInvalidRefreshToken
	}

	now := time.Now()
	trx := uc.DB.WithContext(ctx).Begin()
	defer trx.Rollback()

	current, err := uc.RefreshTokens.FindByHash(trx, security.HashRefreshToken(req.RefreshToken))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.Log.Http.Warning.Warn().Msg("Unknown refresh token")
			return res.LoginResponse{}, ErrInvalidRefreshToken
		}
		uc.Log.Http.Error.Error().Err(err).Msg("Database error while finding refresh token")
		return res.LoginResponse{}, errors.New("failed to refresh session")
	}

	if !now.Before(current.ExpiresAt) {
		uc.Log.Http.Warning.Warn().
			Str("userId", current.UserID).
			Msg("Expired refresh token")
		return res.LoginResponse{}, ErrInvalidRefreshToken
	}

	if current.RevokedAt != nil {
		rotatedJustNow := current.ReplacedByID != "" && now.Sub(*current.RevokedAt) <= refreshReuseGrace
		if !rotatedJustNow {
			// A token that was rotated long ago or logged out is being
			// replayed: assume it leaked and end the whole session.
			if err := uc.RefreshTokens.RevokeFamily(uc.DB.WithContext(ctx), current.FamilyID, now); err != nil {
				uc.Log.Http.Error.Error().Err(err).Msg("Failed to revoke refresh token family")
			}
			uc.Log.Http.Warning.Warn().
				Str("userId", current.UserID).
				Str("familyId", current.FamilyID).
				Msg("Revoked refresh token was reused, session revoked")
			return res.LoginResponse{}, ErrInvalidRefreshToken
		}
	}

	session, newID, err := uc.issueSession(trx, current.UserID, current.FamilyID)
	if err != nil {
		uc.Log.Http.Error.Error().Err(err).Str("userId", current.UserID).Msg("Failed to generate tokens")
		return res.LoginResponse{}, errors.New("failed to refresh session")
	}

	if current.RevokedAt == nil {
		// If a concurrent request rotated it first, nothing is updated and
		// the new token simply lives next to the other one (same as the
		// grace period above).
		if _, err := uc.RefreshTokens.Rotate(trx, current.ID, newID, now); err != nil {
			uc.Log.Http.Error.Error().Err(err).Msg("Failed to rotate refresh token")
			return res.LoginResponse{}, errors.New("failed to refresh session")
		}
	}

	if err := trx.Commit().Error; err != nil {
		uc.Log.Http.Error.Error().Err(err).Msg("Failed to commit refresh transaction")
		return res.LoginResponse{}, errors.New("failed to refresh session")
	}

	uc.Log.Http.Info.Info().Str("userId", current.UserID).Msg("Session refreshed")
	return session, nil
}

func (uc *AuthUsecaseImpl) Logout(ctx context.Context, req *req.RefreshTokenRequest) error {
	if req.RefreshToken == "" {
		return nil
	}

	db := uc.DB.WithContext(ctx)
	current, err := uc.RefreshTokens.FindByHash(db, security.HashRefreshToken(req.RefreshToken))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		uc.Log.Http.Error.Error().Err(err).Msg("Database error while finding refresh token")
		return errors.New("failed to logout")
	}

	if err := uc.RefreshTokens.RevokeFamily(db, current.FamilyID, time.Now()); err != nil {
		uc.Log.Http.Error.Error().Err(err).Msg("Failed to revoke refresh token family")
		return errors.New("failed to logout")
	}

	uc.Log.Http.Info.Info().Str("userId", current.UserID).Msg("User logged out")
	return nil
}

// issueSession creates an access token and a refresh token for the user. An
// empty familyID starts a new family (a new login). It returns the response
// and the id of the stored refresh token.
func (uc *AuthUsecaseImpl) issueSession(db *gorm.DB, userID, familyID string) (res.LoginResponse, string, error) {
	accessToken, err := uc.JWT.GenerateAccessToken(userID)
	if err != nil {
		return res.LoginResponse{}, "", err
	}

	rawRefresh, refreshHash, err := security.NewRefreshToken()
	if err != nil {
		return res.LoginResponse{}, "", err
	}

	if familyID == "" {
		familyID = uuid.New().String()
	}

	refreshTTL := uc.JWT.RefreshTokenTTL()
	record := &entity.RefreshToken{
		UserID:    userID,
		FamilyID:  familyID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().Add(refreshTTL),
	}
	if err := db.Create(record).Error; err != nil {
		return res.LoginResponse{}, "", err
	}

	return res.LoginResponse{
		Token:            accessToken,
		TokenType:        "Bearer",
		ExpiresIn:        int64(uc.JWT.AccessTokenTTL().Seconds()),
		RefreshToken:     rawRefresh,
		RefreshExpiresIn: int64(refreshTTL.Seconds()),
	}, record.ID, nil
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
