package usecase

import "errors"

// Errors with messages that are safe to show to the client.
var (
	ErrSelfChat       = errors.New("you cannot start a chat with yourself")
	ErrUserNotFound   = errors.New("user not found")
	ErrNotParticipant = errors.New("you are not a participant of this chat")
	ErrInvalidProfile = errors.New("name must be 2-50 characters and phone number 8-20 characters")
	ErrPhoneTaken     = errors.New("phone number is already used by another account")

	ErrInvalidRefreshToken = errors.New("refresh token is invalid or expired")
)
