package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/usecase"
)

type ChatHandler struct {
	usecase.ChatUsecase
	*logrus.Logger
}

func NewChatHandler(chatUsecase usecase.ChatUsecase, logger *logrus.Logger) *ChatHandler {
	return &ChatHandler{
		ChatUsecase: chatUsecase,
		Logger:      logger,
	}
}

// GetAllChat godoc
// @Summary Get chat messages by chat ID
// @Description Retrieve all messages from a chat room
// @Tags Chat
// @Produce json
// @Param chatId path string true "Chat ID"
// @Success 200 {array} entity.Messages
// @Failure 500 {object} fiber.Map
// @Router /api/v1/chat/{chatId}/messages [get]
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
