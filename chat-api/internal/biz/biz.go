package biz

import "errors"

var (
	ErrConversationNotFound    = errors.New("conversation not found")
	ErrNotParticipant          = errors.New("user is not a participant")
	ErrInsufficientPermissions = errors.New("insufficient permissions")
	ErrInvalidRequest          = errors.New("invalid request")
	ErrInvalidDMParticipants   = errors.New("DM conversations must have exactly 2 participants")
	ErrMessageNotFound         = errors.New("message not found")
)
