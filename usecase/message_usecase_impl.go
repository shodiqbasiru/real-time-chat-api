package usecase

import (
	"context"
	"errors"
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
	db  *gorm.DB
	log *logger.AppLogger
}

func NewMessageUsecase(db *gorm.DB, logger *logger.AppLogger) MessageUsecase {
	logger.Http.Info.Info().Msg("Message usecase initialized")
	return &messageUsecase{
		db:  db,
		log: logger,
	}
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
		CreatedAt:    message.CreatedAt.Format(time.RFC3339),
	}

	uc.log.Http.Info.Info().
		Str("messageId", message.ID).
		Str("chatId", payload.ChatID).
		Str("senderId", payload.SenderID).
		Msg("Message processed successfully")

	return broadcastMsg, nil
}

func (uc *messageUsecase) MarkDelivered(ctx context.Context, messageID, userID string) (*DeliveryReceipt, error) {
	var message entity.Messages
	if err := uc.db.WithContext(ctx).Where("id = ?", messageID).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	result := uc.db.WithContext(ctx).
		Model(&entity.MessageStatus{}).
		Where("message_id = ? AND user_id = ? AND is_delivered = false", messageID, userID).
		Updates(map[string]interface{}{
			"is_delivered": true,
			"delivered_at": time.Now(),
		})
	if result.Error != nil {
		uc.log.Http.Error.Error().Err(result.Error).Str("messageId", messageID).Msg("Failed to mark message as delivered")
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}

	uc.promoteDelivered(ctx, []string{messageID})
	return &DeliveryReceipt{MessageID: message.ID, ChatID: message.ChatId, SenderID: message.SenderId}, nil
}

func (uc *messageUsecase) MarkPendingDelivered(ctx context.Context, userID string) ([]DeliveryReceipt, error) {
	now := time.Now()
	var receipts []DeliveryReceipt
	err := uc.db.WithContext(ctx).Raw(`
		UPDATE t_message_status ms
		SET is_delivered = true, delivered_at = ?, updated_at = ?
		FROM t_messages m
		WHERE m.id = ms.message_id
		  AND ms.user_id = ?
		  AND ms.is_delivered = false
		  AND ms.deleted_at IS NULL
		RETURNING m.id AS message_id, m.chat_id AS chat_id, m.sender_id AS sender_id`,
		now, now, userID,
	).Scan(&receipts).Error
	if err != nil {
		uc.log.Http.Error.Error().Err(err).Str("userId", userID).Msg("Failed to mark pending messages as delivered")
		return nil, err
	}

	if len(receipts) > 0 {
		ids := make([]string, 0, len(receipts))
		for _, r := range receipts {
			ids = append(ids, r.MessageID)
		}
		uc.promoteDelivered(ctx, ids)
		uc.log.Http.Info.Info().Str("userId", userID).Int("count", len(receipts)).Msg("Pending messages marked as delivered")
	}
	return receipts, nil
}

// promoteDelivered sets a message's overall status to delivered once every
// recipient has received it.
func (uc *messageUsecase) promoteDelivered(ctx context.Context, messageIDs []string) {
	err := uc.db.WithContext(ctx).Exec(`
		UPDATE t_messages SET status = ?, delivered_at = ?, updated_at = ?
		WHERE id IN ?
		  AND status = ?
		  AND NOT EXISTS (
		    SELECT 1 FROM t_message_status ms
		    WHERE ms.message_id = t_messages.id AND ms.is_delivered = false AND ms.deleted_at IS NULL
		  )`,
		enum.MessageStatusDelivered, time.Now(), time.Now(), messageIDs, enum.MessageStatusSent,
	).Error
	if err != nil {
		uc.log.Http.Warning.Warn().Err(err).Msg("Failed to promote message status to delivered")
	}
}

func (uc *messageUsecase) MarkMessagesAsRead(ctx context.Context, chatID, userID string) (int64, error) {
	uc.log.Http.Info.Info().
		Str("chatId", chatID).
		Str("userId", userID).
		Msg("MarkMessagesAsRead started")

	var participants int64
	if err := uc.db.WithContext(ctx).
		Model(&entity.ChatParticipant{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Count(&participants).Error; err != nil {
		return 0, err
	}
	if participants == 0 {
		return 0, ErrNotParticipant
	}

	now := time.Now()
	result := uc.db.WithContext(ctx).Exec(`
		UPDATE t_message_status ms
		SET is_read = true, read_at = ?, is_delivered = true,
		    delivered_at = COALESCE(ms.delivered_at, ?), updated_at = ?
		FROM t_messages m
		WHERE m.id = ms.message_id
		  AND m.chat_id = ?
		  AND ms.user_id = ?
		  AND ms.is_read = false
		  AND ms.deleted_at IS NULL`,
		now, now, now, chatID, userID,
	)
	if result.Error != nil {
		uc.log.Http.Error.Error().
			Err(result.Error).
			Str("chatId", chatID).
			Str("userId", userID).
			Msg("Failed to update message status")
		return 0, result.Error
	}

	if result.RowsAffected > 0 {
		// A message is "read" once every recipient has read it.
		err := uc.db.WithContext(ctx).Exec(`
			UPDATE t_messages
			SET status = ?, read_at = ?, delivered_at = COALESCE(delivered_at, ?), updated_at = ?
			WHERE chat_id = ?
			  AND sender_id <> ?
			  AND status <> ?
			  AND deleted_at IS NULL
			  AND NOT EXISTS (
			    SELECT 1 FROM t_message_status ms
			    WHERE ms.message_id = t_messages.id AND ms.is_read = false AND ms.deleted_at IS NULL
			  )`,
			enum.MessageStatusRead, now, now, now, chatID, userID, enum.MessageStatusRead,
		).Error
		if err != nil {
			uc.log.Http.Warning.Warn().Err(err).Str("chatId", chatID).Msg("Failed to update message status to read")
		}
	}

	uc.log.Http.Info.Info().
		Str("chatId", chatID).
		Str("userId", userID).
		Int64("updatedCount", result.RowsAffected).
		Msg("Messages marked as read")

	return result.RowsAffected, nil
}
