package handler

import (
	"github.com/gofiber/fiber/v2"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/security"
	"real-time-chat-app/usecase"
)

type ChatHandler struct {
	usecase.ChatUsecase
	usecase.MessageUsecase
	Log *logger.AppLogger
	*security.JWT
}

func NewChatHandler(chatUsecase usecase.ChatUsecase, messageUsecase usecase.MessageUsecase, logger *logger.AppLogger, JWT *security.JWT) *ChatHandler {
	return &ChatHandler{ChatUsecase: chatUsecase, MessageUsecase: messageUsecase, Log: logger, JWT: JWT}
}

func (handler *ChatHandler) GetAllChat(c *fiber.Ctx) error {
	handler.Log.Http.Stream.Info().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("ip", c.IP()).
		Msg("Incoming request: Get all chats")

	// get token from header
	token := c.Get("Authorization")[7:]

	handler.Log.Http.Info.Info().
		Str("path", c.Path()).
		Msg("Processing get all chats request")

	chatResponses, err := handler.ChatUsecase.GetChatsByUser(c.Context(), token)
	if err != nil {
		handler.Log.Http.Error.Error().
			Err(err).
			Str("path", c.Path()).
			Msg("Failed to get chats")

		handler.Log.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusInternalServerError).
			Msg("Response: Failed to get chats")

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve chats",
		})
	}

	responses := res.CommonResponse[[]res.ChatResponse]{
		Message:    "Successfully to Get All Chats",
		StatusCode: fiber.StatusOK,
		Data:       chatResponses,
	}

	handler.Log.Http.Info.Info().
		Int("chatCount", len(chatResponses)).
		Msg("Successfully retrieved all chats")

	handler.Log.Http.Stream.Info().
		Int("statusCode", fiber.StatusOK).
		Int("chatCount", len(chatResponses)).
		Msg("Response: Successfully retrieved all chats")

	return c.Status(fiber.StatusOK).JSON(responses)
}

func (handler *ChatHandler) GetMessagesByID(c *fiber.Ctx) error {
	chatId := c.Params("chatId")

	handler.Log.Http.Stream.Info().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("chatId", chatId).
		Str("ip", c.IP()).
		Msg("Incoming request: Get messages by chat ID")

	if chatId == "" {
		handler.Log.Http.Warning.Warn().
			Str("path", c.Path()).
			Msg("Request missing chatId parameter")

		handler.Log.Http.Stream.Error().
			Int("statusCode", fiber.StatusBadRequest).
			Msg("Response: Bad request - chatId is required")

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "chatId is required",
		})
	}

	token := c.Get("Authorization")[7:]

	handler.Log.Http.Info.Info().
		Str("chatId", chatId).
		Msg("Processing get messages request")

	ctx := c.Context()
	messages, err := handler.ChatUsecase.GetMessagesByChatID(ctx, token, chatId)
	if err != nil {
		handler.Log.Http.Error.Error().
			Err(err).
			Str("chatId", chatId).
			Msg("Failed to get messages by chat ID")

		handler.Log.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusInternalServerError).
			Str("chatId", chatId).
			Msg("Response: Failed to get messages")

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	handler.Log.Http.Info.Info().
		Str("chatId", chatId).
		Int("messageCount", len(messages)).
		Msg("Successfully retrieved messages")

	handler.Log.Http.Stream.Info().
		Int("statusCode", fiber.StatusOK).
		Str("chatId", chatId).
		Int("messageCount", len(messages)).
		Msg("Response: Successfully retrieved messages")

	return c.JSON(fiber.Map{
		"chatId":   chatId,
		"messages": messages,
	})
}

func (handler *ChatHandler) MarkMessagesAsRead(c *fiber.Ctx) error {
	chatId := c.Params("chatId")

	handler.Log.Http.Stream.Info().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("chatId", chatId).
		Str("ip", c.IP()).
		Msg("Incoming request: Mark messages as read")

	token := c.Get("Authorization")[7:]

	userID, err := handler.JWT.GetUserIdFromToken(token)
	if err != nil {
		handler.Log.Http.Error.Error().
			Err(err).
			Str("chatId", chatId).
			Msg("Invalid token - failed to extract user ID")

		handler.Log.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusUnauthorized).
			Msg("Response: Unauthorized - invalid token")

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid token",
		})
	}

	handler.Log.Http.Info.Info().
		Str("chatId", chatId).
		Str("userId", userID).
		Msg("Processing mark messages as read")

	ctx := c.Context()
	err = handler.MessageUsecase.MarkMessagesAsRead(ctx, chatId, userID)
	if err != nil {
		handler.Log.Http.Error.Error().
			Err(err).
			Str("chatId", chatId).
			Str("userId", userID).
			Msg("Failed to mark messages as read")

		handler.Log.Http.Stream.Error().
			Err(err).
			Int("statusCode", fiber.StatusInternalServerError).
			Str("chatId", chatId).
			Msg("Response: Failed to mark messages as read")

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	handler.Log.Http.Info.Info().
		Str("chatId", chatId).
		Str("userId", userID).
		Msg("Messages marked as read successfully")

	handler.Log.Http.Stream.Info().
		Int("statusCode", fiber.StatusOK).
		Str("chatId", chatId).
		Str("userId", userID).
		Msg("Response: Messages marked as read")

	return c.JSON(fiber.Map{
		"chatId": chatId,
		"status": "messages marked as read",
	})
}
