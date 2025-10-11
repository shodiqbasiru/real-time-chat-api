package repository

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"real-time-chat-app/entity"
)

type ChatRepository struct {
	Repository[entity.Chat]
}

func NewChatRepository() *ChatRepository {
	return &ChatRepository{}
}

func (repository ChatRepository) FindPersonalChat(ctx context.Context, db *gorm.DB, userAID, userBID string) (*entity.Chat, error) {
	var chat entity.Chat
	err := db.WithContext(ctx).
		Joins("JOIN t_chat_participant cp1 ON cp1.chat_id = t_chat.id").
		Joins("JOIN t_chat_participant cp2 ON cp2.chat_id = t_chat.id").
		Where("cp1.user_id = ? AND cp2.user_id = ? AND t_chat.chat_type = ?", userAID, userBID, "personal").
		First(&chat).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &chat, err
}

func (repository ChatRepository) FindChatByID(ctx context.Context, db *gorm.DB, id string) (*entity.Chat, error) {
	var chat entity.Chat
	err := db.WithContext(ctx).Where("id = ?", id).First(&chat).Error
	if err != nil {
		return nil, err
	}
	return &chat, nil
}

func (repository ChatRepository) CreateChatWithParticipants(ctx context.Context, db *gorm.DB, chat *entity.Chat, participants []entity.ChatParticipant) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(chat).Error; err != nil {
			return err
		}
		for i := range participants {
			participants[i].ChatID = chat.ID
		}
		return tx.Create(&participants).Error
	})
}

func (repository ChatRepository) FindAllByUserID(ctx context.Context, db *gorm.DB, userID string) ([]entity.Chat, error) {
	var chats []entity.Chat

	err := db.WithContext(ctx).
		Model(&entity.Chat{}).
		Joins("JOIN t_chat_participant cp ON cp.chat_id = t_chat.id").
		Where("cp.user_id = ?", userID).
		Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(1)
		}).
		Preload("Participants").
		Find(&chats).Error

	if err != nil {
		return nil, err
	}

	return chats, nil
}

func (repository ChatRepository) IsUserInChat(ctx context.Context, db *gorm.DB, chatId, userId string) (bool, error) {
	var count int64
	err := db.WithContext(ctx).
		Model(&entity.ChatParticipant{}).
		Where("chat_id = ? AND user_id = ?", chatId, userId).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (repository ChatRepository) FindMessagesByChatID(ctx context.Context, db *gorm.DB, chatId string) ([]entity.Messages, error) {
	var messages []entity.Messages
	err := db.WithContext(ctx).
		Preload("Sender").
		Where("chat_id = ?", chatId).
		Order("created_at ASC").
		Find(&messages).Error
	return messages, err
}
