package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/security"
	"real-time-chat-app/usecase"
)

type ChatHandler struct {
	usecase.ChatUsecase
	usecase.MessageUsecase
	*logrus.Logger
	*security.JWT
}

func NewChatHandler(chatUsecase usecase.ChatUsecase, messageUsecase usecase.MessageUsecase, logger *logrus.Logger, JWT *security.JWT) *ChatHandler {
	return &ChatHandler{ChatUsecase: chatUsecase, MessageUsecase: messageUsecase, Logger: logger, JWT: JWT}
}

func (handler *ChatHandler) GetAllChat(c *fiber.Ctx) error {
	// get token from header
	token := c.Get("Authorization")[7:]

	chatResponses, err := handler.ChatUsecase.GetChatsByUser(c.Context(), token)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	responses := res.CommonResponse[[]res.ChatResponse]{
		Message:    "Successfully to Get All Chats",
		StatusCode: fiber.StatusOK,
		Data:       chatResponses,
	}

	return c.Status(fiber.StatusOK).JSON(responses)
}

func (handler *ChatHandler) GetMessagesByID(c *fiber.Ctx) error {
	ctx := c.Context()
	chatId := c.Params("chatId")
	token := c.Get("Authorization")[7:]

	if chatId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "chatId is required",
		})
	}

	messages, err := handler.ChatUsecase.GetMessagesByChatID(ctx, token, chatId)
	if err != nil {
		handler.Logger.WithError(err).Error("Failed to get messages by chat ID")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"chatId":   chatId,
		"messages": messages,
	})

}

func (handler *ChatHandler) MarkMessagesAsRead(c *fiber.Ctx) error {
	ctx := c.Context()
	chatId := c.Params("chatId")
	token := c.Get("Authorization")[7:]

	userID, err := handler.JWT.GetUserIdFromToken(token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid token",
		})
	}

	err = handler.MessageUsecase.MarkMessagesAsRead(ctx, chatId, userID)
	if err != nil {
		handler.Logger.WithError(err).Error("Failed to mark messages as read")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"chatId": chatId,
		"status": "messages marked as read",
	})
}
