package repository

import (
	"context"
	"errors"
	"real-time-chat-app/entity"
	"real-time-chat-app/enum"

	"gorm.io/gorm"
)

type ChatRepository struct {
	Repository[entity.Chat]
}

func NewChatRepository() *ChatRepository {
	return &ChatRepository{}
}

// FindPersonalChat returns the private chat between two users, or nil when
// they have none. If duplicates exist from older versions, the oldest wins so
// every caller agrees on the same chat.
func (repository ChatRepository) FindPersonalChat(ctx context.Context, db *gorm.DB, userAID, userBID string) (*entity.Chat, error) {
	var chat entity.Chat
	err := db.WithContext(ctx).
		Joins("JOIN t_chat_participant cp1 ON cp1.chat_id = t_chat.id AND cp1.user_id = ?", userAID).
		Joins("JOIN t_chat_participant cp2 ON cp2.chat_id = t_chat.id AND cp2.user_id = ?", userBID).
		Where("t_chat.chat_type = ?", enum.PRIVATE).
		Order("t_chat.created_at ASC").
		First(&chat).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &chat, nil
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

// FindParticipantIDs returns the user IDs of everyone in a chat.
func (repository ChatRepository) FindParticipantIDs(ctx context.Context, db *gorm.DB, chatID string) ([]string, error) {
	var ids []string
	err := db.WithContext(ctx).
		Model(&entity.ChatParticipant{}).
		Where("chat_id = ?", chatID).
		Pluck("user_id", &ids).Error
	return ids, err
}
