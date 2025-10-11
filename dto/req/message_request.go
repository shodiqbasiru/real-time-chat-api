package req

type MessageRequest struct {
	ChatID     string `json:"chatId"`
	SenderID   string `json:"senderId"`
	ReceiverID string `json:"receiverId,omitempty"`
	Content    string `json:"content"`
	ChatType   string `json:"type,omitempty"` // optional, "personal" | "group"
}
