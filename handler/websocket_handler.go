package handler

import (
	"context"
	"encoding/json"
	"github.com/gofiber/contrib/websocket"
	"gorm.io/gorm"
	"real-time-chat-app/config/logger"
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
		handler.Log.WS.Error.Error().
			Str("remoteAddr", c.RemoteAddr().String()).
			Msg("WebSocket connection rejected: missing userId")

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

	response := map[string]string{
		"type":   "connected",
		"userId": userID,
	}
	_ = c.WriteJSON(response)

	handler.Log.WS.Stream.Info().
		Str("userId", userID).
		Str("type", "connected").
		Msg("WebSocket connection established")

	// Main message loop
	for {
		var msg BroadcastMessage
		if err := c.ReadJSON(&msg); err != nil {
			handler.Log.WS.Warning.Warn().
				Str("userId", userID).
				Err(err).
				Msg("User disconnected or read error")
			break
		}

		msgJSON, _ := json.Marshal(msg)
		handler.Log.WS.Stream.Info().
			Str("userId", userID).
			Str("messageType", msg.Type).
			Str("payload", string(msgJSON)).
			Msg("Incoming WebSocket message")

		if msg.Type == "" {
			handler.Log.WS.Warning.Warn().
				Str("userId", userID).
				Msg("Received message with empty type")
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
			handler.Log.WS.Warning.Warn().
				Str("userId", userID).
				Str("messageType", msg.Type).
				Msg("Unknown message type received")
			handler.sendError(c, "unknown message type: "+msg.Type)
		}
	}
}

func (handler *WebSocketHandler) handleJoinRoom(ctx context.Context, c *websocket.Conn, userID string, msg BroadcastMessage) {
	handler.Log.WS.Info.Info().
		Str("userId", userID).
		Str("chatId", msg.ChatID).
		Str("receiverId", msg.ReceiverID).
		Msg("Processing join_room request")

	chatID, isNewChat, err := handler.joinRoom(ctx, userID, msg.ReceiverID, msg.ChatID)
	if err != nil {
		handler.Log.WS.Error.Error().
			Str("userId", userID).
			Str("chatId", msg.ChatID).
			Err(err).
			Msg("Failed to join room")
		handler.sendError(c, "failed to join room: "+err.Error())
		return
	}

	response := map[string]string{
		"type":   "joined_room",
		"chatId": chatID,
	}
	_ = c.WriteJSON(response)

	handler.Log.WS.Stream.Info().
		Str("userId", userID).
		Str("chatId", chatID).
		Bool("isNewChat", isNewChat).
		Str("type", "joined_room").
		Msg("Sent joined_room response")

	if isNewChat && msg.ReceiverID != "" {
		handler.Log.WS.Info.Info().
			Str("chatId", chatID).
			Str("senderId", userID).
			Str("receiverId", msg.ReceiverID).
			Msg("New chat created, notifying receiver")
		handler.notifyNewChat(ctx, chatID, userID, msg.ReceiverID)
	}
}

func (handler *WebSocketHandler) handleLeaveRoom(userID, chatID string) {
	if chatID == "" {
		handler.Log.WS.Warning.Warn().
			Str("userId", userID).
			Msg("Leave room failed: chatId is empty")
		return
	}

	handler.Log.WS.Info.Info().
		Str("userId", userID).
		Str("chatId", chatID).
		Msg("Processing leave_room request")

	handler.leaveRoom(userID, chatID)
}

func (handler *WebSocketHandler) joinRoom(ctx context.Context, userID, receiverID, chatID string) (string, bool, error) {
	var err error
	isNewChat := false

	handler.Log.WS.Trace.Trace().
		Str("userId", userID).
		Str("receiverId", receiverID).
		Str("chatId", chatID).
		Msg("Ensuring chat exists")

	chatID, err = handler.MessageUC.EnsureChat(ctx, chatID, userID, receiverID)
	if err != nil {
		handler.Log.WS.Error.Error().
			Str("userId", userID).
			Err(err).
			Msg("Failed to ensure chat")
		return "", false, err
	}

	if chatID != "" && receiverID != "" {
		var messageCount int64
		handler.DB.Model(&entity.Messages{}).Where("chat_id = ?", chatID).Count(&messageCount)
		isNewChat = messageCount == 0

		handler.Log.WS.Trace.Trace().
			Str("chatId", chatID).
			Int64("messageCount", messageCount).
			Bool("isNewChat", isNewChat).
			Msg("Chat status checked")
	}

	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	if handler.Rooms[chatID] == nil {
		handler.Rooms[chatID] = make(map[string]*websocket.Conn)
		handler.Log.WS.Trace.Trace().
			Str("chatId", chatID).
			Msg("Created new room")
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

		handler.Log.WS.Info.Info().
			Str("userId", userID).
			Str("chatId", chatID).
			Int("remainingUsers", len(room)).
			Msg("User left chat room")

		if len(room) == 0 {
			delete(handler.Rooms, chatID)
			handler.Log.WS.Info.Info().
				Str("chatId", chatID).
				Msg("Chat room deleted (empty)")
		}
	} else {
		handler.Log.WS.Warning.Warn().
			Str("userId", userID).
			Str("chatId", chatID).
			Msg("Attempted to leave non-existent room")
	}
}

