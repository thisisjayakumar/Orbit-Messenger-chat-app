package biz

import (
	"errors"
	
	"github.com/google/wire"
)

var (
	ErrMessageNotFound = errors.New("message not found")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrInvalidPayload  = errors.New("invalid payload")
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewMessageUsecase)
