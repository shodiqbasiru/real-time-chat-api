package usecase

import (
	"context"
	"errors"
	"fmt"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto/req"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/entity"
	"real-time-chat-app/enum"
	"real-time-chat-app/mapper"
	"real-time-chat-app/repository"
	"real-time-chat-app/security"

	"gorm.io/gorm"
)

type ChatUsecaseImpl struct {
	*repository.ChatRepository
	MessageUsecase
	UserUsecase
	Log *logger.AppLogger
	*gorm.DB
	*security.JWT
}

func NewChatUsecase(chatRepository *repository.ChatRepository, messageUC MessageUsecase, userUC UserUsecase, logger *logger.AppLogger, DB *gorm.DB, JWT *security.JWT) *ChatUsecaseImpl {
	return &ChatUsecaseImpl{ChatRepository: chatRepository, MessageUsecase: messageUC, UserUsecase: userUC, Log: logger, DB: DB, JWT: JWT}
}

func (uc *ChatUsecaseImpl) CreatePersonalChat(ctx context.Context, userAID, userBID, content string) (*res.CreateChatResponse, error) {
	uc.Log.Http.Info.Info().
		Str("chatId", content).
		Str("userAID", userAID).
		Str("userBID", userBID).
		Msg("CreatePersonalChat started")

	uc.Log.Http.Trace.Trace().
		Str("chatId", content).
		Str("userAID", userAID).
		Str("userBID", userBID).
		Msg("Checking for existing personal chat")

	existingChat, err := uc.ChatRepository.FindPersonalChat(ctx, uc.DB, userAID, userBID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("userAID", userAID).
			Str("userBID", userBID).
			Msg("Failed to check for existing personal chat")
		return nil, err
	}

	if existingChat != nil {
		uc.Log.Http.Info.Info().
			Str("chatId", existingChat.ID).
			Str("userAID", userAID).
			Str("userBID", userBID).
			Msg("Personal chat already exists")
		return nil, nil
	}

	uc.Log.Http.Trace.Trace().
		Str("userAID", userAID).
		Str("userBID", userBID).
		Msg("Creating new personal chat")

	newChat := &entity.Chat{
		ChatType: enum.PRIVATE,
	}

	participants := []entity.ChatParticipant{
		{UserID: userAID},
		{UserID: userBID},
	}

	if err := uc.ChatRepository.CreateChatWithParticipants(ctx, uc.DB, newChat, participants); err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("userAID", userAID).
			Str("userBID", userBID).
			Msg("Failed to create personal chat")
		return nil, err
	}

	newMessage := req.MessageRequest{
		ChatID:     newChat.ID,
		SenderID:   userAID,
		ReceiverID: userBID,
		Content:    content,
	}

	BCMessage, err := uc.MessageUsecase.ProcessIncomingMessage(ctx, newMessage)
	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Msg("Failed to create newChat")
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	receiver, _ := uc.UserUsecase.FindUser(ctx, userBID)

	uc.Log.Http.Info.Info().
		Str("chatId", newChat.ID).
		Str("userAID", userAID).
		Str("userBID", userBID).
		Msg("Personal chat created successfully")

	return mapper.MapperCreateChatResponse(*newChat, *receiver, BCMessage), err
}

func (uc *ChatUsecaseImpl) IsUserInChat(ctx context.Context, chatID, userID string) (bool, error) {
	return uc.ChatRepository.IsUserInChat(ctx, uc.DB, chatID, userID)
}

