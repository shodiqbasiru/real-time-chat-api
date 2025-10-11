package usecase

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
	"real-time-chat-app/enum"
	"real-time-chat-app/repository"
	"real-time-chat-app/security"
)

type ChatUsecaseImpl struct {
	*repository.ChatRepository
	*logrus.Logger
	*gorm.DB
	*security.JWT
}

func NewChatUsecase(chatRepository *repository.ChatRepository, logger *logrus.Logger, DB *gorm.DB, JWT *security.JWT) *ChatUsecaseImpl {
	return &ChatUsecaseImpl{ChatRepository: chatRepository, Logger: logger, DB: DB, JWT: JWT}
}

func (uc *ChatUsecaseImpl) EnsurePersonalChat(ctx context.Context, userAID, userBID string) (*entity.Chat, error) {
	existingChat, err := uc.ChatRepository.FindPersonalChat(ctx, uc.DB, userAID, userBID)
	if err != nil {
		return nil, err
	}
	if existingChat != nil {
		return existingChat, nil
	}

	newChat := &entity.Chat{
		ChatType: enum.PRIVATE,
	}

	participants := []entity.ChatParticipant{
		{UserID: userAID},
		{UserID: userBID},
	}

	if err := uc.ChatRepository.CreateChatWithParticipants(ctx, uc.DB, newChat, participants); err != nil {
		return nil, err
	}

	return newChat, nil
}

func (uc *ChatUsecaseImpl) CreateGroupChat(ctx context.Context, name string, creatorID string, memberIDs []string) (*entity.Chat, error) {
	newChat := &entity.Chat{
		ChatType:  enum.GROUP,
		GroupName: name,
	}

	participants := make([]entity.ChatParticipant, 0, len(memberIDs)+1)
	participants = append(participants, entity.ChatParticipant{UserID: creatorID})
	for _, id := range memberIDs {
		participants = append(participants, entity.ChatParticipant{UserID: id})
	}

	if err := uc.ChatRepository.CreateChatWithParticipants(ctx, uc.DB, newChat, participants); err != nil {
		return nil, err
	}

	return newChat, nil
}

func (uc *ChatUsecaseImpl) FindChatByID(ctx context.Context, db *gorm.DB, chatID string) (*entity.Chat, error) {
	return uc.ChatRepository.FindChatByID(ctx, db, chatID)
}

func (uc *ChatUsecaseImpl) GetChatsByUser(ctx context.Context, token string) ([]res.ChatResponse, error) {
	// 1. Extract user ID dari token
	userId, err := uc.JWT.GetUserIdFromToken(token)
	if err != nil {
		uc.Logger.WithError(err).Error("Failed to extract user ID from token")
		return nil, err
	}

	// 2. Ambil semua chat milik user ini
	chats, err := uc.ChatRepository.FindAllByUserID(ctx, uc.DB, userId)
	if err != nil {
		uc.Logger.WithError(err).Error("Failed to get chats by user ID")
		return nil, err
	}

	var chatResponses []res.ChatResponse

	for _, chat := range chats {
		var chatUsername string

		// 3. Tentukan nama chat berdasarkan tipe
		if chat.ChatType == enum.PRIVATE {
			// Cari participant lain selain user ini
			for _, participant := range chat.Participants {
				if participant.UserID != userId {
					var otherUser entity.User
					if err := uc.DB.First(&otherUser, "id = ?", participant.UserID).Error; err == nil {
						chatUsername = otherUser.Name
					}
					break
				}
			}
		} else if chat.ChatType == enum.GROUP {
			chatUsername = chat.GroupName
		}

		// 4. Ambil pesan terakhir (opsional)
		var lastMessage entity.Messages
		if err := uc.DB.Where("chat_id = ?", chat.ID).
			Order("created_at DESC").
			First(&lastMessage).Error; err != nil {
			// kalau tidak ada pesan, abaikan error
		}

		// 5. Bangun response
		chatResponses = append(chatResponses, res.ChatResponse{
			ChatId:          chat.ID,
			ChatUsername:    chatUsername,
			LastMessageTime: lastMessage.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return chatResponses, nil
}

func (uc *ChatUsecaseImpl) GetMessagesByChatID(ctx context.Context, token string, chatId string) ([]res.MessageResponse, error) {
	// Ambil userId dari token untuk validasi
	userId, err := uc.JWT.GetUserIdFromToken(token)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Pastikan user termasuk dalam chat ini (security check)
	isParticipant, err := uc.ChatRepository.IsUserInChat(ctx, uc.DB, chatId, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to verify participant: %w", err)
	}
	if !isParticipant {
		return nil, fmt.Errorf("user not authorized for this chat")
	}

	// Ambil semua messages berdasarkan chatId
	messages, err := uc.ChatRepository.FindMessagesByChatID(ctx, uc.DB, chatId)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// Map ke response DTO
	var responses []res.MessageResponse
	for _, msg := range messages {
		responses = append(responses, res.MessageResponse{
			MessageId:  msg.ID,
			Content:    msg.Content,
			SenderId:   msg.SenderId,
			SenderName: msg.Sender.Name,
			CreatedAt:  msg.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return responses, nil
}
