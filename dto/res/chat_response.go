package res

type ChatResponse struct {
	ChatId          string `json:"chatId"`
	ChatUsername    string `json:"chatUsername"`
	LastMessageTime string `json:"lastMessageTime"`
}
