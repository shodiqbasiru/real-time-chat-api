package res

type MessageResponse struct {
	MessageId  string `json:"messageId"`
	Content    string `json:"content"`
	SenderId   string `json:"senderId"`
	SenderName string `json:"senderName"`
	CreatedAt  string `json:"createdAt"`
	Status     string `json:"status"`
	IsRead     bool   `json:"isRead"`
}
