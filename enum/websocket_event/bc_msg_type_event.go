package websocket_event

type BCEventType string

const (
	// connection
	CONNECTED BCEventType = "connected"

	// chat lifecycle
	CREATE_CHAT  BCEventType = "create_chat"
	CHAT_CREATED BCEventType = "chat_created"

	// room
	JOIN_ROOM   BCEventType = "join_room"
	JOINED_ROOM BCEventType = "joined_room"
	LEAVE_ROOM  BCEventType = "leave_room"

	// message
	SEND_MESSAGE BCEventType = "send_message"
	NEW_MESSAGE  BCEventType = "new_message"

	// status
	USER_STATUS       BCEventType = "user_status"
	MESSAGE_DELIVERED BCEventType = "message_delivered"
	MESSAGE_READ      BCEventType = "message_read"

	// presence
	TYPING      BCEventType = "typing"
	STOP_TYPING BCEventType = "stop_typing"

	ERROR BCEventType = "error"
)
