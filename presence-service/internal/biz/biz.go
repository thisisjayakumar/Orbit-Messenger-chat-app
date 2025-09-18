package biz

import (
	"errors"
	"time"
	
	"github.com/google/wire"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrUserNotFound    = errors.New("user not found")
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewPresenceUsecaseFromConfig)

// NewPresenceUsecaseFromConfig creates presence usecase with default config
func NewPresenceUsecaseFromConfig(repo PresenceRepo) *PresenceUsecase {
	return NewPresenceUsecase(repo, 30*time.Second, 60*time.Second)
}
