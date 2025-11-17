package handler

import (
	"context"
	"github.com/gofiber/contrib/websocket"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/entity"
	"real-time-chat-app/usecase"
	"sync"
)

type BroadcastMessage struct {
	Type       string `json:"type"`
	UserID     string `json:"userId"`
	ReceiverID string `json:"receiverId"`
	ChatID     string `json:"chatId"`
	Content    string `json:"content"`
}

type WebSocketHandler struct {
	DB        *gorm.DB
	Logger    *logrus.Logger
	ChatUC    usecase.ChatUsecase
	MessageUC usecase.MessageUsecase
	Clients   map[string]*websocket.Conn            // userId -> conn
	Rooms     map[string]map[string]*websocket.Conn // chatId -> map[userId]*conn
	Mutex     sync.RWMutex
}

func NewWebSocketHandler(db *gorm.DB, logger *logrus.Logger, chatUC usecase.ChatUsecase, messageUC usecase.MessageUsecase) *WebSocketHandler {
	return &WebSocketHandler{
		DB:        db,
		Logger:    logger,
		ChatUC:    chatUC,
		MessageUC: messageUC,
		Clients:   make(map[string]*websocket.Conn),
		Rooms:     make(map[string]map[string]*websocket.Conn),
	}
}

func (handler *WebSocketHandler) HandleWebSocket(c *websocket.Conn) {
	ctx := context.Background()

	userID := c.Query("userId")

	if userID == "" {
		handler.Logger.Error("WebSocket connection rejected: missing userId")
		c.WriteJSON(map[string]string{
			"type":  "error",
			"error": "userId is required",
		})
		c.Close()
		return
	}

	handler.registerClient(userID, c)
	defer func() {
		handler.removeClient(userID, c)
		c.Close()
	}()

	_ = c.WriteJSON(map[string]string{
		"type":   "connected",
		"userId": userID,
	})

	for {
		var msg BroadcastMessage
		if err := c.ReadJSON(&msg); err != nil {
			handler.Logger.Warnf("User %s disconnected: %v", userID, err)
			break
		}

		if msg.Type == "" {
			handler.sendError(c, "message type is required")
			continue
		}

		switch msg.Type {
		case "join_room":
			handler.handleJoinRoom(ctx, c, userID, msg)
		case "leave_room":
			handler.handleLeaveRoom(userID, msg.ChatID)
		case "send_message":
			handler.handleSendMessage(ctx, userID, msg)
		case "typing":
			handler.handleTyping(userID, msg.ChatID, true)
		case "stop_typing":
			handler.handleTyping(userID, msg.ChatID, false)
		default:
			handler.Logger.Warnf("Unknown message type: %s", msg.Type)
			handler.sendError(c, "unknown message type: "+msg.Type)
		}
	}
}

func (handler *WebSocketHandler) handleJoinRoom(ctx context.Context, c *websocket.Conn, userID string, msg BroadcastMessage) {
	chatID, isNewChat, err := handler.joinRoom(ctx, userID, msg.ReceiverID, msg.ChatID)
	if err != nil {
		handler.Logger.Errorf("Join room failed for user %s: %v", userID, err)
		handler.sendError(c, "failed to join room: "+err.Error())
		return
	}

	_ = c.WriteJSON(map[string]string{
		"type":   "joined_room",
		"chatId": chatID,
	})

	// IMPROVEMENT: Broadcast new_chat ke receiver jika chat baru
	if isNewChat && msg.ReceiverID != "" {
		handler.notifyNewChat(ctx, chatID, userID, msg.ReceiverID)
	}
}

func (handler *WebSocketHandler) handleLeaveRoom(userID, chatID string) {
	if chatID == "" {
		handler.Logger.Warn("Leave room failed: chatId is empty")
		return
	}
	handler.leaveRoom(userID, chatID)
}

func (handler *WebSocketHandler) joinRoom(ctx context.Context, userID, receiverID, chatID string) (string, bool, error) {
	var err error
	isNewChat := false

	// Use MessageUsecase untuk ensure chat
	chatID, err = handler.MessageUC.EnsureChat(ctx, chatID, userID, receiverID)
	if err != nil {
		return "", false, err
	}

	// Check if this is a new chat (just created)
	if chatID != "" && receiverID != "" {
		// Cek apakah chat baru dibuat (belum ada messages)
		var messageCount int64
		handler.DB.Model(&entity.Messages{}).Where("chat_id = ?", chatID).Count(&messageCount)
		isNewChat = messageCount == 0
	}

	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	if handler.Rooms[chatID] == nil {
		handler.Rooms[chatID] = make(map[string]*websocket.Conn)
	}
	handler.Rooms[chatID][userID] = handler.Clients[userID]
	handler.Logger.Infof("User %s joined chat room %s", userID, chatID)

	return chatID, isNewChat, nil
}

