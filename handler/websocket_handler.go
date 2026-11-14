package handler

import (
	"context"
	"encoding/json"
	"errors"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/dto/res"
	bcType "real-time-chat-app/enum/websocket_event"
	"real-time-chat-app/mapper"
	"real-time-chat-app/security"
	"real-time-chat-app/usecase"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

const (
	writeTimeout     = 10 * time.Second
	maxMessageLength = 4000

	// CloseUnauthorized is sent when the token is missing or invalid. Clients
	// should sign in again instead of reconnecting.
	CloseUnauthorized = 4401
)

var errClientClosed = errors.New("websocket client closed")

// RealtimeNotifier lets REST handlers push events to connected clients.
type RealtimeNotifier interface {
	NotifyChatRead(ctx context.Context, chatID, readerID string)
}

// IncomingEvent is an event sent by a client.
type IncomingEvent struct {
	Type       bcType.BCEventType `json:"type"`
	ReceiverID string             `json:"receiverId"`
	ChatID     string             `json:"chatId"`
	MessageID  string             `json:"messageId"`
	Content    string             `json:"content"`
}

// client is one WebSocket connection. A user can have several (tabs, devices,
// a reload that overlaps with the old socket closing).
type client struct {
	userID string
	conn   *websocket.Conn

	// The underlying connection does not allow concurrent writers, and the
	// connection object is recycled once the handler returns, so every write
	// goes through this lock and stops once the client is closed.
	mu     sync.Mutex
	closed bool
}

func (c *client) send(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errClientClosed
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteJSON(v)
}

func (c *client) closeWith(code int, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason))
}

func (c *client) markClosed() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
}

type WebSocketHandler struct {
	Log       *logger.AppLogger
	JWT       *security.JWT
	ChatUC    usecase.ChatUsecase
	MessageUC usecase.MessageUsecase

	mu      sync.RWMutex
	clients map[string]map[*client]struct{} // userID -> connections

	// presenceMu serialises presence broadcasts so observers always end up
	// with the latest state, even when a user connects and disconnects at
	// the same moment (e.g. a page reload).
	presenceMu sync.Mutex
	announced  map[string]bool
}

func NewWebSocketHandler(log *logger.AppLogger, jwt *security.JWT, chatUC usecase.ChatUsecase, messageUC usecase.MessageUsecase) *WebSocketHandler {
	log.WS.Info.Info().Msg("WebSocket handler initialized")
	return &WebSocketHandler{
		Log:       log,
		JWT:       jwt,
		ChatUC:    chatUC,
		MessageUC: messageUC,
		clients:   make(map[string]map[*client]struct{}),
		announced: make(map[string]bool),
	}
}

// HandleWebSocket serves one connection. The client authenticates with the
// same JWT it uses for REST calls: /ws?token=<jwt>.
func (h *WebSocketHandler) HandleWebSocket(conn *websocket.Conn) {
	ctx := context.Background()

	userID, err := h.JWT.GetUserIdFromToken(conn.Query("token"))
	if err != nil || userID == "" {
		h.Log.WS.Warning.Warn().
			Str("remoteAddr", conn.RemoteAddr().String()).
			Msg("WebSocket connection rejected: invalid or missing token")
		c := &client{conn: conn}
		_ = c.send(map[string]string{"type": string(bcType.ERROR), "error": "unauthorized"})
		c.closeWith(CloseUnauthorized, "unauthorized")
		return
	}

	c := &client{userID: userID, conn: conn}
	h.register(c)
	defer func() {
		h.unregister(c)
		h.announcePresence(userID)
		h.Log.WS.Stream.Info().Str("userId", userID).Msg("WebSocket connection closed")
	}()

	h.Log.WS.Stream.Info().
		Str("userId", userID).
		Str("remoteAddr", conn.RemoteAddr().String()).
		Msg("WebSocket connection established")

	_ = c.send(map[string]interface{}{
		"type":   bcType.CONNECTED,
		"userId": userID,
	})
	_ = c.send(map[string]interface{}{
		"type":    bcType.ONLINE_USERS,
		"userIds": h.onlineUserIDs(userID),
	})
	h.announcePresence(userID)
	h.deliverPending(ctx, userID)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				h.Log.WS.Warning.Warn().Str("userId", userID).Err(err).Msg("WebSocket read error")
			}
			return
		}

		var event IncomingEvent
		if err := json.Unmarshal(data, &event); err != nil {
			_ = c.sendError("invalid message format")
			continue
		}

		h.Log.WS.Stream.Info().
			Str("userId", userID).
			Str("messageType", string(event.Type)).
			Msg("Incoming WebSocket message")

		h.dispatch(ctx, c, event)
	}
}

