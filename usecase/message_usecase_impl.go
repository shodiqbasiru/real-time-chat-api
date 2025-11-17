package usecase

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"real-time-chat-app/dto"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/entity"
	"real-time-chat-app/enum"
	"time"
)

type messageUsecase struct {
	db          *gorm.DB
	chatUsecase ChatUsecase
}

func NewMessageUsecase(db *gorm.DB, chatUC ChatUsecase) MessageUsecase {
	return &messageUsecase{db: db, chatUsecase: chatUC}
}

func (uc *messageUsecase) EnsureChat(ctx context.Context, chatID, senderID, receiverID string) (string, error) {
	if chatID == "" && receiverID != "" {
		chat, err := uc.chatUsecase.EnsurePersonalChat(ctx, senderID, receiverID)
		if err != nil {
			return "", err
		}
		return chat.ID, nil
	}

	if chatID != "" {
		_, err := uc.chatUsecase.FindChatByID(ctx, uc.db, chatID)
		if err != nil {
			return "", err
		}
		return chatID, nil
	}

	return "", fmt.Errorf("invalid chat session")
}

func (uc *messageUsecase) ProcessIncomingMessage(ctx context.Context, payload req.MessageRequest) (dto.BroadcastMessage, error) {
	var sender entity.User
	if err := uc.db.Where("id = ?", payload.SenderID).First(&sender).Error; err != nil {
		return dto.BroadcastMessage{}, err
	}

	message := entity.Messages{
		Content:  payload.Content,
		ChatId:   payload.ChatID,
		SenderId: payload.SenderID,
		Status:   enum.MessageStatusSent, // status delivered to receiver
	}

	if err := uc.db.Create(&message).Error; err != nil {
		return dto.BroadcastMessage{}, err
	}

	// get participant chat
	var participants []entity.ChatParticipant
	if err := uc.db.Where("chat_id = ?", payload.ChatID).Find(&participants).Error; err != nil {
		return dto.BroadcastMessage{}, err
	}

	// Buat MessageStatus untuk semua peserta selain pengirim
	for _, p := range participants {
		if p.UserID != payload.SenderID {
			status := entity.MessageStatus{
				IsRead:    false,
				MessageID: message.ID,
				UserID:    p.UserID,
			}
			uc.db.Create(&status)
		}
	}

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
	return broadcastMsg, nil
}

func (uc *messageUsecase) MarkMessagesAsRead(ctx context.Context, chatID, userID string) error {
	var messages []entity.Messages
	if err := uc.db.Where("chat_id = ?", chatID).Find(&messages).Error; err != nil {
		return err
	}

	messageIDs := make([]string, 0)
	for _, m := range messages {
		messageIDs = append(messageIDs, m.ID)
	}

	if len(messageIDs) > 0 {
		if err := uc.db.Model(&entity.MessageStatus{}).
			Where("message_id IN ? AND user_id = ? AND is_read = false", messageIDs, userID).
			Updates(map[string]interface{}{
				"is_read": true,
				"read_at": time.Now(),
			}).Error; err != nil {
			return err
		}

		//  Update status Messages ke "read" jika semua receiver sudah baca
		for _, msgID := range messageIDs {
			var unreadCount int64
			uc.db.Model(&entity.MessageStatus{}).
				Where("message_id = ? AND is_read = false", msgID).
				Count(&unreadCount)

			if unreadCount == 0 {
				uc.db.Model(&entity.Messages{}).
					Where("id = ?", msgID).
					Update("status", enum.MessageStatusRead)
			}
		}

	}
	return nil
}
