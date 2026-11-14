package usecase

import (
	"context"
	"real-time-chat-app/dto"
	"real-time-chat-app/dto/req"
)

// DeliveryReceipt tells a sender that one of their messages reached a recipient.
type DeliveryReceipt struct {
	MessageID string
	ChatID    string
	SenderID  string
}

type MessageUsecase interface {
	ProcessIncomingMessage(ctx context.Context, payload req.MessageRequest) (dto.BroadcastMessage, error)
	// MarkDelivered records that userID received messageID. It returns nil when
	// there was nothing to update (unknown message, not a recipient, or already
	// delivered).
	MarkDelivered(ctx context.Context, messageID, userID string) (*DeliveryReceipt, error)
	// MarkPendingDelivered marks everything sent to userID while they were
	// offline as delivered.
	MarkPendingDelivered(ctx context.Context, userID string) ([]DeliveryReceipt, error)
	// MarkMessagesAsRead marks every message in the chat as read by userID and
	// returns how many were newly read.
	MarkMessagesAsRead(ctx context.Context, chatID, userID string) (int64, error)
}