func (handler *WebSocketHandler) handleSendMessage(ctx context.Context, senderID string, msg BroadcastMessage) {
	handler.Log.WS.Info.Info().
		Str("senderId", senderID).
		Str("chatId", msg.ChatID).
		Int("contentLength", len(msg.Content)).
		Msg("Processing send_message request")

	if msg.ChatID == "" {
		handler.Log.WS.Error.Error().
			Str("senderId", senderID).
			Msg("Send message failed: chatId is empty")
		handler.sendErrorToUser(senderID, "chatId is required")
		return
	}

	if msg.Content == "" {
		handler.Log.WS.Error.Error().
			Str("senderId", senderID).
			Str("chatId", msg.ChatID).
			Msg("Send message failed: content is empty")
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
		handler.Log.WS.Error.Error().
			Str("senderId", senderID).
			Str("chatId", msg.ChatID).
			Err(err).
			Msg("Failed to process incoming message")
		handler.sendErrorToUser(senderID, "failed to send message")
		return
	}

	handler.Log.WS.Info.Info().
		Str("messageId", broadcastMsg.MessageID).
		Str("chatId", broadcastMsg.ChatID).
		Str("senderId", broadcastMsg.SenderID).
		Msg("Message created successfully")

	broadcastPayload := map[string]interface{}{
		"type":         "new_message",
		"messageId":    broadcastMsg.MessageID,
		"chatId":       broadcastMsg.ChatID,
		"senderId":     broadcastMsg.SenderID,
		"senderName":   broadcastMsg.SenderName,
		"senderAvatar": broadcastMsg.SenderAvatar,
		"content":      broadcastMsg.Content,
		"status":       broadcastMsg.Status,
		"createdAt":    broadcastMsg.CreatedAt,
	}

	handler.broadcastToRoom(broadcastMsg.ChatID, broadcastPayload)

	handler.notifyOfflineParticipants(ctx, broadcastMsg.ChatID, senderID)
}

func (handler *WebSocketHandler) broadcastToRoom(chatID string, message interface{}) {
	handler.Mutex.RLock()
	room, exists := handler.Rooms[chatID]
	handler.Mutex.RUnlock()

	if !exists {
		handler.Log.WS.Warning.Warn().
			Str("chatId", chatID).
			Msg("Cannot broadcast: room not found")
		return
	}

	successCount := 0
	failCount := 0

	for userID, conn := range room {
		if err := conn.WriteJSON(message); err != nil {
			handler.Log.WS.Error.Error().
				Str("userId", userID).
				Str("chatId", chatID).
				Err(err).
				Msg("Failed to send message to user")
			failCount++
		} else {
			successCount++
		}
	}

	handler.Log.WS.Stream.Info().
		Str("chatId", chatID).
		Int("successCount", successCount).
		Int("failCount", failCount).
		Int("totalUsers", len(room)).
		Str("messageType", "new_message").
		Msg("Message broadcast completed")
}

