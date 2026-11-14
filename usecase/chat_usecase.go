package usecase

import (
	"context"
	"real-time-chat-app/dto"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
)

// PersonalChatResult is the outcome of starting (or reusing) a private chat
// together with the first message that was sent into it.
type PersonalChatResult struct {
	Chat     entity.Chat
	Created  bool
	Sender   entity.User
	Receiver entity.User
	Message  dto.BroadcastMessage
}

type ChatUsecase interface {
	CreatePersonalChat(ctx context.Context, senderID, receiverID, content string) (*PersonalChatResult, error)
	IsUserInChat(ctx context.Context, chatID, userID string) (bool, error)
	GetParticipantIDs(ctx context.Context, chatID string) ([]string, error)

	CreateGroupChat(ctx context.Context, name string, creatorID string, memberIDs []string) (*entity.Chat, error)
	FindChatByID(ctx context.Context, chatID string) (*entity.Chat, error)
	GetChatsByUser(ctx context.Context, token string) ([]res.ChatResponse, error)
	GetMessagesByChatID(ctx context.Context, token string, chatId string) ([]res.MessageResponse, error)
}
