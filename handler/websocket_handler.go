package handler

import (
	"context"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2/log"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/entity"
	"real-time-chat-app/usecase"
	"sync"
)

type WebSocketHandler struct {
	*gorm.DB
	*logrus.Logger
	sync.Mutex
	usecase.ChatUsecase
	Clients   map[string]map[*websocket.Conn]bool // chatId -> list of clients
	Broadcast chan BroadcastMessage
}

type BroadcastMessage struct {
	MessageID    string `json:"messageId"`
	ChatID       string `json:"chatId"`
	SenderID     string `json:"senderId"`
	SenderName   string `json:"senderName"`
	SenderAvatar string `json:"sender_avatar,omitempty"`
	Content      string `json:"content"`
	CreatedAt    string `json:"createdAt"`
}

func NewWebSocketHandler(db *gorm.DB, logger *logrus.Logger, chatUsecase usecase.ChatUsecase) *WebSocketHandler {
	handler := &WebSocketHandler{
		DB:          db,
		Logger:      logger,
		Clients:     make(map[string]map[*websocket.Conn]bool),
		Broadcast:   make(chan BroadcastMessage),
		ChatUsecase: chatUsecase,
	}
	go handler.runBroadcast()
	return handler
}

func (handler *WebSocketHandler) HandleWebSocket(c *websocket.Conn) {
	ctx := context.Background()

	chatID := c.Query("chatId")         // dari query param, kalau ada
	senderID := c.Query("senderId")     // id user yang sedang login
	receiverID := c.Query("receiverId") // opsional, untuk personal chat baru

	var chat *entity.Chat
	var err error

	// Kalau chatId belum dikirim → berarti user baru memulai chat
	if chatID == "" && receiverID != "" {
		chat, err = handler.ChatUsecase.EnsurePersonalChat(ctx, senderID, receiverID)
		if err != nil {
			handler.Logger.Errorf("Failed to ensure personal chat: %v", err)
			c.Close()
			return
		}
		chatID = chat.ID
		handler.Logger.Infof("New personal chat created: %s", chatID)
	} else if chatID != "" {
		// Kalau sudah ada chatId, ambil datanya untuk validasi
		_, err := handler.ChatUsecase.FindChatByID(ctx, handler.DB, chatID)
		if err != nil {
			handler.Logger.Errorf("Failed to find chat: %v", err)
			c.Close()
			return
		}
		handler.Logger.Infof("Joined existing chat: %s", chatID)
	} else {
		handler.Logger.Warn("Invalid connection request: missing chatId or receiverId")
		c.Close()
		return
	}

	// Daftarkan koneksi ke map[chatID]
	handler.registerClient(chatID, c)
	defer func() {
		handler.removeClient(chatID, c)
		c.Close()
	}()

	for {
		var payload req.MessageRequest
		log.Tracef("Payload <=========> %v", payload)
		err := c.ReadJSON(&payload)
		if err != nil {
			handler.Logger.Warnf("Read error: %v", err)
			break
		}

		payload.ChatID = chatID

		// Ambil informasi user pengirim dari database
		var sender entity.User
		if err := handler.DB.Where("id = ?", payload.SenderID).First(&sender).Error; err != nil {
			handler.Logger.Errorf("Failed to get sender info: %v", err)
			continue
		}

		// Simpan ke database
		message := entity.Messages{
			Content:  payload.Content,
			ChatId:   chatID,
			SenderId: payload.SenderID,
		}
		if err := handler.DB.Create(&message).Error; err != nil {
			handler.Logger.Errorf("Failed to save message: %v", err)
			continue
		}

		// Broadcast dengan informasi lengkap termasuk senderName
		broadcastMsg := BroadcastMessage{
			MessageID:    message.ID,
			ChatID:       chatID,
			SenderID:     payload.SenderID,
			SenderName:   sender.Name,
			SenderAvatar: sender.Avatar,
			Content:      payload.Content,
			CreatedAt:    message.CreatedAt.Format("2006-01-02 15:04:05"),
		}

		handler.Logger.Infof("Broadcasting message: %+v", broadcastMsg)
		handler.Broadcast <- broadcastMsg
	}

}

func (handler *WebSocketHandler) registerClient(chatID string, conn *websocket.Conn) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	if handler.Clients[chatID] == nil {
		handler.Clients[chatID] = make(map[*websocket.Conn]bool)
	}
	handler.Clients[chatID][conn] = true
	handler.Logger.Infof("Client joined chat room: %s (Total: %d)", chatID, len(handler.Clients[chatID]))
}

func (handler *WebSocketHandler) removeClient(chatID string, conn *websocket.Conn) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	if clients, ok := handler.Clients[chatID]; ok {
		delete(clients, conn)
		if len(clients) == 0 {
			delete(handler.Clients, chatID)
		}
	}
	handler.Logger.Infof("Client left chat room: %s", chatID)
}

func (handler *WebSocketHandler) runBroadcast() {
	for {
		msg := <-handler.Broadcast
		handler.Mutex.Lock()
		clients := handler.Clients[msg.ChatID]
		for conn := range clients {
			if err := conn.WriteJSON(msg); err != nil {
				handler.Logger.Warnf("Error broadcasting message: %v", err)
				conn.Close()
				delete(clients, conn)
			}
		}
		handler.Mutex.Unlock()
	}
}