func (uc *ChatUsecaseImpl) CreateGroupChat(ctx context.Context, name string, creatorID string, memberIDs []string) (*entity.Chat, error) {
	uc.Log.Http.Info.Info().
		Str("groupName", name).
		Str("creatorId", creatorID).
		Int("memberCount", len(memberIDs)).
		Msg("CreateGroupChat started")

	newChat := &entity.Chat{
		ChatType:  enum.GROUP,
		GroupName: name,
	}

	participants := make([]entity.ChatParticipant, 0, len(memberIDs)+1)
	participants = append(participants, entity.ChatParticipant{UserID: creatorID})
	for _, id := range memberIDs {
		participants = append(participants, entity.ChatParticipant{UserID: id})
	}

	uc.Log.Http.Trace.Trace().
		Str("groupName", name).
		Int("totalParticipants", len(participants)).
		Msg("Creating group chat with participants")

	uc.Log.Http.Trace.Trace().
		Str("groupName", name).
		Int("totalParticipants", len(participants)).
		Msg("Creating group chat with participants")

	if err := uc.ChatRepository.CreateChatWithParticipants(ctx, uc.DB, newChat, participants); err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("groupName", name).
			Str("creatorId", creatorID).
			Msg("Failed to create group chat")
		return nil, err
	}

	uc.Log.Http.Info.Info().
		Str("chatId", newChat.ID).
		Str("groupName", name).
		Int("participantCount", len(participants)).
		Msg("Group chat created successfully")

	return newChat, nil
}

func (uc *ChatUsecaseImpl) FindChatByID(ctx context.Context, chatID string) (*entity.Chat, error) {
	uc.Log.Http.Trace.Trace().
		Str("chatId", chatID).
		Msg("Finding chat by ID")

	chat, err := uc.ChatRepository.FindChatByID(ctx, uc.DB, chatID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.Log.Http.Warning.Warn().
				Str("chatId", chatID).
				Msg("Chat not found")
		} else {
			uc.Log.Http.Error.Error().
				Err(err).
				Str("chatId", chatID).
				Msg("Failed to find chat by ID")
		}
		return nil, err
	}

	uc.Log.Http.Trace.Trace().
		Str("chatId", chatID).
		Str("chatType", string(chat.ChatType)).
		Msg("Chat found")

	return chat, nil
}

func (uc *ChatUsecaseImpl) GetChatsByUser(ctx context.Context, token string) ([]res.ChatResponse, error) {
	uc.Log.Http.Info.Info().Msg("GetChatsByUser started")

	// Extract user ID from token
	userId, err := uc.JWT.GetUserIdFromToken(token)
	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Msg("Failed to extract user ID from token")
		return nil, errors.New("invalid token")
	}

	uc.Log.Http.Trace.Trace().
		Str("userId", userId).
		Msg("Finding chats for user")

	// Get chats
	chats, err := uc.ChatRepository.FindAllByUserID(ctx, uc.DB, userId)
	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("userId", userId).
			Msg("Failed to get chats by user ID")
		return nil, err
	}

	uc.Log.Http.Trace.Trace().
		Str("userId", userId).
		Int("chatCount", len(chats)).
		Msg("Chats found, fetching unread counts")

	// Get unread counts
	unreadMap, err := uc.getUnreadCount(ctx, userId)
	if err != nil {
		uc.Log.Http.Warning.Warn().
			Err(err).
			Str("userId", userId).
			Msg("Failed to get unread counts, continuing with empty map")
		unreadMap = make(map[string]int)
	}

	var chatResponses []res.ChatResponse

	for _, chat := range chats {
		var chatUsername string
		var charReceiverId string

		if chat.ChatType == enum.PRIVATE {
			// Find other participant
			for _, participant := range chat.Participants {
				if participant.UserID != userId {
					var otherUser entity.User
					if err := uc.DB.First(&otherUser, "id = ?", participant.UserID).Error; err == nil {
						chatUsername = otherUser.Name
						charReceiverId = otherUser.ID
					} else {
						uc.Log.Http.Warning.Warn().
							Err(err).
							Str("chatId", chat.ID).
							Str("participantId", participant.UserID).
							Msg("Failed to get other user info")
					}
					break
				}
			}
		} else if chat.ChatType == enum.GROUP {
			chatUsername = chat.GroupName
		}

		// Get last message
		var lastMessage entity.Messages
		var lastMessageContent string
		var lastMessageTime string

		if err := uc.DB.Where("chat_id = ?", chat.ID).
			Order("created_at DESC").
			First(&lastMessage).Error; err == nil {
			lastMessageContent = lastMessage.Content
			lastMessageTime = lastMessage.CreatedAt.Format("2006-01-02 15:04:05")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			uc.Log.Http.Trace.Trace().
				Str("chatId", chat.ID).
				Msg("No messages in chat yet")
		}

		unread := unreadMap[chat.ID]

		chatResponses = append(chatResponses, res.ChatResponse{
			ChatId:          chat.ID,
			ReceiverId:      charReceiverId,
			ChatUsername:    chatUsername,
			LastMessage:     lastMessageContent,
			UnreadCount:     uint(unread),
			LastMessageTime: lastMessageTime,
		})
	}

	uc.Log.Http.Info.Info().
		Str("userId", userId).
		Int("chatCount", len(chatResponses)).
		Msg("Successfully retrieved chats for user")

	return chatResponses, nil
}

