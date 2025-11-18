package usecase

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/entity"
	"real-time-chat-app/enum"
	"time"
)

type messageUsecase struct {
	db          *gorm.DB
	chatUsecase ChatUsecase
	log         *logger.AppLogger
}

func NewMessageUsecase(db *gorm.DB, chatUC ChatUsecase, logger *logger.AppLogger) MessageUsecase {
	logger.Http.Info.Info().Msg("Message usecase initialized")
	return &messageUsecase{
		db:          db,
		chatUsecase: chatUC,
		log:         logger,
	}
}

func (uc *messageUsecase) EnsureChat(ctx context.Context, chatID, senderID, receiverID string) (string, error) {
	uc.log.Http.Info.Info().
		Str("chatId", chatID).
		Str("senderId", senderID).
		Str("receiverId", receiverID).
		Msg("EnsureChat started")

	// Create new personal chat if chatID is empty
	if chatID == "" && receiverID != "" {
		uc.log.Http.Trace.Trace().
			Str("senderId", senderID).
			Str("receiverId", receiverID).
			Msg("Creating or finding personal chat")

		chat, err := uc.chatUsecase.EnsurePersonalChat(ctx, senderID, receiverID)
		if err != nil {
			uc.log.Http.Error.Error().
				Err(err).
				Str("senderId", senderID).
				Str("receiverId", receiverID).
				Msg("Failed to ensure personal chat")
			return "", err
		}

		uc.log.Http.Info.Info().
			Str("chatId", chat.ID).
			Str("senderId", senderID).
			Str("receiverId", receiverID).
			Msg("Personal chat ensured")

		return chat.ID, nil
	}

	// Verify existing chat
	if chatID != "" {
		uc.log.Http.Trace.Trace().
			Str("chatId", chatID).
			Msg("Verifying existing chat")

		_, err := uc.chatUsecase.FindChatByID(ctx, uc.db, chatID)
		if err != nil {
			uc.log.Http.Error.Error().
				Err(err).
				Str("chatId", chatID).
				Msg("Chat not found or error occurred")
			return "", err
		}

		uc.log.Http.Trace.Trace().
			Str("chatId", chatID).
			Msg("Chat verified")

		return chatID, nil
	}

	uc.log.Http.Warning.Warn().
		Str("chatId", chatID).
		Str("receiverId", receiverID).
		Msg("Invalid chat session - both chatId and receiverId are empty")

	return "", fmt.Errorf("invalid chat session")
}

