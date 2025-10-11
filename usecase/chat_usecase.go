package usecase

import (
	"context"
	"gorm.io/gorm"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
)

type ChatUsecase interface {
	EnsurePersonalChat(ctx context.Context, userAID, userBID string) (*entity.Chat, error)
	CreateGroupChat(ctx context.Context, name string, creatorID string, memberIDs []string) (*entity.Chat, error)
	FindChatByID(ctx context.Context, db *gorm.DB, chatID string) (*entity.Chat, error)
	GetChatsByUser(ctx context.Context, token string) ([]res.ChatResponse, error)
	GetMessagesByChatID(ctx context.Context, token string, chatId string) ([]res.MessageResponse, error)
}
