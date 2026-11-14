package res

type CreateChatResponse struct {
	ChatId          string `json:"chatId"`
	ReceiverId      string `json:"receiverId"`
	ChatUsername    string `json:"chatUsername"`
	ChatAvatar      string `json:"chatAvatar"`
	ChatType        string `json:"chatType"`
	LastMessage     string `json:"lastMessage"`
	LastMessageTime string `json:"lastMessageTime"`
	UnreadCount     uint   `json:"unreadCount"`
}
