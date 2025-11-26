package handler

import (
	"context"
	"github.com/gofiber/contrib/websocket"
	"gorm.io/gorm"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/entity"
	"real-time-chat-app/usecase"
	"sync"
	"time"
)

type BroadcastMessage struct {
	Type       string `json:"type"`
	UserID     string `json:"userId"`
	ReceiverID string `json:"receiverId"`
	ChatID     string `json:"chatId"`
	MessageID  string `json:"messageId"` // NEW: for acknowledgment
	Content    string `json:"content"`
}

type WebSocketHandler struct {
	DB        *gorm.DB
	Log       *logger.AppLogger
	ChatUC    usecase.ChatUsecase
	MessageUC usecase.MessageUsecase
	Clients   map[string]*websocket.Conn            // userId -> conn
	Rooms     map[string]map[string]*websocket.Conn // chatId -> map[userId]*conn
	Mutex     sync.RWMutex
}

func NewWebSocketHandler(db *gorm.DB, logger *logger.AppLogger, chatUC usecase.ChatUsecase, messageUC usecase.MessageUsecase) *WebSocketHandler {
	logger.WS.Info.Info().Msg("WebSocket handler initialized")
	return &WebSocketHandler{
		DB:        db,
		Log:       logger,
		ChatUC:    chatUC,
		MessageUC: messageUC,
		Clients:   make(map[string]*websocket.Conn),
		Rooms:     make(map[string]map[string]*websocket.Conn),
	}
}

func (handler *WebSocketHandler) HandleWebSocket(c *websocket.Conn) {
	ctx := context.Background()
	userID := c.Query("userId")

	handler.Log.WS.Stream.Info().
		Str("userId", userID).
		Str("remoteAddr", c.RemoteAddr().String()).
		Msg("WebSocket connection attempt")

	if userID == "" {
		handler.Log.WS.Error.Error().Msg("WebSocket connection rejected: missing userId")
		c.WriteJSON(map[string]string{"type": "error", "error": "userId is required"})
		c.Close()
		return
	}

	// Register client and set user online
	handler.registerClient(userID, c)
	handler.setUserOnlineStatus(userID, true)

	defer func() {
		handler.setUserOnlineStatus(userID, false)
		handler.removeClient(userID, c)
		c.Close()
	}()

	// Send connection confirmation
	c.WriteJSON(map[string]string{"type": "connected", "userId": userID})
	handler.Log.WS.Stream.Info().Str("userId", userID).Msg("WebSocket connection established")

	// Broadcast user online status
	handler.broadcastUserStatus(userID, true)

	// Main message loop
	for {
		var msg BroadcastMessage
		if err := c.ReadJSON(&msg); err != nil {
			handler.Log.WS.Warning.Warn().Str("userId", userID).Err(err).Msg("User disconnected or read error")
			break
		}

		handler.Log.WS.Stream.Info().
			Str("userId", userID).
			Str("messageType", msg.Type).
			Msg("Incoming WebSocket message")

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
		case "message_delivered":
			handler.handleMessageDelivered(ctx, userID, msg.MessageID, msg.ChatID)
		case "message_read":
			handler.handleMessageRead(ctx, userID, msg.MessageID, msg.ChatID)
		case "typing":
			handler.handleTyping(userID, msg.ChatID, true)
		case "stop_typing":
			handler.handleTyping(userID, msg.ChatID, false)
		default:
			handler.Log.WS.Warning.Warn().Str("messageType", msg.Type).Msg("Unknown message type")
			handler.sendError(c, "unknown message type: "+msg.Type)
		}
	}

	// Broadcast user offline status
	handler.broadcastUserStatus(userID, false)
}

// NEW: Set user online/offline status in database
func (handler *WebSocketHandler) setUserOnlineStatus(userID string, isOnline bool) {
	now := time.Now()
	updates := map[string]interface{}{
		"is_online": isOnline,
		"last_seen": now,
	}

	if err := handler.DB.Model(&entity.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		handler.Log.WS.Error.Error().Err(err).Str("userId", userID).Msg("Failed to update user online status")
	}
}