func (uc *messageUsecase) ProcessIncomingMessage(ctx context.Context, payload req.MessageRequest) (dto.BroadcastMessage, error) {
	uc.log.Http.Info.Info().
		Str("senderId", payload.SenderID).
		Str("chatId", payload.ChatID).
		Int("contentLength", len(payload.Content)).
		Msg("ProcessIncomingMessage started")

	uc.log.Http.Trace.Trace().
		Str("senderId", payload.SenderID).
		Msg("Fetching sender information")

	// Get sender info
	var sender entity.User
	if err := uc.db.Where("id = ?", payload.SenderID).First(&sender).Error; err != nil {
		uc.log.Http.Error.Error().
			Err(err).
			Str("senderId", payload.SenderID).
			Msg("Failed to find sender")
		return dto.BroadcastMessage{}, fmt.Errorf("sender not found: %w", err)
	}

	uc.log.Http.Trace.Trace().
		Str("senderId", payload.SenderID).
		Str("chatId", payload.ChatID).
		Msg("Creating message entity")

	// Create message
	message := entity.Messages{
		Content:  payload.Content,
		ChatId:   payload.ChatID,
		SenderId: payload.SenderID,
		Status:   enum.MessageStatusSent,
	}

	if err := uc.db.Create(&message).Error; err != nil {
		uc.log.Http.Error.Error().
			Err(err).
			Str("senderId", payload.SenderID).
			Str("chatId", payload.ChatID).
			Msg("Failed to create message")
		return dto.BroadcastMessage{}, fmt.Errorf("failed to create message: %w", err)
	}

	uc.log.Http.Info.Info().
		Str("messageId", message.ID).
		Str("chatId", payload.ChatID).
		Str("senderId", payload.SenderID).
		Msg("Message created successfully")

	uc.log.Http.Trace.Trace().
		Str("chatId", payload.ChatID).
		Msg("Fetching chat participants")

	// Get participants
	var participants []entity.ChatParticipant
	if err := uc.db.Where("chat_id = ?", payload.ChatID).Find(&participants).Error; err != nil {
		uc.log.Http.Error.Error().
			Err(err).
			Str("chatId", payload.ChatID).
			Msg("Failed to get chat participants")
		return dto.BroadcastMessage{}, fmt.Errorf("failed to get participants: %w", err)
	}

	uc.log.Http.Trace.Trace().
		Str("messageId", message.ID).
		Int("participantCount", len(participants)).
		Msg("Creating message status for participants")

	// Create message status for all participants except sender
	statusCount := 0
	for _, p := range participants {
		if p.UserID != payload.SenderID {
			status := entity.MessageStatus{
				IsRead:    false,
				MessageID: message.ID,
				UserID:    p.UserID,
			}
			if err := uc.db.Create(&status).Error; err != nil {
				uc.log.Http.Warning.Warn().
					Err(err).
					Str("userId", p.UserID).
					Str("messageId", message.ID).
					Msg("Failed to create message status for participant")
			} else {
				statusCount++
			}
		}
	}

	uc.log.Http.Trace.Trace().
		Str("messageId", message.ID).
		Int("statusCreated", statusCount).
		Msg("Message status created for participants")

	// Prepare broadcast message
	broadcastMsg := dto.BroadcastMessage{
		MessageID:    message.ID,
		ChatID:       payload.ChatID,
		SenderID:     payload.SenderID,
		SenderName:   sender.Name,
		SenderAvatar: sender.Avatar,
		Status:       string(message.Status),
		Content:      payload.Content,
		CreatedAt:    message.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	uc.log.Http.Info.Info().
		Str("messageId", message.ID).
		Str("chatId", payload.ChatID).
		Str("senderId", payload.SenderID).
		Msg("Message processed successfully")

	return broadcastMsg, nil
}

func (uc *messageUsecase) MarkMessagesAsRead(ctx context.Context, chatID, userID string) error {
	uc.log.Http.Info.Info().
		Str("chatId", chatID).
		Str("userId", userID).
		Msg("MarkMessagesAsRead started")

	uc.log.Http.Trace.Trace().
		Str("chatId", chatID).
		Msg("Fetching messages in chat")

	// Get all messages in chat
	var messages []entity.Messages
	if err := uc.db.Where("chat_id = ?", chatID).Find(&messages).Error; err != nil {
		uc.log.Http.Error.Error().
			Err(err).
			Str("chatId", chatID).
			Msg("Failed to fetch messages")
		return err
	}

	if len(messages) == 0 {
		uc.log.Http.Trace.Trace().
			Str("chatId", chatID).
			Msg("No messages found in chat")
		return nil
	}

	messageIDs := make([]string, 0, len(messages))
	for _, m := range messages {
		messageIDs = append(messageIDs, m.ID)
	}

	uc.log.Http.Trace.Trace().
		Str("chatId", chatID).
		Str("userId", userID).
		Int("messageCount", len(messageIDs)).
		Msg("Marking message status as read")

	// Update message status for user
	result := uc.db.Model(&entity.MessageStatus{}).
		Where("message_id IN ? AND user_id = ? AND is_read = false", messageIDs, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": time.Now(),
		})

	if result.Error != nil {
		uc.log.Http.Error.Error().
			Err(result.Error).
			Str("chatId", chatID).
			Str("userId", userID).
			Msg("Failed to update message status")
		return result.Error
	}

	uc.log.Http.Info.Info().
		Str("chatId", chatID).
		Str("userId", userID).
		Int64("updatedCount", result.RowsAffected).
		Msg("Message status updated")

	uc.log.Http.Trace.Trace().
		Str("chatId", chatID).
		Msg("Checking if all recipients have read messages")

	// Update message status to "read" if all recipients have read
	updatedMessages := 0
	for _, msgID := range messageIDs {
		var unreadCount int64
		uc.db.Model(&entity.MessageStatus{}).
			Where("message_id = ? AND is_read = false", msgID).
			Count(&unreadCount)

		if unreadCount == 0 {
			if err := uc.db.Model(&entity.Messages{}).
				Where("id = ?", msgID).
				Update("status", enum.MessageStatusRead).Error; err != nil {
				uc.log.Http.Warning.Warn().
					Err(err).
					Str("messageId", msgID).
					Msg("Failed to update message status to read")
			} else {
				updatedMessages++
			}
		}
	}

	if updatedMessages > 0 {
		uc.log.Http.Trace.Trace().
			Str("chatId", chatID).
			Int("updatedMessages", updatedMessages).
			Msg("Updated message status to 'read'")
	}

	uc.log.Http.Info.Info().
		Str("chatId", chatID).
		Str("userId", userID).
		Msg("Successfully marked messages as read")

	return nil
}
