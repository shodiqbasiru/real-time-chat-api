package usecase

import (
	"context"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
)

type ChatUsecase interface {
	CreatePersonalChat(ctx context.Context, userAID, userBID, content string) (*res.CreateChatResponse, error)
	IsUserInChat(ctx context.Context, chatID, userID string) (bool, error)

	CreateGroupChat(ctx context.Context, name string, creatorID string, memberIDs []string) (*entity.Chat, error)
	FindChatByID(ctx context.Context, chatID string) (*entity.Chat, error)
	GetChatsByUser(ctx context.Context, token string) ([]res.ChatResponse, error)
	GetMessagesByChatID(ctx context.Context, token string, chatId string) ([]res.MessageResponse, error)
}