func (handler *WebSocketHandler) notifyNewChat(ctx context.Context, chatID, senderID, receiverID string) {
	handler.Log.WS.Trace.Trace().
		Str("chatId", chatID).
		Str("senderId", senderID).
		Str("receiverId", receiverID).
		Msg("Fetching sender and chat info for new_chat notification")

	var sender entity.User
	if err := handler.DB.Where("id = ?", senderID).First(&sender).Error; err != nil {
		handler.Log.WS.Error.Error().
			Str("senderId", senderID).
			Err(err).
			Msg("Failed to get sender info")
		return
	}

	chat, err := handler.ChatUC.FindChatByID(ctx, handler.DB, chatID)
	if err != nil {
		handler.Log.WS.Error.Error().
			Str("chatId", chatID).
			Err(err).
			Msg("Failed to get chat info")
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

	handler.Log.WS.Stream.Info().
		Str("receiverId", receiverID).
		Str("chatId", chatID).
		Str("type", "new_chat").
		Msg("Sent new_chat notification")
}

func (handler *WebSocketHandler) notifyOfflineParticipants(ctx context.Context, chatID, senderID string) {
	handler.Log.WS.Trace.Trace().
		Str("chatId", chatID).
		Str("senderId", senderID).
		Msg("Checking offline participants for notification")

	var participants []entity.ChatParticipant
	if err := handler.DB.Where("chat_id = ?", chatID).Find(&participants).Error; err != nil {
		handler.Log.WS.Error.Error().
			Str("chatId", chatID).
			Err(err).
			Msg("Failed to get participants")
		return
	}

	var sender entity.User
	if err := handler.DB.Where("id = ?", senderID).First(&sender).Error; err != nil {
		handler.Log.WS.Error.Error().
			Str("senderId", senderID).
			Err(err).
			Msg("Failed to get sender info")
		return
	}

	var lastMessage entity.Messages
	if err := handler.DB.Where("chat_id = ?", chatID).
		Order("created_at DESC").
		First(&lastMessage).Error; err != nil {
		handler.Log.WS.Warning.Warn().
			Str("chatId", chatID).
			Err(err).
			Msg("No messages found in chat")
		return
	}

	handler.Mutex.RLock()
	room := handler.Rooms[chatID]
	handler.Mutex.RUnlock()

	offlineCount := 0

	for _, p := range participants {
		if p.UserID == senderID {
			continue
		}

		if room != nil {
			if _, inRoom := room[p.UserID]; inRoom {
				handler.Log.WS.Trace.Trace().
					Str("userId", p.UserID).
					Str("chatId", chatID).
					Msg("User already in room, skipping notification")
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
		offlineCount++
	}

	if offlineCount > 0 {
		handler.Log.WS.Info.Info().
			Str("chatId", chatID).
			Int("notifiedCount", offlineCount).
			Msg("Notified offline participants")
	}
}

func (handler *WebSocketHandler) sendToUser(userID string, message interface{}) {
	handler.Mutex.RLock()
	conn, exists := handler.Clients[userID]
	handler.Mutex.RUnlock()

	if !exists {
		handler.Log.WS.Trace.Trace().
			Str("userId", userID).
			Msg("User is offline, cannot send notification")
		return
	}

	if err := conn.WriteJSON(message); err != nil {
		handler.Log.WS.Error.Error().
			Str("userId", userID).
			Err(err).
			Msg("Failed to send notification to user")
	} else {
		msgType := "unknown"
		if m, ok := message.(map[string]interface{}); ok {
			if t, ok := m["type"].(string); ok {
				msgType = t
			}
		}

		handler.Log.WS.Stream.Info().
			Str("userId", userID).
			Str("messageType", msgType).
			Msg("Notification sent to user")
	}
}

func (handler *WebSocketHandler) handleTyping(userID, chatID string, isTyping bool) {
	if chatID == "" {
		handler.Log.WS.Warning.Warn().
			Str("userId", userID).
			Msg("Typing event failed: chatId is empty")
		return
	}

	handler.Mutex.RLock()
	room, exists := handler.Rooms[chatID]
	handler.Mutex.RUnlock()

	if !exists {
		handler.Log.WS.Warning.Warn().
			Str("userId", userID).
			Str("chatId", chatID).
			Msg("Typing event failed: room not found")
		return
	}

	typingMsg := map[string]interface{}{
		"type":     "typing",
		"chatId":   chatID,
		"userId":   userID,
		"isTyping": isTyping,
	}

	sentCount := 0
	for uid, conn := range room {
		if uid != userID {
			if err := conn.WriteJSON(typingMsg); err == nil {
				sentCount++
			}
		}
	}

	handler.Log.WS.Trace.Trace().
		Str("userId", userID).
		Str("chatId", chatID).
		Bool("isTyping", isTyping).
		Int("sentCount", sentCount).
		Msg("Typing indicator broadcasted")
}

func (handler *WebSocketHandler) sendErrorToUser(userID, errorMsg string) {
	handler.Mutex.RLock()
	conn, exists := handler.Clients[userID]
	handler.Mutex.RUnlock()

	if exists {
		errorPayload := map[string]string{
			"type":  "error",
			"error": errorMsg,
		}
		_ = conn.WriteJSON(errorPayload)

		handler.Log.WS.Stream.Error().
			Str("userId", userID).
			Str("error", errorMsg).
			Str("type", "error").
			Msg("Sent error response to user")
	}
}

func (handler *WebSocketHandler) sendError(c *websocket.Conn, errorMsg string) {
	errorPayload := map[string]string{
		"type":  "error",
		"error": errorMsg,
	}
	_ = c.WriteJSON(errorPayload)

	handler.Log.WS.Stream.Error().
		Str("error", errorMsg).
		Str("type", "error").
		Msg("Sent error response")
}

func (handler *WebSocketHandler) registerClient(userID string, conn *websocket.Conn) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	handler.Clients[userID] = conn

	handler.Log.WS.Info.Info().
		Str("userId", userID).
		Int("totalClients", len(handler.Clients)).
		Str("remoteAddr", conn.RemoteAddr().String()).
		Msg("User registered successfully")
}

func (handler *WebSocketHandler) removeClient(userID string, conn *websocket.Conn) {
	handler.Mutex.Lock()
	defer handler.Mutex.Unlock()

	delete(handler.Clients, userID)

	roomsLeft := 0
	for chatID, room := range handler.Rooms {
		if _, exists := room[userID]; exists {
			delete(room, userID)
			roomsLeft++

			handler.Log.WS.Trace.Trace().
				Str("userId", userID).
				Str("chatId", chatID).
				Msg("User removed from room during disconnect")

			if len(room) == 0 {
				delete(handler.Rooms, chatID)
				handler.Log.WS.Trace.Trace().
					Str("chatId", chatID).
					Msg("Empty room deleted during user disconnect")
			}
		}
	}

	handler.Log.WS.Info.Info().
		Str("userId", userID).
		Int("remainingClients", len(handler.Clients)).
		Int("roomsLeft", roomsLeft).
		Msg("User disconnected and cleaned up")
}
