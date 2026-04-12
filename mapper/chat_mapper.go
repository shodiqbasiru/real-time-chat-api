package mapper

import (
	"real-time-chat-app/dto"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
)

func MapperCreateChatResponse(chat entity.Chat, receiver entity.User, BCMessage dto.BroadcastMessage) *res.CreateChatResponse {
	return &res.CreateChatResponse{
		ChatId:       chat.ID,
		ReceiverId:   receiver.ID,
		ChatUsername: receiver.Name,
		ChatAvatar:   receiver.Avatar,
		ChatType:     "PRIVATE",
		LastMessage:  BCMessage.Content,
		UnreadCount:  0,
	}
}