// NEW: Broadcast user online/offline status to all connected clients
func (handler *WebSocketHandler) broadcastUserStatus(userID string, isOnline bool) {
	handler.Mutex.RLock()
	defer handler.Mutex.RUnlock()

	var user entity.User
	if err := handler.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return
	}

	statusMsg := map[string]interface{}{
		"type":     "user_status",
		"userId":   userID,
		"username": user.Name,
		"isOnline": isOnline,
		"lastSeen": user.LastSeen.Format("2006-01-02 15:04:05"),
	}

	// Broadcast to all connected clients
	for uid, conn := range handler.Clients {
		if uid != userID {
			if err := conn.WriteJSON(statusMsg); err != nil {
				handler.Log.WS.Error.Error().Err(err).Str("userId", uid).Msg("Failed to send user status")
			}
		}
	}

	handler.Log.WS.Info.Info().
		Str("userId", userID).
		Bool("isOnline", isOnline).
		Int("broadcastCount", len(handler.Clients)-1).
		Msg("User status broadcasted")
}

func (handler *WebSocketHandler) handleJoinRoom(ctx context.Context, c *websocket.Conn, userID string, msg BroadcastMessage) {
	handler.Log.WS.Info.Info().
		Str("userId", userID).
		Str("chatId", msg.ChatID).
		Str("receiverId", msg.ReceiverID).
		Msg("Processing join_room request")

	chatID, isNewChat, err := handler.joinRoom(ctx, userID, msg.ReceiverID, msg.ChatID)
	if err != nil {
		handler.Log.WS.Error.Error().Err(err).Msg("Failed to join room")
		handler.sendError(c, "failed to join room: "+err.Error())
		return
	}

	c.WriteJSON(map[string]string{"type": "joined_room", "chatId": chatID})
	handler.Log.WS.Stream.Info().Str("chatId", chatID).Msg("Sent joined_room response")

	// NEW: Mark messages as delivered when user joins room
	handler.markMessagesAsDelivered(ctx, chatID, userID)

	if isNewChat && msg.ReceiverID != "" {
		handler.notifyNewChat(ctx, chatID, userID, msg.ReceiverID)
	}
}

func (handler *WebSocketHandler) handleLeaveRoom(userID, chatID string) {
	if chatID == "" {
		return
	}
	handler.Log.WS.Info.Info().Str("userId", userID).Str("chatId", chatID).Msg("Processing leave_room")
	handler.leaveRoom(userID, chatID)
}

func (handler *WebSocketHandler) joinRoom(ctx context.Context, userID, receiverID, chatID string) (string, bool, error) {
	var err error
	isNewChat := false

	chatID, err = handler.MessageUC.EnsureChat(ctx, chatID, userID, receiverID)
	if err != nil {
		return "", false, err
	}

	if chatID != "" && receiverID != "" {
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

	handler.Log.WS.Info.Info().
		Str("userId", userID).
		Str("chatId", chatID).
		Int("roomSize", len(handler.Rooms[chatID])).
		Msg("User joined chat room")

	return chatID, isNewChat, nil
}

func (handler *WebSocketHandler) leaveRoom(userID, chatID string) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	if room, ok := handler.Rooms[chatID]; ok {
		delete(room, userID)
		handler.Log.WS.Info.Info().Str("userId", userID).Str("chatId", chatID).Msg("User left chat room")

		if len(room) == 0 {
			delete(handler.Rooms, chatID)
		}
	}
}

func (handler *WebSocketHandler) handleSendMessage(ctx context.Context, senderID string, msg BroadcastMessage) {
	if msg.ChatID == "" || msg.Content == "" {
		handler.sendErrorToUser(senderID, "chatId and content are required")
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
		handler.Log.WS.Error.Error().Err(err).Msg("Failed to process message")
		handler.sendErrorToUser(senderID, "failed to send message")
		return
	}

	broadcastPayload := map[string]interface{}{
		"type":         "new_message",
		"messageId":    broadcastMsg.MessageID,
		"chatId":       broadcastMsg.ChatID,
		"senderId":     broadcastMsg.SenderID,
		"senderName":   broadcastMsg.SenderName,
		"senderAvatar": broadcastMsg.SenderAvatar,
		"content":      broadcastMsg.Content,
		"status":       "sent", // Initial status
		"createdAt":    broadcastMsg.CreatedAt,
	}

	handler.broadcastToRoom(broadcastMsg.ChatID, broadcastPayload)

	// NEW: Auto-mark as delivered for users in the room
	handler.autoMarkAsDelivered(ctx, broadcastMsg.ChatID, broadcastMsg.MessageID, senderID)

	handler.notifyOfflineParticipants(ctx, broadcastMsg.ChatID, senderID)
}

