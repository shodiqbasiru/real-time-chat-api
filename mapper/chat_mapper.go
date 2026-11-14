package mapper

import (
	"real-time-chat-app/dto"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
)

// MapperCreateChatResponse describes a private chat from the point of view of
// the user who is NOT `other`: `other` is the person they are chatting with.
func MapperCreateChatResponse(chat entity.Chat, other entity.User, BCMessage dto.BroadcastMessage) *res.CreateChatResponse {
	return &res.CreateChatResponse{
		ChatId:          chat.ID,
		ReceiverId:      other.ID,
		ChatUsername:    other.Name,
		ChatAvatar:      other.Avatar,
		ChatType:        string(chat.ChatType),
		LastMessage:     BCMessage.Content,
		LastMessageTime: BCMessage.CreatedAt,
		UnreadCount:     0,
	}
}
