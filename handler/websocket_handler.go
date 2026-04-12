package handler

import (
	"context"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/entity"
	bcType "real-time-chat-app/enum/websocket_event"
	"real-time-chat-app/usecase"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"gorm.io/gorm"
)

type BroadcastMessage struct {
	Type       bcType.BCEventType `json:"type"`
	UserID     string             `json:"userId"`
	ReceiverID string             `json:"receiverId"`
	ChatID     string             `json:"chatId"`
	MessageID  string             `json:"messageId"`
	Content    string             `json:"content"`
}

type WebSocketHandler struct {
	DB        *gorm.DB
	Log       *logger.AppLogger
	ChatUC    usecase.ChatUsecase
	MessageUC usecase.MessageUsecase

	Clients map[string]*websocket.Conn
	Rooms   map[string]map[string]*websocket.Conn
	Mutex   sync.RWMutex
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

func (handler *WebSocketHandler) HandleWebSocket(conn *websocket.Conn) {
	ctx := context.Background()
	userID := conn.Query("userId")

	handler.Log.WS.Stream.Info().
		Str("userId", userID).
		Str("remoteAddr", conn.RemoteAddr().String()).
		Msg("WebSocket connection attempt")

	if userID == "" {
		handler.Log.WS.Error.Error().Msg("WebSocket connection rejected: missing userId")
		conn.WriteJSON(map[string]string{
			"type":  string(bcType.ERROR),
			"error": "userId required",
		})
		conn.Close()
		return
	}

	// Register client and set user online
	handler.registerClient(userID, conn)

	defer func() {
		handler.removeClient(userID)
		handler.onDisConnect(userID)
		conn.Close()
	}()

	// Send connection confirmation
	conn.WriteJSON(map[string]string{
		"type":   string(bcType.CONNECTED),
		"userId": userID,
	})

	handler.onConnect(userID, conn)

	handler.Log.WS.Stream.Info().Str("userId", userID).Msg("WebSocket connection established")
	for {
		var msg BroadcastMessage
		if err := conn.ReadJSON(&msg); err != nil {
			handler.Log.WS.Warning.Warn().Str("userId", userID).Err(err).Msg("User disconnected or read error")
			break
		}

		handler.Log.WS.Stream.Info().
			Str("userId", userID).
			Str("messageType", string(msg.Type)).
			Msg("Incoming WebSocket message")

		if msg.Type == "" {
			handler.sendError(conn, "message type is required")
			continue
		}

		switch msg.Type {
		case bcType.CREATE_CHAT:
			handler.handleCreateChat(ctx, conn, userID, msg)

		case bcType.JOIN_ROOM:
			handler.handleJoinRoom(ctx, conn, userID, msg)

		case bcType.LEAVE_ROOM:
			handler.handleLeaveRoom(userID, msg.ChatID)

		case bcType.SEND_MESSAGE:
			handler.handleSendMessage(ctx, userID, msg)

		case bcType.MESSAGE_DELIVERED:
			handler.handleMessageDelivered(userID, msg.MessageID, msg.ChatID)

		case bcType.TYPING:
			// handler.broadcastTyping(userID, msg.ChatID, true)

		case bcType.STOP_TYPING:
			// handler.broadcastTyping(userID, msg.ChatID, false)

		default:
			handler.Log.WS.Warning.Warn().Str("messageType", string(msg.Type)).Msg("Unknown message type")
			handler.sendError(conn, "unknown message type: "+string(msg.Type))
		}
	}
}

func (handler *WebSocketHandler) handleCreateChat(ctx context.Context, c *websocket.Conn, senderID string, msg BroadcastMessage) {
	if msg.ReceiverID == "" || msg.Content == "" {
		handler.sendError(c, "receiverId and content required")
		return
	}

	createResponse, err := handler.ChatUC.CreatePersonalChat(ctx, senderID, msg.ReceiverID, msg.Content)
	if err != nil {
		handler.sendError(c, err.Error())
		return
	}

	handler.sendToUser(senderID, map[string]interface{}{
		"type":         bcType.CHAT_CREATED,
		"chatId":       createResponse.ChatId,
		"chatUsername": createResponse.ChatUsername,
		"chatAvatar":   createResponse.ChatAvatar,
		"receiverId":   createResponse.ReceiverId,
		"unreadCount":  createResponse.UnreadCount,
	})
}

func (handler *WebSocketHandler) handleJoinRoom(ctx context.Context, c *websocket.Conn, userID string, msg BroadcastMessage) {
	handler.Log.WS.Info.Info().
		Str("userId", userID).
		Str("chatId", msg.ChatID).
		Str("receiverId", msg.ReceiverID).
		Msg("Processing join_room")

	if msg.ChatID == "" {
		handler.sendError(c, "chatId is required")
		return
	}

	ok, err := handler.ChatUC.IsUserInChat(ctx, msg.ChatID, userID)
	if err != nil || !ok {
		handler.Log.WS.Error.Error().Err(err).Msg("Failed to join room")
		handler.sendError(c, "failed to join room: "+err.Error())
		return
	}

	handler.joinRoom(userID, msg.ChatID)
	c.WriteJSON(map[string]interface{}{
		"type":   bcType.JOINED_ROOM,
		"chatId": msg.ChatID,
	})

}

func (handler *WebSocketHandler) handleLeaveRoom(userID, chatID string) {
	if chatID == "" {
		return
	}
	handler.Log.WS.Info.Info().Str("userId", userID).Str("chatId", chatID).Msg("Processing leave_room")
	handler.leaveRoom(userID, chatID)
}

func (handler *WebSocketHandler) joinRoom(userID, chatID string) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	if handler.Rooms[chatID] == nil {
		handler.Rooms[chatID] = make(map[string]*websocket.Conn)
	}
	handler.Rooms[chatID][userID] = handler.Clients[userID]
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

	handler.Mutex.RLock()
	room, exists := handler.Rooms[msg.ChatID]
	_, joined := room[senderID]
	handler.Mutex.RUnlock()

	if !exists || !joined {
		handler.sendErrorToUser(senderID, "join room first")
		return
	}

	broadcast, err := handler.MessageUC.ProcessIncomingMessage(ctx,
		req.MessageRequest{
			ChatID:   msg.ChatID,
			SenderID: senderID,
			Content:  msg.Content,
		},
	)

	if err != nil {
		handler.Log.WS.Error.Error().
			Err(err).
			Str("chatId", msg.ChatID).
			Str("senderId", senderID).
			Msg("failed to process incoming message")

		handler.sendErrorToUser(senderID, "failed to send message")
		return
	}

	handler.broadcastToRoom(msg.ChatID, map[string]interface{}{
		"type":         "new_message",
		"chatId":       broadcast.ChatID,
		"messageId":    broadcast.MessageID,
		"senderId":     broadcast.SenderID,
		"senderName":   broadcast.SenderName,
		"senderAvatar": broadcast.SenderAvatar,
		"content":      broadcast.Content,
		"status":       broadcast.Status,
		"createdAt":    broadcast.CreatedAt,
	})
}

