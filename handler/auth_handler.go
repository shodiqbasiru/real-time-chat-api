package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/usecase"
)

type AuthHandler struct {
	usecase.AuthUsecase
	*logrus.Logger
}

func NewAuthHandler(authUseCase usecase.AuthUsecase, logger *logrus.Logger) *AuthHandler {
	return &AuthHandler{AuthUsecase: authUseCase, Logger: logger}
}

func (handler *AuthHandler) RegisterUser(ctx *fiber.Ctx) error {
	// parse request
	payload := new(req.RegisterRequest)
	if err := ctx.BodyParser(payload); err != nil {
		return err
	}
	// get from useCase
	registerResponse, err := handler.AuthUsecase.RegisterUser(ctx.Context(), payload)
	if err != nil {
		handler.Logger.WithError(err).Errorf("Failed to register new user: %v", err)
		return err
	}
	// response
	response := res.CommonResponse[res.RegisterResponse]{
		Message:    "Successfully to register new user",
		StatusCode: fiber.StatusOK,
		Data:       registerResponse,
	}
	handler.Logger.Infof("Success register user with id: %s", registerResponse.ID)
	return ctx.Status(fiber.StatusOK).JSON(response)
}

func (handler *AuthHandler) LoginUser(ctx *fiber.Ctx) error {
	// parse request
	payload := new(req.LoginRequest)
	if err := ctx.BodyParser(payload); err != nil {
		return err
	}
	// get from useCase
	loginResponse, err := handler.AuthUsecase.LoginUser(ctx.Context(), payload)
	if err != nil {
		handler.Logger.WithError(err).Errorf("Failed to login: %v", err)
		return err
	}
	// response
	response := res.CommonResponse[res.LoginResponse]{
		Message:    "Successfully to login",
		StatusCode: fiber.StatusOK,
		Data:       loginResponse,
	}
	return ctx.Status(fiber.StatusOK).JSON(response)
}