func (h *WebSocketHandler) dispatch(ctx context.Context, c *client, event IncomingEvent) {
	switch event.Type {
	case "":
		_ = c.sendError("message type is required")
	case bcType.CREATE_CHAT:
		h.handleCreateChat(ctx, c, event)
	case bcType.SEND_MESSAGE:
		h.handleSendMessage(ctx, c, event)
	case bcType.JOIN_ROOM:
		h.handleJoinRoom(ctx, c, event)
	case bcType.LEAVE_ROOM:
		// Messages are delivered to every participant, so there is no room
		// state to leave. Kept so older clients do not get an error.
	case bcType.MESSAGE_DELIVERED:
		h.handleMessageDelivered(ctx, c, event)
	case bcType.MESSAGE_READ:
		h.handleMessageRead(ctx, c, event)
	case bcType.TYPING:
		h.handleTyping(ctx, c, event.ChatID, true)
	case bcType.STOP_TYPING:
		h.handleTyping(ctx, c, event.ChatID, false)
	default:
		_ = c.sendError("unknown message type: " + string(event.Type))
	}
}

// ---------------------------------------------------------------------------
// Event handlers

func (h *WebSocketHandler) handleCreateChat(ctx context.Context, c *client, event IncomingEvent) {
	content := strings.TrimSpace(event.Content)
	if event.ReceiverID == "" || content == "" {
		_ = c.sendError("receiverId and content are required")
		return
	}
	if len([]rune(content)) > maxMessageLength {
		_ = c.sendError("message is too long")
		return
	}

	result, err := h.ChatUC.CreatePersonalChat(ctx, c.userID, event.ReceiverID, content)
	if err != nil {
		h.Log.WS.Error.Error().Err(err).Str("userId", c.userID).Msg("Failed to create chat")
		_ = c.sendError(publicError(err, "failed to create chat"))
		return
	}

	// Each side sees the chat named after the other person.
	h.sendToUser(c.userID, chatCreatedEvent(
		mapper.MapperCreateChatResponse(result.Chat, result.Receiver, result.Message), c.userID))
	h.sendToUser(result.Receiver.ID, chatCreatedEvent(
		mapper.MapperCreateChatResponse(result.Chat, result.Sender, result.Message), c.userID))

	h.sendToUsers([]string{c.userID, result.Receiver.ID}, newMessageEvent(result.Message))
}

func (h *WebSocketHandler) handleSendMessage(ctx context.Context, c *client, event IncomingEvent) {
	content := strings.TrimSpace(event.Content)
	if event.ChatID == "" || content == "" {
		_ = c.sendError("chatId and content are required")
		return
	}
	if len([]rune(content)) > maxMessageLength {
		_ = c.sendError("message is too long")
		return
	}

	participants, ok := h.participantsFor(ctx, c, event.ChatID)
	if !ok {
		return
	}

	message, err := h.MessageUC.ProcessIncomingMessage(ctx, req.MessageRequest{
		ChatID:   event.ChatID,
		SenderID: c.userID,
		Content:  content,
	})
	if err != nil {
		h.Log.WS.Error.Error().
			Err(err).
			Str("chatId", event.ChatID).
			Str("senderId", c.userID).
			Msg("Failed to process incoming message")
		_ = c.sendError("failed to send message")
		return
	}

	h.sendToUsers(participants, newMessageEvent(message))
}

