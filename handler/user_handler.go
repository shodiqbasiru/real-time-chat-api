package handler

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/usecase"
)

type UserHandler struct {
	usecase.UserUsecase
	Log *logger.AppLogger
}

func NewUserHandler(userUsecase usecase.UserUsecase, logger *logger.AppLogger) *UserHandler {
	return &UserHandler{UserUsecase: userUsecase, Log: logger}
}

func (handler *UserHandler) GetUserByToken(ctx *fiber.Ctx) error {
	handler.Log.Http.Stream.Info().
		Str("method", ctx.Method()).
		Str("path", ctx.Path()).
		Str("ip", ctx.IP()).
		Msg("Incoming request: Get user by token")

	token := ctx.Get("Authorization")[7:]

	handler.Log.Http.Info.Info().
		Str("path", ctx.Path()).
		Msg("Processing get user by token request")

	userResponse, err := handler.UserUsecase.GetUserIDByToken(ctx.Context(), token)
	if err != nil {
		handler.Log.Http.Error.Error().
			Err(err).
			Str("path", ctx.Path()).
			Msg("Failed to get user by token")

		handler.Log.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusInternalServerError).
			Msg("Response: Failed to get user")

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	response := res.CommonResponse[res.UserResponse]{
		Message:    "Successfully To Get User By ID",
		StatusCode: fiber.StatusOK,
		Data:       userResponse,
	}

	handler.Log.Http.Info.Info().
		Str("userId", userResponse.ID).
		Str("userName", userResponse.Name).
		Msg("Successfully retrieved user by token")

	handler.Log.Http.Stream.Info().
		Int("statusCode", fiber.StatusOK).
		Str("userId", userResponse.ID).
		Str("userName", userResponse.Name).
		Msg("Response: Successfully retrieved user")

	return ctx.Status(fiber.StatusOK).JSON(response)
}

func (handler *UserHandler) GetAllUsers(ctx *fiber.Ctx) error {
	handler.Log.Http.Stream.Info().
		Str("method", ctx.Method()).
		Str("path", ctx.Path()).
		Str("ip", ctx.IP()).
		Msg("Incoming request: Get all users")

	handler.Log.Http.Info.Info().
		Str("path", ctx.Path()).
		Msg("Processing get all users request")

	userResponses, err := handler.UserUsecase.GetAllUser(ctx.Context())
	if err != nil {
		handler.Log.Http.Error.Error().
			Err(err).
			Str("path", ctx.Path()).
			Msg("Failed to get all users")

		handler.Log.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusInternalServerError).
			Msg("Response: Failed to get users")

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	responses := res.CommonResponse[[]res.UserResponse]{
		Message:    "Successfully To Get All User",
		StatusCode: fiber.StatusOK,
		Data:       userResponses,
	}

	handler.Log.Http.Info.Info().
		Int("userCount", len(userResponses)).
		Msg("Successfully retrieved all users")

	handler.Log.Http.Stream.Info().
		Int("statusCode", fiber.StatusOK).
		Int("userCount", len(userResponses)).
		Msg("Response: Successfully retrieved all users")

	return ctx.Status(fiber.StatusOK).JSON(responses)
}

func (handler *UserHandler) GetUser(ctx *fiber.Ctx) error {
	userId := ctx.Params("userId")
	handler.Log.Http.Stream.Info().
		Str("method", ctx.Method()).
		Str("path", ctx.Path()).
		Str("userId", userId).
		Str("ip", ctx.IP()).
		Msg("Incoming request: Get User By Id")

	userResponse, err := handler.UserUsecase.GetUserByID(ctx.Context(), userId)
	if err != nil {
		handler.Log.Http.Error.Error().
			Err(err).
			Str("path", ctx.Path()).
			Msg("Failed to get a user")

		handler.Log.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusInternalServerError).
			Msg("Response: Failed to get a user")

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	response := res.CommonResponse[res.UserResponse]{
		Message:    "Successfully To Get a User",
		StatusCode: fiber.StatusOK,
		Data:       userResponse,
	}

	handler.Log.Http.Info.Info().
		Msg("Successfully to get a user")

	handler.Log.Http.Stream.Info().
		Int("statusCode", fiber.StatusOK).
		Msg("Response: Successfully to get a user")

	return ctx.Status(fiber.StatusOK).JSON(response)
}

// EditUser updates the signed-in user's name and phone number.
func (handler *UserHandler) EditUser(ctx *fiber.Ctx) error {
	handler.Log.Http.Stream.Info().
		Str("method", ctx.Method()).
		Str("path", ctx.Path()).
		Str("ip", ctx.IP()).
		Msg("Incoming request: Edit profile")

	var request req.EditProfileRequest
	if err := ctx.BodyParser(&request); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	token := ctx.Get("Authorization")[7:]
	userResponse, err := handler.UserUsecase.UpdateProfile(ctx.Context(), token, &request)
	if err != nil {
		status := fiber.StatusInternalServerError
		message := "failed to update profile"
		switch {
		case errors.Is(err, usecase.ErrInvalidProfile):
			status, message = fiber.StatusBadRequest, err.Error()
		case errors.Is(err, usecase.ErrPhoneTaken):
			status, message = fiber.StatusConflict, err.Error()
		case errors.Is(err, usecase.ErrUserNotFound):
			status, message = fiber.StatusNotFound, err.Error()
		}

		handler.Log.Http.Stream.Error().
			Err(err).
			Int("statusCode", status).
			Msg("Response: Failed to update profile")

		return ctx.Status(status).JSON(fiber.Map{"error": message})
	}

	handler.Log.Http.Stream.Info().
		Int("statusCode", fiber.StatusOK).
		Str("userId", userResponse.ID).
		Msg("Response: Profile updated")

	return ctx.Status(fiber.StatusOK).JSON(res.CommonResponse[res.UserResponse]{
		Message:    "Successfully To Update Profile",
		StatusCode: fiber.StatusOK,
		Data:       userResponse,
	})
}