// NEW: Auto mark message as delivered for online users in room
func (handler *WebSocketHandler) autoMarkAsDelivered(ctx context.Context, chatID, messageID, senderID string) {
	handler.Mutex.RLock()
	room, exists := handler.Rooms[chatID]
	handler.Mutex.RUnlock()

	if !exists {
		return
	}

	now := time.Now()
	for userID := range room {
		if userID != senderID {
			// Update message status to delivered
			err := handler.DB.Model(&entity.MessageStatus{}).
				Where("message_id = ? AND user_id = ?", messageID, userID).
				Updates(map[string]interface{}{
					"is_delivered": true,
					"delivered_at": now,
				}).Error

			if err != nil {
				handler.Log.WS.Error.Error().Err(err).Msg("Failed to mark as delivered")
				continue
			}

			// Send delivered acknowledgment to sender
			handler.sendToUser(senderID, map[string]interface{}{
				"type":      "message_delivered",
				"messageId": messageID,
				"chatId":    chatID,
				"userId":    userID,
			})
		}
	}

	// Check if all recipients have received the message
	handler.updateMessageStatusIfAll(messageID, "delivered")
}

// NEW: Mark messages as delivered when user joins room
func (handler *WebSocketHandler) markMessagesAsDelivered(ctx context.Context, chatID, userID string) {
	var undeliveredMessages []entity.Messages
	err := handler.DB.Where("chat_id = ? AND sender_id != ?", chatID, userID).
		Find(&undeliveredMessages).Error

	if err != nil {
		return
	}

	now := time.Now()
	for _, msg := range undeliveredMessages {
		// Update message status
		handler.DB.Model(&entity.MessageStatus{}).
			Where("message_id = ? AND user_id = ? AND is_delivered = false", msg.ID, userID).
			Updates(map[string]interface{}{
				"is_delivered": true,
				"delivered_at": now,
			})

		// Notify sender
		handler.sendToUser(msg.SenderId, map[string]interface{}{
			"type":      "message_delivered",
			"messageId": msg.ID,
			"chatId":    chatID,
			"userId":    userID,
		})

		// Update main message status if all delivered
		handler.updateMessageStatusIfAll(msg.ID, "delivered")
	}
}

// NEW: Handle message delivered acknowledgment
func (handler *WebSocketHandler) handleMessageDelivered(ctx context.Context, userID, messageID, chatID string) {
	if messageID == "" {
		return
	}

	now := time.Now()
	err := handler.DB.Model(&entity.MessageStatus{}).
		Where("message_id = ? AND user_id = ?", messageID, userID).
		Updates(map[string]interface{}{
			"is_delivered": true,
			"delivered_at": now,
		}).Error

	if err != nil {
		handler.Log.WS.Error.Error().Err(err).Msg("Failed to update delivered status")
		return
	}

	// Get sender ID
	var message entity.Messages
	if err := handler.DB.Where("id = ?", messageID).First(&message).Error; err != nil {
		return
	}

	// Notify sender
	handler.sendToUser(message.SenderId, map[string]interface{}{
		"type":      "message_delivered",
		"messageId": messageID,
		"chatId":    chatID,
		"userId":    userID,
	})

	handler.updateMessageStatusIfAll(messageID, "delivered")
}

// NEW: Handle message read acknowledgment
func (handler *WebSocketHandler) handleMessageRead(ctx context.Context, userID, messageID, chatID string) {
	if messageID == "" {
		return
	}

	now := time.Now()
	err := handler.DB.Model(&entity.MessageStatus{}).
		Where("message_id = ? AND user_id = ?", messageID, userID).
		Updates(map[string]interface{}{
			"is_delivered": true,
			"delivered_at": now,
			"is_read":      true,
			"read_at":      now,
		}).Error

	if err != nil {
		handler.Log.WS.Error.Error().Err(err).Msg("Failed to update read status")
		return
	}

	// Get sender ID
	var message entity.Messages
	if err := handler.DB.Where("id = ?", messageID).First(&message).Error; err != nil {
		return
	}

	// Notify sender
	handler.sendToUser(message.SenderId, map[string]interface{}{
		"type":      "message_read",
		"messageId": messageID,
		"chatId":    chatID,
		"userId":    userID,
	})

	handler.updateMessageStatusIfAll(messageID, "read")
}

// NEW: Update message status if all recipients have same status
func (handler *WebSocketHandler) updateMessageStatusIfAll(messageID string, status string) {
	var total, count int64
	handler.DB.Model(&entity.MessageStatus{}).Where("message_id = ?", messageID).Count(&total)

	if status == "delivered" {
		handler.DB.Model(&entity.MessageStatus{}).
			Where("message_id = ? AND is_delivered = true", messageID).Count(&count)
	} else if status == "read" {
		handler.DB.Model(&entity.MessageStatus{}).
			Where("message_id = ? AND is_read = true", messageID).Count(&count)
	}

	if total > 0 && count == total {
		now := time.Now()
		updates := map[string]interface{}{"status": status}

		if status == "delivered" {
			updates["delivered_at"] = now
		} else if status == "read" {
			updates["read_at"] = now
		}

		handler.DB.Model(&entity.Messages{}).Where("id = ?", messageID).Updates(updates)
	}
}