// handleJoinRoom only checks membership: every participant receives a chat's
// events without joining. Kept for clients that still send it.
func (h *WebSocketHandler) handleJoinRoom(ctx context.Context, c *client, event IncomingEvent) {
	if event.ChatID == "" {
		_ = c.sendError("chatId is required")
		return
	}
	if _, ok := h.participantsFor(ctx, c, event.ChatID); !ok {
		return
	}
	_ = c.send(map[string]interface{}{
		"type":   bcType.JOINED_ROOM,
		"chatId": event.ChatID,
	})
}

func (h *WebSocketHandler) handleMessageDelivered(ctx context.Context, c *client, event IncomingEvent) {
	if event.MessageID == "" {
		return
	}
	receipt, err := h.MessageUC.MarkDelivered(ctx, event.MessageID, c.userID)
	if err != nil {
		h.Log.WS.Error.Error().Err(err).Str("messageId", event.MessageID).Msg("Failed to mark message as delivered")
		return
	}
	if receipt != nil {
		h.sendToUser(receipt.SenderID, deliveredEvent(*receipt, c.userID))
	}
}

func (h *WebSocketHandler) handleMessageRead(ctx context.Context, c *client, event IncomingEvent) {
	if event.ChatID == "" {
		_ = c.sendError("chatId is required")
		return
	}
	updated, err := h.MessageUC.MarkMessagesAsRead(ctx, event.ChatID, c.userID)
	if err != nil {
		_ = c.sendError(publicError(err, "failed to mark messages as read"))
		return
	}
	if updated > 0 {
		h.NotifyChatRead(ctx, event.ChatID, c.userID)
	}
}

func (h *WebSocketHandler) handleTyping(ctx context.Context, c *client, chatID string, isTyping bool) {
	if chatID == "" {
		return
	}
	participants, err := h.ChatUC.GetParticipantIDs(ctx, chatID)
	if err != nil || !contains(participants, c.userID) {
		return
	}
	h.sendToUsers(without(participants, c.userID), map[string]interface{}{
		"type":     bcType.TYPING,
		"chatId":   chatID,
		"userId":   c.userID,
		"isTyping": isTyping,
	})
}

// NotifyChatRead tells the other participants that readerID has read
// everything in the chat.
func (h *WebSocketHandler) NotifyChatRead(ctx context.Context, chatID, readerID string) {
	participants, err := h.ChatUC.GetParticipantIDs(ctx, chatID)
	if err != nil {
		h.Log.WS.Error.Error().Err(err).Str("chatId", chatID).Msg("Failed to load participants for read receipt")
		return
	}
	h.sendToUsers(without(participants, readerID), map[string]interface{}{
		"type":   bcType.MESSAGE_READ,
		"chatId": chatID,
		"userId": readerID,
	})
}

// deliverPending marks messages that arrived while the user was offline as
// delivered and lets their senders know.
func (h *WebSocketHandler) deliverPending(ctx context.Context, userID string) {
	receipts, err := h.MessageUC.MarkPendingDelivered(ctx, userID)
	if err != nil {
		return
	}
	for _, r := range receipts {
		h.sendToUser(r.SenderID, deliveredEvent(r, userID))
	}
}

// participantsFor returns the chat's participants, or reports an error to
// the client and returns false when the user is not one of them.
func (h *WebSocketHandler) participantsFor(ctx context.Context, c *client, chatID string) ([]string, bool) {
	participants, err := h.ChatUC.GetParticipantIDs(ctx, chatID)
	if err != nil {
		h.Log.WS.Error.Error().Err(err).Str("chatId", chatID).Msg("Failed to load chat participants")
		_ = c.sendError("something went wrong, please try again")
		return nil, false
	}
	if contains(participants, c.userID) {
		return participants, true
	}
	_ = c.sendError(usecase.ErrNotParticipant.Error())
	return nil, false
}

// ---------------------------------------------------------------------------
// Connections and presence

func (h *WebSocketHandler) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.userID] == nil {
		h.clients[c.userID] = make(map[*client]struct{})
	}
	h.clients[c.userID][c] = struct{}{}
	h.Log.WS.Info.Info().
		Str("userId", c.userID).
		Int("connections", len(h.clients[c.userID])).
		Msg("Client registered")
}

