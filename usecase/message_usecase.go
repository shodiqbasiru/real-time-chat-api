package usecase

import (
	"context"
	"real-time-chat-app/dto"
	"real-time-chat-app/dto/req"
)

type MessageUsecase interface {
	EnsureChat(ctx context.Context, chatID, senderID, receiverID string) (string, error)
	ProcessIncomingMessage(ctx context.Context, payload req.MessageRequest) (dto.BroadcastMessage, error)
	MarkMessagesAsRead(ctx context.Context, chatID, userID string) error
}
