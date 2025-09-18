package biz

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type PresenceStatus string

const (
	StatusOnline     PresenceStatus = "online"
	StatusAway       PresenceStatus = "away"
	StatusOffline    PresenceStatus = "offline"
	StatusDoNotDisturb PresenceStatus = "dnd"
)

type UserPresence struct {
	UserID       uuid.UUID      `json:"user_id"`
	Status       PresenceStatus `json:"status"`
	LastSeen     time.Time      `json:"last_seen"`
	DeviceInfo   string         `json:"device_info,omitempty"`
	CustomStatus string         `json:"custom_status,omitempty"`
}

type DeviceSession struct {
	ID            uuid.UUID  `json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	ClientID      string     `json:"client_id"`
	DeviceInfo    string     `json:"device_info,omitempty"`
	IP            string     `json:"ip,omitempty"`
	ConnectedAt   time.Time  `json:"connected_at"`
	DisconnectedAt *time.Time `json:"disconnected_at,omitempty"`
	LastHeartbeat time.Time  `json:"last_heartbeat"`
}

type PresenceUpdate struct {
	UserID       uuid.UUID      `json:"user_id"`
	Status       PresenceStatus `json:"status"`
	CustomStatus string         `json:"custom_status,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
}

type HeartbeatMessage struct {
	UserID    uuid.UUID `json:"user_id"`
	ClientID  string    `json:"client_id"`
	Timestamp time.Time `json:"timestamp"`
}

type PresenceRepo interface {
	SetUserPresence(ctx context.Context, presence *UserPresence) error
	GetUserPresence(ctx context.Context, userID uuid.UUID) (*UserPresence, error)
	GetMultipleUserPresence(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]*UserPresence, error)
	
	CreateDeviceSession(ctx context.Context, session *DeviceSession) error
	UpdateDeviceSession(ctx context.Context, session *DeviceSession) error
	GetDeviceSession(ctx context.Context, clientID string) (*DeviceSession, error)
	GetUserDeviceSessions(ctx context.Context, userID uuid.UUID) ([]*DeviceSession, error)
	DisconnectDeviceSession(ctx context.Context, clientID string) error
	
	// Bulk operations for cleanup
	GetStaleDeviceSessions(ctx context.Context, timeout time.Duration) ([]*DeviceSession, error)
	CleanupStalePresence(ctx context.Context, timeout time.Duration) error
}

type PresenceUsecase struct {
	repo              PresenceRepo
	heartbeatInterval time.Duration
	offlineTimeout    time.Duration
}

func NewPresenceUsecase(repo PresenceRepo, heartbeatInterval, offlineTimeout time.Duration) *PresenceUsecase {
	return &PresenceUsecase{
		repo:              repo,
		heartbeatInterval: heartbeatInterval,
		offlineTimeout:    offlineTimeout,
	}
}

func (uc *PresenceUsecase) HandleClientConnected(ctx context.Context, clientID string, userID uuid.UUID, deviceInfo, ip string) error {
	// Create device session
	session := &DeviceSession{
		ID:            uuid.New(),
		UserID:        userID,
		ClientID:      clientID,
		DeviceInfo:    deviceInfo,
		IP:            ip,
		ConnectedAt:   time.Now(),
		LastHeartbeat: time.Now(),
	}

	if err := uc.repo.CreateDeviceSession(ctx, session); err != nil {
		return err
	}

	// Update user presence to online
	presence := &UserPresence{
		UserID:   userID,
		Status:   StatusOnline,
		LastSeen: time.Now(),
	}

	return uc.repo.SetUserPresence(ctx, presence)
}

func (uc *PresenceUsecase) HandleClientDisconnected(ctx context.Context, clientID string) error {
	session, err := uc.repo.GetDeviceSession(ctx, clientID)
	if err != nil {
		return err
	}

	// Disconnect the session
	if err := uc.repo.DisconnectDeviceSession(ctx, clientID); err != nil {
		return err
	}

	// Check if user has other active sessions
	activeSessions, err := uc.repo.GetUserDeviceSessions(ctx, session.UserID)
	if err != nil {
		return err
	}

	// If no active sessions, set user to offline
	hasActiveSessions := false
	for _, s := range activeSessions {
		if s.DisconnectedAt == nil && s.ClientID != clientID {
			hasActiveSessions = true
			break
		}
	}

	if !hasActiveSessions {
		presence := &UserPresence{
			UserID:   session.UserID,
			Status:   StatusOffline,
			LastSeen: time.Now(),
		}
		return uc.repo.SetUserPresence(ctx, presence)
	}

	return nil
}

func (uc *PresenceUsecase) HandlePresenceUpdate(ctx context.Context, payload []byte) error {
	var update PresenceUpdate
	if err := json.Unmarshal(payload, &update); err != nil {
		return err
	}

	presence := &UserPresence{
		UserID:       update.UserID,
		Status:       update.Status,
		LastSeen:     update.Timestamp,
		CustomStatus: update.CustomStatus,
	}

	return uc.repo.SetUserPresence(ctx, presence)
}

func (uc *PresenceUsecase) HandleHeartbeat(ctx context.Context, payload []byte) error {
	var heartbeat HeartbeatMessage
	if err := json.Unmarshal(payload, &heartbeat); err != nil {
		return err
	}

	session, err := uc.repo.GetDeviceSession(ctx, heartbeat.ClientID)
	if err != nil {
		return err
	}

	session.LastHeartbeat = heartbeat.Timestamp
	return uc.repo.UpdateDeviceSession(ctx, session)
}

func (uc *PresenceUsecase) GetUserPresence(ctx context.Context, userID uuid.UUID) (*UserPresence, error) {
	return uc.repo.GetUserPresence(ctx, userID)
}

func (uc *PresenceUsecase) GetMultipleUserPresence(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]*UserPresence, error) {
	return uc.repo.GetMultipleUserPresence(ctx, userIDs)
}

func (uc *PresenceUsecase) SetUserStatus(ctx context.Context, userID uuid.UUID, status PresenceStatus, customStatus string) error {
	presence := &UserPresence{
		UserID:       userID,
		Status:       status,
		LastSeen:     time.Now(),
		CustomStatus: customStatus,
	}

	return uc.repo.SetUserPresence(ctx, presence)
}

// CleanupStalePresence removes stale presence data
func (uc *PresenceUsecase) CleanupStalePresence(ctx context.Context) error {
	// Get stale device sessions
	staleSessions, err := uc.repo.GetStaleDeviceSessions(ctx, uc.offlineTimeout)
	if err != nil {
		return err
	}

	// Process each stale session
	for _, session := range staleSessions {
		if err := uc.HandleClientDisconnected(ctx, session.ClientID); err != nil {
			// Log error but continue processing
			continue
		}
	}

	// Clean up stale presence records
	return uc.repo.CleanupStalePresence(ctx, uc.offlineTimeout)
}

// GetUserDeviceSessions returns all device sessions for a user
func (uc *PresenceUsecase) GetUserDeviceSessions(ctx context.Context, userID uuid.UUID) ([]*DeviceSession, error) {
	return uc.repo.GetUserDeviceSessions(ctx, userID)
}