func (h *WebSocketHandler) unregister(c *client) {
	c.markClosed()
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.clients[c.userID]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.clients, c.userID)
		}
	}
}

func (h *WebSocketHandler) isOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID]) > 0
}

func (h *WebSocketHandler) onlineUserIDs(exclude string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.clients))
	for id := range h.clients {
		if id != exclude {
			ids = append(ids, id)
		}
	}
	return ids
}

// announcePresence broadcasts the user's current online state if it changed
// since the last announcement.
func (h *WebSocketHandler) announcePresence(userID string) {
	h.presenceMu.Lock()
	defer h.presenceMu.Unlock()

	online := h.isOnline(userID)
	if h.announced[userID] == online {
		return
	}
	if online {
		h.announced[userID] = true
	} else {
		delete(h.announced, userID)
	}

	event := map[string]interface{}{
		"type":     bcType.USER_STATUS,
		"userId":   userID,
		"isOnline": online,
	}
	for _, c := range h.snapshot(func(id string) bool { return id != userID }) {
		_ = c.send(event)
	}
	h.Log.WS.Info.Info().Str("userId", userID).Bool("isOnline", online).Msg("User status broadcasted")
}

// snapshot copies the matching connections so writes happen without holding
// the map lock.
func (h *WebSocketHandler) snapshot(match func(userID string) bool) []*client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*client
	for id, conns := range h.clients {
		if !match(id) {
			continue
		}
		for c := range conns {
			out = append(out, c)
		}
	}
	return out
}

func (h *WebSocketHandler) sendToUser(userID string, event interface{}) {
	h.sendToUsers([]string{userID}, event)
}

func (h *WebSocketHandler) sendToUsers(userIDs []string, event interface{}) {
	wanted := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		wanted[id] = struct{}{}
	}
	for _, c := range h.snapshot(func(id string) bool { _, ok := wanted[id]; return ok }) {
		if err := c.send(event); err != nil && !errors.Is(err, errClientClosed) {
			h.Log.WS.Error.Error().Err(err).Str("userId", c.userID).Msg("Failed to send event")
		}
	}
}

func (c *client) sendError(message string) error {
	return c.send(map[string]string{"type": string(bcType.ERROR), "error": message})
}

// ---------------------------------------------------------------------------
// Payloads

func newMessageEvent(m dto.BroadcastMessage) map[string]interface{} {
	return map[string]interface{}{
		"type":         bcType.NEW_MESSAGE,
		"chatId":       m.ChatID,
		"messageId":    m.MessageID,
		"senderId":     m.SenderID,
		"senderName":   m.SenderName,
		"senderAvatar": m.SenderAvatar,
		"content":      m.Content,
		"status":       m.Status,
		"createdAt":    m.CreatedAt,
	}
}

func chatCreatedEvent(chat *res.CreateChatResponse, initiatorID string) map[string]interface{} {
	return map[string]interface{}{
		"type":            bcType.CHAT_CREATED,
		"chatId":          chat.ChatId,
		"receiverId":      chat.ReceiverId,
		"chatUsername":    chat.ChatUsername,
		"chatAvatar":      chat.ChatAvatar,
		"chatType":        chat.ChatType,
		"lastMessage":     chat.LastMessage,
		"lastMessageTime": chat.LastMessageTime,
		"unreadCount":     chat.UnreadCount,
		"initiatorId":     initiatorID,
	}
}

func deliveredEvent(r usecase.DeliveryReceipt, recipientID string) map[string]interface{} {
	return map[string]interface{}{
		"type":      bcType.MESSAGE_DELIVERED,
		"messageId": r.MessageID,
		"chatId":    r.ChatID,
		"userId":    recipientID,
	}
}

// publicError returns err's message when it is meant for clients, otherwise
// the fallback, so database errors never leak to the browser.
func publicError(err error, fallback string) string {
	for _, known := range []error{usecase.ErrSelfChat, usecase.ErrUserNotFound, usecase.ErrNotParticipant} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return fallback
}

func without(ids []string, exclude string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != exclude {
			out = append(out, id)
		}
	}
	return out
}

func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
