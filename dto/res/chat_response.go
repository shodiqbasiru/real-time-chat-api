package res

type ChatResponse struct {
	ChatId          string `json:"chatId"`
	ReceiverId      string `json:"receiverId"`
	ChatUsername    string `json:"chatUsername"`
	LastMessage     string `json:"lastMessage"`
	UnreadCount     uint   `json:"unreadCount"`
	LastMessageTime string `json:"lastMessageTime"`
}