func (handler *WebSocketHandler) broadcastToRoom(chatID string, message interface{}) {
	handler.Mutex.RLock()
	room, exists := handler.Rooms[chatID]
	handler.Mutex.RUnlock()

	if !exists {
		return
	}

	for userID, conn := range room {
		if err := conn.WriteJSON(message); err != nil {
			handler.Log.WS.Error.Error().Err(err).Str("userId", userID).Msg("Failed to send message")
		}
	}
}

func (handler *WebSocketHandler) notifyNewChat(ctx context.Context, chatID, senderID, receiverID string) {
	var sender entity.User
	if err := handler.DB.Where("id = ?", senderID).First(&sender).Error; err != nil {
		return
	}

	chat, err := handler.ChatUC.FindChatByID(ctx, handler.DB, chatID)
	if err != nil {
		return
	}

	notification := map[string]interface{}{
		"type":         "new_chat",
		"chatId":       chatID,
		"chatUsername": sender.Name,
		"chatAvatar":   sender.Avatar,
		"chatType":     string(chat.ChatType),
		"lastMessage":  "",
		"unreadCount":  0,
	}

	handler.sendToUser(receiverID, notification)
}

func (handler *WebSocketHandler) notifyOfflineParticipants(ctx context.Context, chatID, senderID string) {
	var participants []entity.ChatParticipant
	if err := handler.DB.Where("chat_id = ?", chatID).Find(&participants).Error; err != nil {
		return
	}

	var sender entity.User
	if err := handler.DB.Where("id = ?", senderID).First(&sender).Error; err != nil {
		return
	}

	var lastMessage entity.Messages
	if err := handler.DB.Where("chat_id = ?", chatID).Order("created_at DESC").First(&lastMessage).Error; err != nil {
		return
	}

	handler.Mutex.RLock()
	room := handler.Rooms[chatID]
	handler.Mutex.RUnlock()

	for _, p := range participants {
		if p.UserID == senderID {
			continue
		}

		if room != nil {
			if _, inRoom := room[p.UserID]; inRoom {
				continue
			}
		}

		var unreadCount int64
		handler.DB.Model(&entity.MessageStatus{}).
			Joins("JOIN t_messages ON t_messages.id = t_message_status.message_id").
			Where("t_message_status.user_id = ? AND t_messages.chat_id = ? AND t_message_status.is_read = false",
				p.UserID, chatID).
			Count(&unreadCount)

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

func (handler *WebSocketHandler) sendToUser(userID string, message interface{}) {
	handler.Mutex.RLock()
	conn, exists := handler.Clients[userID]
	handler.Mutex.RUnlock()

	if !exists {
		return
	}

	if err := conn.WriteJSON(message); err != nil {
		handler.Log.WS.Error.Error().Err(err).Str("userId", userID).Msg("Failed to send to user")
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
			conn.WriteJSON(typingMsg)
		}
	}
}

func (handler *WebSocketHandler) sendErrorToUser(userID, errorMsg string) {
	handler.Mutex.RLock()
	conn, exists := handler.Clients[userID]
	handler.Mutex.RUnlock()

	if exists {
		conn.WriteJSON(map[string]string{"type": "error", "error": errorMsg})
	}
}

func (handler *WebSocketHandler) sendError(c *websocket.Conn, errorMsg string) {
	c.WriteJSON(map[string]string{"type": "error", "error": errorMsg})
}

func (handler *WebSocketHandler) registerClient(userID string, conn *websocket.Conn) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()
	handler.Clients[userID] = conn
	handler.Log.WS.Info.Info().Str("userId", userID).Int("totalClients", len(handler.Clients)).Msg("User registered")
}

func (handler *WebSocketHandler) removeClient(userID string, conn *websocket.Conn) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	delete(handler.Clients, userID)

	for chatID, room := range handler.Rooms {
		if _, exists := room[userID]; exists {
			delete(room, userID)
			if len(room) == 0 {
				delete(handler.Rooms, chatID)
			}
		}
	}

	handler.Log.WS.Info.Info().Str("userId", userID).Msg("User disconnected and cleaned up")
}
