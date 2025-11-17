package dto

type BroadcastMessage struct {
	MessageID    string `json:"messageId"`
	ChatID       string `json:"chatId"`
	SenderID     string `json:"senderId"`
	SenderName   string `json:"senderName"`
	SenderAvatar string `json:"senderAvatar,omitempty"`
	Content      string `json:"content"`
	CreatedAt    string `json:"createdAt"`
	Status       string `json:"status"`
}
