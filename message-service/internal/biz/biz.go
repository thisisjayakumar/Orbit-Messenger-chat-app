package biz

import "errors"

var (
	ErrMessageNotFound = errors.New("message not found")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrInvalidPayload  = errors.New("invalid payload")
)