func (handler *WebSocketHandler) leaveRoom(userID, chatID string) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	if room, ok := handler.Rooms[chatID]; ok {
		delete(room, userID)
		handler.Logger.Infof("User %s left chat room %s", userID, chatID)

		if len(room) == 0 {
			delete(handler.Rooms, chatID)
			handler.Logger.Infof("Chat room %s deleted (empty)", chatID)
		}
	}
}

func (handler *WebSocketHandler) handleSendMessage(ctx context.Context, senderID string, msg BroadcastMessage) {
	if msg.ChatID == "" {
		handler.Logger.Error("Send message failed: chatId is empty")
		handler.sendErrorToUser(senderID, "chatId is required")
		return
	}

	if msg.Content == "" {
		handler.Logger.Error("Send message failed: content is empty")
		handler.sendErrorToUser(senderID, "message content cannot be empty")
		return
	}

	msgRequest := req.MessageRequest{
		SenderID:   senderID,
		ReceiverID: msg.ReceiverID,
		ChatID:     msg.ChatID,
		Content:    msg.Content,
	}

	broadcastMsg, err := handler.MessageUC.ProcessIncomingMessage(ctx, msgRequest)
	if err != nil {
		handler.Logger.Errorf("ProcessIncomingMessage failed: %v", err)
		handler.sendErrorToUser(senderID, "failed to send message")
		return
	}

	handler.Logger.Infof("Message created: ID=%s, ChatID=%s, Sender=%s",
		broadcastMsg.MessageID, broadcastMsg.ChatID, broadcastMsg.SenderID)

	// Broadcast message ke semua participants di room
	handler.broadcastToRoom(broadcastMsg.ChatID, map[string]interface{}{
		"type":         "new_message",
		"messageId":    broadcastMsg.MessageID,
		"chatId":       broadcastMsg.ChatID,
		"senderId":     broadcastMsg.SenderID,
		"senderName":   broadcastMsg.SenderName,
		"senderAvatar": broadcastMsg.SenderAvatar,
		"content":      broadcastMsg.Content,
		"status":       broadcastMsg.Status,
		"createdAt":    broadcastMsg.CreatedAt,
	})

	// 🔥 IMPROVEMENT: Notify receiver tentang chat baru jika mereka belum join room
	handler.notifyOfflineParticipants(ctx, broadcastMsg.ChatID, senderID)
}

func (handler *WebSocketHandler) broadcastToRoom(chatID string, message interface{}) {
	handler.Mutex.RLock()
	room, exists := handler.Rooms[chatID]
	handler.Mutex.RUnlock()

	if !exists {
		handler.Logger.Warnf("Cannot broadcast: room %s not found", chatID)
		return
	}

	for userID, conn := range room {
		if err := conn.WriteJSON(message); err != nil {
			handler.Logger.Errorf("Failed to send message to user %s: %v", userID, err)
		}
	}

	handler.Logger.Infof("Message broadcasted to %d users in room %s", len(room), chatID)
}

// 🔥 NEW: Notify receiver tentang chat baru
func (handler *WebSocketHandler) notifyNewChat(ctx context.Context, chatID, senderID, receiverID string) {
	handler.Logger.Infof("Notifying new chat: chatId=%s, sender=%s, receiver=%s", chatID, senderID, receiverID)

	// Get sender info
	var sender entity.User
	if err := handler.DB.Where("id = ?", senderID).First(&sender).Error; err != nil {
		handler.Logger.Errorf("Failed to get sender info: %v", err)
		return
	}

	// Get chat info
	chat, err := handler.ChatUC.FindChatByID(ctx, handler.DB, chatID)
	if err != nil {
		handler.Logger.Errorf("Failed to get chat info: %v", err)
		return
	}

	// Prepare new_chat notification
	notification := map[string]interface{}{
		"type":         "new_chat",
		"chatId":       chatID,
		"chatUsername": sender.Name,
		"chatAvatar":   sender.Avatar,
		"chatType":     string(chat.ChatType),
		"lastMessage":  "", // Belum ada message
		"unreadCount":  0,
	}

	// Send ke receiver jika online
	handler.sendToUser(receiverID, notification)
}

