package biz

import (
	"context"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// Mock PresenceRepo
// ──────────────────────────────────────────────

type mockPresenceRepo struct {
	setUserPresenceFunc         func(ctx context.Context, p *UserPresence) error
	getUserPresenceFunc         func(ctx context.Context, uid int) (*UserPresence, error)
	getMultipleUserPresenceFunc func(ctx context.Context, uids []int) (map[int]*UserPresence, error)
	createDeviceSessionFunc     func(ctx context.Context, s *DeviceSession) error
	updateDeviceSessionFunc     func(ctx context.Context, s *DeviceSession) error
	getDeviceSessionFunc        func(ctx context.Context, cid string) (*DeviceSession, error)
	getUserDeviceSessionsFunc   func(ctx context.Context, uid int) ([]*DeviceSession, error)
	disconnectDeviceSessionFunc func(ctx context.Context, cid string) error
	getStaleDeviceSessionsFunc  func(ctx context.Context, timeout time.Duration) ([]*DeviceSession, error)
	cleanupStalePresenceFunc    func(ctx context.Context, timeout time.Duration) error
}

func (m *mockPresenceRepo) SetUserPresence(ctx context.Context, p *UserPresence) error {
	if m.setUserPresenceFunc != nil {
		return m.setUserPresenceFunc(ctx, p)
	}
	return nil
}
func (m *mockPresenceRepo) GetUserPresence(ctx context.Context, uid int) (*UserPresence, error) {
	if m.getUserPresenceFunc != nil {
		return m.getUserPresenceFunc(ctx, uid)
	}
	return nil, ErrUserNotFound
}
func (m *mockPresenceRepo) GetMultipleUserPresence(ctx context.Context, uids []int) (map[int]*UserPresence, error) {
	if m.getMultipleUserPresenceFunc != nil {
		return m.getMultipleUserPresenceFunc(ctx, uids)
	}
	return nil, nil
}
func (m *mockPresenceRepo) CreateDeviceSession(ctx context.Context, s *DeviceSession) error {
	if m.createDeviceSessionFunc != nil {
		return m.createDeviceSessionFunc(ctx, s)
	}
	return nil
}
func (m *mockPresenceRepo) UpdateDeviceSession(ctx context.Context, s *DeviceSession) error {
	if m.updateDeviceSessionFunc != nil {
		return m.updateDeviceSessionFunc(ctx, s)
	}
	return nil
}
func (m *mockPresenceRepo) GetDeviceSession(ctx context.Context, cid string) (*DeviceSession, error) {
	if m.getDeviceSessionFunc != nil {
		return m.getDeviceSessionFunc(ctx, cid)
	}
	return nil, ErrSessionNotFound
}
func (m *mockPresenceRepo) GetUserDeviceSessions(ctx context.Context, uid int) ([]*DeviceSession, error) {
	if m.getUserDeviceSessionsFunc != nil {
		return m.getUserDeviceSessionsFunc(ctx, uid)
	}
	return nil, nil
}
func (m *mockPresenceRepo) DisconnectDeviceSession(ctx context.Context, cid string) error {
	if m.disconnectDeviceSessionFunc != nil {
		return m.disconnectDeviceSessionFunc(ctx, cid)
	}
	return nil
}
func (m *mockPresenceRepo) GetStaleDeviceSessions(ctx context.Context, timeout time.Duration) ([]*DeviceSession, error) {
	if m.getStaleDeviceSessionsFunc != nil {
		return m.getStaleDeviceSessionsFunc(ctx, timeout)
	}
	return nil, nil
}
func (m *mockPresenceRepo) CleanupStalePresence(ctx context.Context, timeout time.Duration) error {
	if m.cleanupStalePresenceFunc != nil {
		return m.cleanupStalePresenceFunc(ctx, timeout)
	}
	return nil
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func newUc(repo PresenceRepo) *PresenceUsecase {
	return NewPresenceUsecase(repo, 30*time.Second, 60*time.Second)
}

// ──────────────────────────────────────────────
// Tests: HandleClientConnected
// ──────────────────────────────────────────────

func TestHandleClientConnected_CreatesSessionAndSetsOnline(t *testing.T) {
	var createdSession *DeviceSession
	var setPresence *UserPresence

	repo := &mockPresenceRepo{
		createDeviceSessionFunc: func(ctx context.Context, s *DeviceSession) error {
			createdSession = s
			return nil
		},
		setUserPresenceFunc: func(ctx context.Context, p *UserPresence) error {
			setPresence = p
			return nil
		},
	}
	uc := newUc(repo)

	err := uc.HandleClientConnected(context.Background(), "client-1", 42, "iPhone 15", "192.168.1.1")
	if err != nil {
		t.Fatalf("HandleClientConnected() error = %v", err)
	}

	if createdSession == nil {
		t.Fatal("session was not created")
	}
	if createdSession.ClientID != "client-1" {
		t.Errorf("session.ClientID = %q, want %q", createdSession.ClientID, "client-1")
	}
	if createdSession.UserID != 42 {
		t.Errorf("session.UserID = %d, want %d", createdSession.UserID, 42)
	}
	if createdSession.DeviceInfo != "iPhone 15" {
		t.Errorf("session.DeviceInfo = %q, want %q", createdSession.DeviceInfo, "iPhone 15")
	}
	if createdSession.IP != "192.168.1.1" {
		t.Errorf("session.IP = %q, want %q", createdSession.IP, "192.168.1.1")
	}

	if setPresence == nil {
		t.Fatal("presence was not set")
	}
	if setPresence.UserID != 42 {
		t.Errorf("presence.UserID = %d, want %d", setPresence.UserID, 42)
	}
	if setPresence.Status != StatusOnline {
		t.Errorf("presence.Status = %q, want %q", setPresence.Status, StatusOnline)
	}
}

// ──────────────────────────────────────────────
// Tests: HandleClientDisconnected
// ──────────────────────────────────────────────

func TestHandleClientDisconnected_LastSession_SetsOffline(t *testing.T) {
	repo := &mockPresenceRepo{
		getDeviceSessionFunc: func(ctx context.Context, cid string) (*DeviceSession, error) {
			return &DeviceSession{ClientID: cid, UserID: 42}, nil
		},
		disconnectDeviceSessionFunc: func(ctx context.Context, cid string) error {
			return nil
		},
		getUserDeviceSessionsFunc: func(ctx context.Context, uid int) ([]*DeviceSession, error) {
			// Only this session exists (already disconnected since we called DisconnectDeviceSession)
			return []*DeviceSession{}, nil
		},
		setUserPresenceFunc: func(ctx context.Context, p *UserPresence) error {
			if p.Status != StatusOffline {
				t.Errorf("presence.Status = %q, want %q", p.Status, StatusOffline)
			}
			return nil
		},
	}
	uc := newUc(repo)

	err := uc.HandleClientDisconnected(context.Background(), "client-1")
	if err != nil {
		t.Fatalf("HandleClientDisconnected() error = %v", err)
	}
}

func TestHandleClientDisconnected_OtherActiveSession_StaysOnline(t *testing.T) {
	var presenceSet bool
	repo := &mockPresenceRepo{
		getDeviceSessionFunc: func(ctx context.Context, cid string) (*DeviceSession, error) {
			return &DeviceSession{ClientID: cid, UserID: 42}, nil
		},
		disconnectDeviceSessionFunc: func(ctx context.Context, cid string) error {
			return nil
		},
		getUserDeviceSessionsFunc: func(ctx context.Context, uid int) ([]*DeviceSession, error) {
			return []*DeviceSession{
				{ClientID: "other-client", UserID: 42, DisconnectedAt: nil},
			}, nil
		},
		setUserPresenceFunc: func(ctx context.Context, p *UserPresence) error {
			presenceSet = true
			return nil
		},
	}
	uc := newUc(repo)

	err := uc.HandleClientDisconnected(context.Background(), "client-1")
	if err != nil {
		t.Fatalf("HandleClientDisconnected() error = %v", err)
	}
	if presenceSet {
		t.Error("presence should NOT be set when other active sessions exist")
	}
}

// ──────────────────────────────────────────────
// Tests: HandlePresenceUpdate
// ──────────────────────────────────────────────

func TestHandlePresenceUpdate_SetsStatus(t *testing.T) {
	var captured *UserPresence
	repo := &mockPresenceRepo{
		setUserPresenceFunc: func(ctx context.Context, p *UserPresence) error {
			captured = p
			return nil
		},
	}
	uc := newUc(repo)

	payload := []byte(`{"user_id":42,"status":"away","custom_status":"brb","timestamp":"2026-01-01T00:00:00Z"}`)
	err := uc.HandlePresenceUpdate(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandlePresenceUpdate() error = %v", err)
	}
	if captured == nil {
		t.Fatal("presence was not set")
	}
	if captured.UserID != 42 {
		t.Errorf("UserID = %d, want %d", captured.UserID, 42)
	}
	if captured.Status != StatusAway {
		t.Errorf("Status = %q, want %q", captured.Status, StatusAway)
	}
	if captured.CustomStatus != "brb" {
		t.Errorf("CustomStatus = %q, want %q", captured.CustomStatus, "brb")
	}
}

func TestHandlePresenceUpdate_InvalidJSON_ReturnsError(t *testing.T) {
	uc := newUc(&mockPresenceRepo{})
	err := uc.HandlePresenceUpdate(context.Background(), []byte(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ──────────────────────────────────────────────
// Tests: HandleHeartbeat
// ──────────────────────────────────────────────

func TestHandleHeartbeat_UpdatesLastHeartbeat(t *testing.T) {
	var updated *DeviceSession
	repo := &mockPresenceRepo{
		getDeviceSessionFunc: func(ctx context.Context, cid string) (*DeviceSession, error) {
			return &DeviceSession{
				ClientID:      cid,
				UserID:        42,
				LastHeartbeat: time.Now().Add(-1 * time.Minute),
			}, nil
		},
		updateDeviceSessionFunc: func(ctx context.Context, s *DeviceSession) error {
			updated = s
			return nil
		},
	}
	uc := newUc(repo)

	now := time.Now()
	payload := []byte(`{"user_id":42,"client_id":"client-1","timestamp":"` + now.Format(time.RFC3339Nano) + `"}`)
	err := uc.HandleHeartbeat(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleHeartbeat() error = %v", err)
	}
	if updated == nil {
		t.Fatal("session was not updated")
	}
}

func TestHandleHeartbeat_SessionNotFound_ReturnsError(t *testing.T) {
	repo := &mockPresenceRepo{
		getDeviceSessionFunc: func(ctx context.Context, cid string) (*DeviceSession, error) {
			return nil, ErrSessionNotFound
		},
	}
	uc := newUc(repo)

	err := uc.HandleHeartbeat(context.Background(), []byte(`{"user_id":1,"client_id":"unknown","timestamp":"2026-01-01T00:00:00Z"}`))
	if err != ErrSessionNotFound {
		t.Errorf("error = %v, want %v", err, ErrSessionNotFound)
	}
}

// ──────────────────────────────────────────────
// Tests: GetUserPresence / GetMultipleUserPresence
// ──────────────────────────────────────────────

func TestGetUserPresence_ReturnsPresence(t *testing.T) {
	repo := &mockPresenceRepo{
		getUserPresenceFunc: func(ctx context.Context, uid int) (*UserPresence, error) {
			return &UserPresence{UserID: uid, Status: StatusOnline, LastSeen: time.Now()}, nil
		},
	}
	uc := newUc(repo)

	p, err := uc.GetUserPresence(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetUserPresence() error = %v", err)
	}
	if p.UserID != 42 {
		t.Errorf("UserID = %d, want %d", p.UserID, 42)
	}
	if p.Status != StatusOnline {
		t.Errorf("Status = %q, want %q", p.Status, StatusOnline)
	}
}

func TestGetMultipleUserPresence_ReturnsMap(t *testing.T) {
	repo := &mockPresenceRepo{
		getMultipleUserPresenceFunc: func(ctx context.Context, uids []int) (map[int]*UserPresence, error) {
			return map[int]*UserPresence{
				1: {UserID: 1, Status: StatusOnline},
				2: {UserID: 2, Status: StatusAway},
			}, nil
		},
	}
	uc := newUc(repo)

	m, err := uc.GetMultipleUserPresence(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatalf("GetMultipleUserPresence() error = %v", err)
	}
	if len(m) != 2 {
		t.Errorf("len = %d, want 2", len(m))
	}
	if m[1].Status != StatusOnline {
		t.Errorf("user 1 status = %q, want %q", m[1].Status, StatusOnline)
	}
	if m[2].Status != StatusAway {
		t.Errorf("user 2 status = %q, want %q", m[2].Status, StatusAway)
	}
}

// ──────────────────────────────────────────────
// Tests: SetUserStatus
// ──────────────────────────────────────────────

func TestSetUserStatus_SetsStatus(t *testing.T) {
	var captured *UserPresence
	repo := &mockPresenceRepo{
		setUserPresenceFunc: func(ctx context.Context, p *UserPresence) error {
			captured = p
			return nil
		},
	}
	uc := newUc(repo)

	err := uc.SetUserStatus(context.Background(), 42, StatusDoNotDisturb, "in a meeting")
	if err != nil {
		t.Fatalf("SetUserStatus() error = %v", err)
	}
	if captured == nil {
		t.Fatal("presence was not set")
	}
	if captured.Status != StatusDoNotDisturb {
		t.Errorf("Status = %q, want %q", captured.Status, StatusDoNotDisturb)
	}
	if captured.CustomStatus != "in a meeting" {
		t.Errorf("CustomStatus = %q, want %q", captured.CustomStatus, "in a meeting")
	}
}

// ──────────────────────────────────────────────
// Tests: NewPresenceUsecaseFromConfig
// ──────────────────────────────────────────────

func TestNewPresenceUsecaseFromConfig_ReturnsValid(t *testing.T) {
	uc := NewPresenceUsecaseFromConfig(&mockPresenceRepo{})
	if uc == nil {
		t.Fatal("NewPresenceUsecaseFromConfig() returned nil")
	}
	if uc.heartbeatInterval != 30*time.Second {
		t.Errorf("heartbeatInterval = %v, want 30s", uc.heartbeatInterval)
	}
	if uc.offlineTimeout != 60*time.Second {
		t.Errorf("offlineTimeout = %v, want 60s", uc.offlineTimeout)
	}
}

// ──────────────────────────────────────────────
// Tests: CleanupStalePresence
// ──────────────────────────────────────────────

func TestCleanupStalePresence_DisconnectsStaleSessions(t *testing.T) {
	var disconnected int
	repo := &mockPresenceRepo{
		getStaleDeviceSessionsFunc: func(ctx context.Context, timeout time.Duration) ([]*DeviceSession, error) {
			return []*DeviceSession{
				{ClientID: "stale-1", UserID: 1},
				{ClientID: "stale-2", UserID: 2},
			}, nil
		},
		getDeviceSessionFunc: func(ctx context.Context, cid string) (*DeviceSession, error) {
			return &DeviceSession{ClientID: cid, UserID: 1}, nil
		},
		disconnectDeviceSessionFunc: func(ctx context.Context, cid string) error {
			disconnected++
			return nil
		},
		getUserDeviceSessionsFunc: func(ctx context.Context, uid int) ([]*DeviceSession, error) {
			return []*DeviceSession{}, nil // No other sessions — set offline
		},
		setUserPresenceFunc: func(ctx context.Context, p *UserPresence) error {
			return nil
		},
		cleanupStalePresenceFunc: func(ctx context.Context, timeout time.Duration) error {
			return nil
		},
	}
	uc := newUc(repo)

	err := uc.CleanupStalePresence(context.Background())
	if err != nil {
		t.Fatalf("CleanupStalePresence() error = %v", err)
	}
	if disconnected != 2 {
		t.Errorf("disconnected = %d, want 2", disconnected)
	}
}

// ──────────────────────────────────────────────
// Tests: GetUserDeviceSessions
// ──────────────────────────────────────────────

func TestGetUserDeviceSessions_ReturnsSessions(t *testing.T) {
	repo := &mockPresenceRepo{
		getUserDeviceSessionsFunc: func(ctx context.Context, uid int) ([]*DeviceSession, error) {
			return []*DeviceSession{
				{ClientID: "c1", UserID: uid},
				{ClientID: "c2", UserID: uid},
			}, nil
		},
	}
	uc := newUc(repo)

	sessions, err := uc.GetUserDeviceSessions(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetUserDeviceSessions() error = %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("len(sessions) = %d, want 2", len(sessions))
	}
}