func (uc *ChatUsecaseImpl) GetMessagesByChatID(ctx context.Context, token string, chatId string) ([]res.MessageResponse, error) {
	uc.Log.Http.Info.Info().
		Str("chatId", chatId).
		Msg("GetMessagesByChatID started")

	// Extract user ID from token
	userId, err := uc.JWT.GetUserIdFromToken(token)
	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("chatId", chatId).
			Msg("Failed to parse token")
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	uc.Log.Http.Trace.Trace().
		Str("userId", userId).
		Str("chatId", chatId).
		Msg("Verifying user is participant in chat")

	// Verify user is participant
	isParticipant, err := uc.ChatRepository.IsUserInChat(ctx, uc.DB, chatId, userId)
	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("userId", userId).
			Str("chatId", chatId).
			Msg("Failed to verify participant")
		return nil, fmt.Errorf("failed to verify participant: %w", err)
	}

	if !isParticipant {
		uc.Log.Http.Warning.Warn().
			Str("userId", userId).
			Str("chatId", chatId).
			Msg("User not authorized for this chat")
		return nil, fmt.Errorf("user not authorized for this chat")
	}

	uc.Log.Http.Trace.Trace().
		Str("userId", userId).
		Str("chatId", chatId).
		Msg("User verified, fetching messages")

	// Get messages
	messages, err := uc.ChatRepository.FindMessagesByChatID(ctx, uc.DB, chatId)
	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("chatId", chatId).
			Msg("Failed to get messages")
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	var responses []res.MessageResponse
	for _, msg := range messages {
		responses = append(responses, res.MessageResponse{
			MessageId:  msg.ID,
			Content:    msg.Content,
			SenderId:   msg.SenderId,
			SenderName: msg.Sender.Name,
			Status:     string(msg.Status),
			CreatedAt:  msg.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	uc.Log.Http.Info.Info().
		Str("userId", userId).
		Str("chatId", chatId).
		Int("messageCount", len(responses)).
		Msg("Successfully retrieved messages")

	return responses, nil
}

func (uc *ChatUsecaseImpl) getUnreadCount(ctx context.Context, userID string) (map[string]int, error) {
	uc.Log.Http.Trace.Trace().
		Str("userId", userID).
		Msg("Calculating unread counts")

	type result struct {
		ChatID string
		Count  int
	}
	var rows []result

	err := uc.DB.Table("t_message_status AS ms").
		Select("m.chat_id, COUNT(ms.id) as count").
		Joins("JOIN t_messages m ON m.id = ms.message_id").
		Where("ms.user_id = ? AND ms.is_read = false", userID).
		Group("m.chat_id").
		Scan(&rows).Error

	if err != nil {
		uc.Log.Http.Error.Error().
			Err(err).
			Str("userId", userID).
			Msg("Failed to calculate unread counts")
		return nil, err
	}

	unreadMap := make(map[string]int)
	for _, r := range rows {
		unreadMap[r.ChatID] = r.Count
	}

	uc.Log.Http.Trace.Trace().
		Str("userId", userID).
		Int("chatsWithUnread", len(unreadMap)).
		Msg("Unread counts calculated")

	return unreadMap, nil
}