// 🔥 NEW: Notify participants yang belum join room (offline atau di chat lain)
func (handler *WebSocketHandler) notifyOfflineParticipants(ctx context.Context, chatID, senderID string) {
	// Get all participants
	var participants []entity.ChatParticipant
	if err := handler.DB.Where("chat_id = ?", chatID).Find(&participants).Error; err != nil {
		handler.Logger.Errorf("Failed to get participants: %v", err)
		return
	}

	// Get sender info
	var sender entity.User
	if err := handler.DB.Where("id = ?", senderID).First(&sender).Error; err != nil {
		handler.Logger.Errorf("Failed to get sender info: %v", err)
		return
	}

	// Get last message
	var lastMessage entity.Messages
	if err := handler.DB.Where("chat_id = ?", chatID).
		Order("created_at DESC").
		First(&lastMessage).Error; err != nil {
		return
	}

	handler.Mutex.RLock()
	room := handler.Rooms[chatID]
	handler.Mutex.RUnlock()

	// Notify participants yang tidak ada di room (belum join atau offline)
	for _, p := range participants {
		if p.UserID == senderID {
			continue // Skip sender
		}

		// Check if user is in room
		if room != nil {
			if _, inRoom := room[p.UserID]; inRoom {
				continue // Skip if already in room (sudah dapat broadcast)
			}
		}

		// Get unread count for this user
		var unreadCount int64
		handler.DB.Model(&entity.MessageStatus{}).
			Joins("JOIN t_messages ON t_messages.id = t_message_status.message_id").
			Where("t_message_status.user_id = ? AND t_messages.chat_id = ? AND t_message_status.is_read = false",
				p.UserID, chatID).
			Count(&unreadCount)

		// Send chat_update notification
		notification := map[string]interface{}{
			"type":            "chat_update",
			"chatId":          chatID,
			"chatUsername":    sender.Name,
			"chatAvatar":      sender.Avatar,
			"lastMessage":     lastMessage.Content,
			"lastMessageTime": lastMessage.CreatedAt.Format("2006-01-02 15:04:05"),
			"unreadCount":     unreadCount,
		}

		handler.sendToUser(p.UserID, notification)
	}
}

// 🔥 NEW: Send message ke specific user
func (handler *WebSocketHandler) sendToUser(userID string, message interface{}) {
	handler.Mutex.RLock()
	conn, exists := handler.Clients[userID]
	handler.Mutex.RUnlock()

	if !exists {
		handler.Logger.Debugf("User %s is offline, cannot send notification", userID)
		return
	}

	if err := conn.WriteJSON(message); err != nil {
		handler.Logger.Errorf("Failed to send to user %s: %v", userID, err)
	} else {
		handler.Logger.Infof("Notification sent to user %s", userID)
	}
}

func (handler *WebSocketHandler) handleTyping(userID, chatID string, isTyping bool) {
	if chatID == "" {
		return
	}

	handler.Mutex.RLock()
	room, exists := handler.Rooms[chatID]
	handler.Mutex.RUnlock()

	if !exists {
		return
	}

	typingMsg := map[string]interface{}{
		"type":     "typing",
		"chatId":   chatID,
		"userId":   userID,
		"isTyping": isTyping,
	}

	for uid, conn := range room {
		if uid != userID {
			_ = conn.WriteJSON(typingMsg)
		}
	}
}

func (handler *WebSocketHandler) sendErrorToUser(userID, errorMsg string) {
	handler.Mutex.RLock()
	conn, exists := handler.Clients[userID]
	handler.Mutex.RUnlock()

	if exists {
		_ = conn.WriteJSON(map[string]string{
			"type":  "error",
			"error": errorMsg,
		})
	}
}

func (handler *WebSocketHandler) sendError(c *websocket.Conn, errorMsg string) {
	_ = c.WriteJSON(map[string]string{
		"type":  "error",
		"error": errorMsg,
	})
}

func (handler *WebSocketHandler) registerClient(userID string, conn *websocket.Conn) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()
	handler.Clients[userID] = conn
	handler.Logger.Infof("User connected: %s (Total: %d)", userID, len(handler.Clients))
}

func (handler *WebSocketHandler) removeClient(userID string, conn *websocket.Conn) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	delete(handler.Clients, userID)

	for chatID, room := range handler.Rooms {
		if _, exists := room[userID]; exists {
			delete(room, userID)
			handler.Logger.Infof("User %s removed from room %s", userID, chatID)

			if len(room) == 0 {
				delete(handler.Rooms, chatID)
			}
		}
	}

	handler.Logger.Infof("User disconnected: %s (Remaining: %d)", userID, len(handler.Clients))
}
