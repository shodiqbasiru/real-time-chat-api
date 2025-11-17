package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/usecase"
)

type UserHandler struct {
	usecase.UserUsecase
	*logrus.Logger
}

func NewUserHandler(userUsecase usecase.UserUsecase, logger *logrus.Logger) *UserHandler {
	return &UserHandler{UserUsecase: userUsecase, Logger: logger}
}

func (handler *UserHandler) GetUserByToken(ctx *fiber.Ctx) error {
	token := ctx.Get("Authorization")[7:]
	if token == "" {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing Authorization header",
		})
	}

	userResponse, err := handler.UserUsecase.GetUserByID(ctx.Context(), token)
	if err != nil {
		handler.Logger.WithError(err).Errorln("Failed to get user by token")
		return err
	}

	response := res.CommonResponse[res.UserResponse]{
		Message:    "Successfully To Get User By ID",
		StatusCode: fiber.StatusOK,
		Data:       userResponse,
	}
	return ctx.Status(fiber.StatusOK).JSON(response)
}

func (handler *UserHandler) GetAllUsers(ctx *fiber.Ctx) error {
	userResponses, err := handler.UserUsecase.GetAllUser(ctx.Context())
	if err != nil {
		handler.Logger.WithError(err).Errorln("Failed to get user by token")
		return err
	}

	responses := res.CommonResponse[[]res.UserResponse]{
		Message:    "Successfully To Get All User",
		StatusCode: fiber.StatusOK,
		Data:       userResponses,
	}
	return ctx.Status(fiber.StatusOK).JSON(responses)
}

func (handler *UserHandler) EditUser(ctx *fiber.Ctx) error {
	panic("")
}
