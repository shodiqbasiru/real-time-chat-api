package handler

import (
	"errors"

	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/usecase"

	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	usecase.AuthUsecase
	*logger.AppLogger
}

func NewAuthHandler(authUseCase usecase.AuthUsecase, logger *logger.AppLogger) *AuthHandler {
	logger.Http.Info.Info().Msg("Auth handler initialized")
	return &AuthHandler{AuthUsecase: authUseCase, AppLogger: logger}
}

func (handler *AuthHandler) RegisterUser(ctx *fiber.Ctx) error {
	// STREAM: Log incoming request
	handler.AppLogger.Http.Stream.Info().
		Str("method", ctx.Method()).
		Str("path", ctx.Path()).
		Str("ip", ctx.IP()).
		Str("userAgent", ctx.Get("User-Agent")).
		Msg("Incoming register request")

	// Parse request
	payload := new(req.RegisterRequest)
	if err := ctx.BodyParser(payload); err != nil {
		handler.AppLogger.Http.Error.Error().
			Err(err).
			Str("path", ctx.Path()).
			Msg("Failed to parse register request body")

		handler.AppLogger.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusBadRequest).
			Msg("Response: Bad request - invalid body")

		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	handler.AppLogger.Http.Info.Info().
		Str("username", payload.Username).
		Str("email", payload.Email).
		Msg("Processing register request")

	// Call usecase
	registerResponse, err := handler.AuthUsecase.RegisterUser(ctx.Context(), payload)
	if err != nil {
		handler.AppLogger.Http.Error.Error().
			Err(err).
			Str("username", payload.Username).
			Str("email", payload.Email).
			Msg("Failed to register user")

		// 📤 STREAM: Log error response
		handler.AppLogger.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusBadRequest).
			Str("username", payload.Username).
			Msg("Response: Registration failed")

		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Success response
	response := res.CommonResponse[res.RegisterResponse]{
		Message:    "Successfully to register new user",
		StatusCode: fiber.StatusCreated,
		Data:       registerResponse,
	}

	handler.AppLogger.Http.Info.Info().
		Str("userId", registerResponse.ID).
		Str("username", registerResponse.Username).
		Str("email", registerResponse.Email).
		Msg("User registered successfully")

	handler.AppLogger.Http.Stream.Info().
		Int("statusCode", fiber.StatusCreated).
		Str("userId", registerResponse.ID).
		Str("username", registerResponse.Username).
		Msg("Response: User registered successfully")

	return ctx.Status(fiber.StatusCreated).JSON(response)
}

func (handler *AuthHandler) LoginUser(ctx *fiber.Ctx) error {
	// 📥 STREAM: Log incoming request
	handler.AppLogger.Http.Stream.Info().
		Str("method", ctx.Method()).
		Str("path", ctx.Path()).
		Str("ip", ctx.IP()).
		Str("userAgent", ctx.Get("User-Agent")).
		Msg("Incoming login request")

	// Parse request
	payload := new(req.LoginRequest)
	if err := ctx.BodyParser(payload); err != nil {
		handler.AppLogger.Http.Error.Error().
			Err(err).
			Str("path", ctx.Path()).
			Msg("Failed to parse login request body")

		handler.AppLogger.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusBadRequest).
			Msg("Response: Bad request - invalid body")

		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	handler.AppLogger.Http.Info.Info().
		Str("username", payload.Username).
		Msg("Processing login request")

	// Call usecase
	loginResponse, err := handler.AuthUsecase.LoginUser(ctx.Context(), payload)
	if err != nil {
		handler.AppLogger.Http.Error.Error().
			Err(err).
			Str("username", payload.Username).
			Msg("Login failed")

		// 📤 STREAM: Log error response
		handler.AppLogger.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusUnauthorized).
			Str("username", payload.Username).
			Msg("Response: Login failed")

		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Success response
	response := res.CommonResponse[res.LoginResponse]{
		Message:    "Successfully to login",
		StatusCode: fiber.StatusOK,
		Data:       loginResponse,
	}

	handler.AppLogger.Http.Info.Info().
		Str("username", payload.Username).
		Msg("User logged in successfully")

	// 📤 STREAM: Log success response
	handler.AppLogger.Http.Stream.Info().
		Int("statusCode", fiber.StatusOK).
		Str("username", payload.Username).
		Bool("tokenGenerated", loginResponse.Token != "").
		Msg("Response: Login successful")

	return ctx.Status(fiber.StatusOK).JSON(response)
}

func (handler *AuthHandler) RefreshToken(ctx *fiber.Ctx) error {
	payload := new(req.RefreshTokenRequest)
	if err := ctx.BodyParser(payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	session, err := handler.AuthUsecase.RefreshSession(ctx.Context(), payload)
	if err != nil {
		status := fiber.StatusInternalServerError
		if errors.Is(err, usecase.ErrInvalidRefreshToken) {
			status = fiber.StatusUnauthorized
		}
		handler.AppLogger.Http.Stream.Error().
			Err(err).
			Int("statusCode", status).
			Msg("Response: Refresh failed")
		return ctx.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(res.CommonResponse[res.LoginResponse]{
		Message:    "Successfully refreshed session",
		StatusCode: fiber.StatusOK,
		Data:       session,
	})
}

func (handler *AuthHandler) Logout(ctx *fiber.Ctx) error {
	payload := new(req.RefreshTokenRequest)
	if err := ctx.BodyParser(payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := handler.AuthUsecase.Logout(ctx.Context(), payload); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(res.CommonResponse[any]{
		Message:    "Successfully logged out",
		StatusCode: fiber.StatusOK,
	})
}