func (handler *WebSocketHandler) onConnect(userId string, conn *websocket.Conn) {
	handler.Mutex.RLock()
	defer handler.Mutex.RUnlock()
	handler.Clients[userId] = conn

	handler.broadcastUserStatus(userId, true)
}

func (handler *WebSocketHandler) onDisConnect(userId string) {
	handler.Mutex.RLock()
	defer handler.Mutex.RUnlock()
	delete(handler.Clients, userId)

	handler.broadcastUserStatus(userId, false)
}

func (handler *WebSocketHandler) broadcastUserStatus(userID string, isOnline bool) {
	statusMsg := map[string]interface{}{
		"type":     bcType.USER_STATUS,
		"userId":   userID,
		"isOnline": isOnline,
	}

	for uid, conn := range handler.Clients {
		if uid != userID {
			conn.WriteJSON(statusMsg)
		}
	}

	handler.Log.WS.Info.Info().
		Str("userId", userID).
		Bool("isOnline", isOnline).
		Int("broadcastCount", len(handler.Clients)-1).
		Msg("User status broadcasted")
}

func (handler *WebSocketHandler) notifyNewChatToReceiver(ctx context.Context, chatId, senderId, receiverId, firstMessage string) {

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

// NEW: Auto mark message as delivered for online users in room
func (handler *WebSocketHandler) autoMarkAsDelivered(chatID, messageID, senderID string) {
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
func (handler *WebSocketHandler) markMessagesAsDelivered(chatID, userID string) {
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
func (handler *WebSocketHandler) handleMessageDelivered(userID, messageID, chatID string) {
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
func (handler *WebSocketHandler) handleMessageRead(userID, messageID, chatID string) {
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

func (handler *WebSocketHandler) removeClient(userID string) {
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
