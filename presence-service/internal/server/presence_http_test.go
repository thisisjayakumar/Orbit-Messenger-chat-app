package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/presence-service/internal/biz"
)

// --------------- mocks ---------------

type mockPresenceRepo struct {
	setUserPresenceFunc         func(ctx context.Context, p *biz.UserPresence) error
	getUserPresenceFunc         func(ctx context.Context, uid int) (*biz.UserPresence, error)
	getMultipleUserPresenceFunc func(ctx context.Context, uids []int) (map[int]*biz.UserPresence, error)
	createDeviceSessionFunc     func(ctx context.Context, s *biz.DeviceSession) error
	updateDeviceSessionFunc     func(ctx context.Context, s *biz.DeviceSession) error
	getDeviceSessionFunc        func(ctx context.Context, cid string) (*biz.DeviceSession, error)
	getUserDeviceSessionsFunc   func(ctx context.Context, uid int) ([]*biz.DeviceSession, error)
	disconnectDeviceSessionFunc func(ctx context.Context, cid string) error
	getStaleDeviceSessionsFunc  func(ctx context.Context, d time.Duration) ([]*biz.DeviceSession, error)
	cleanupStalePresenceFunc    func(ctx context.Context, d time.Duration) error
}

func (m *mockPresenceRepo) SetUserPresence(ctx context.Context, p *biz.UserPresence) error {
	if m.setUserPresenceFunc != nil {
		return m.setUserPresenceFunc(ctx, p)
	}
	return nil
}

func (m *mockPresenceRepo) GetUserPresence(ctx context.Context, uid int) (*biz.UserPresence, error) {
	if m.getUserPresenceFunc != nil {
		return m.getUserPresenceFunc(ctx, uid)
	}
	return nil, nil
}

func (m *mockPresenceRepo) GetMultipleUserPresence(ctx context.Context, uids []int) (map[int]*biz.UserPresence, error) {
	if m.getMultipleUserPresenceFunc != nil {
		return m.getMultipleUserPresenceFunc(ctx, uids)
	}
	return nil, nil
}

func (m *mockPresenceRepo) CreateDeviceSession(ctx context.Context, s *biz.DeviceSession) error {
	if m.createDeviceSessionFunc != nil {
		return m.createDeviceSessionFunc(ctx, s)
	}
	return nil
}

func (m *mockPresenceRepo) UpdateDeviceSession(ctx context.Context, s *biz.DeviceSession) error {
	if m.updateDeviceSessionFunc != nil {
		return m.updateDeviceSessionFunc(ctx, s)
	}
	return nil
}

func (m *mockPresenceRepo) GetDeviceSession(ctx context.Context, cid string) (*biz.DeviceSession, error) {
	if m.getDeviceSessionFunc != nil {
		return m.getDeviceSessionFunc(ctx, cid)
	}
	return nil, nil
}

func (m *mockPresenceRepo) GetUserDeviceSessions(ctx context.Context, uid int) ([]*biz.DeviceSession, error) {
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

func (m *mockPresenceRepo) GetStaleDeviceSessions(ctx context.Context, d time.Duration) ([]*biz.DeviceSession, error) {
	if m.getStaleDeviceSessionsFunc != nil {
		return m.getStaleDeviceSessionsFunc(ctx, d)
	}
	return nil, nil
}

func (m *mockPresenceRepo) CleanupStalePresence(ctx context.Context, d time.Duration) error {
	if m.cleanupStalePresenceFunc != nil {
		return m.cleanupStalePresenceFunc(ctx, d)
	}
	return nil
}

// --------------- helpers ---------------

func setupPresenceTestServer(repo biz.PresenceRepo) *PresenceHTTPServer {
	uc := biz.NewPresenceUsecase(repo, 30*time.Second, 5*time.Minute)
	return NewPresenceHTTPServer(uc, nil)
}

// --------------- GetUserPresence Tests ---------------

func TestHandleGetUserPresence_Success(t *testing.T) {
	now := time.Now()
	repo := &mockPresenceRepo{
		getUserPresenceFunc: func(ctx context.Context, uid int) (*biz.UserPresence, error) {
			return &biz.UserPresence{
				UserID:   uid,
				Status:   biz.StatusOnline,
				LastSeen: now,
			}, nil
		},
	}
	s := setupPresenceTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/presence/42", nil)
	req = mux.SetURLVars(req, map[string]string{"userID": "42"})
	w := httptest.NewRecorder()

	s.handleGetUserPresence(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp biz.UserPresence
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.UserID != 42 {
		t.Errorf("expected user ID 42, got %d", resp.UserID)
	}
	if resp.Status != biz.StatusOnline {
		t.Errorf("expected status 'online', got '%s'", resp.Status)
	}
}

func TestHandleGetUserPresence_InvalidUserID(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/presence/abc", nil)
	req = mux.SetURLVars(req, map[string]string{"userID": "abc"})
	w := httptest.NewRecorder()

	s.handleGetUserPresence(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Invalid user ID" {
		t.Errorf("expected 'Invalid user ID', got '%s'", resp["error"])
	}
}

// --------------- SetUserStatus Tests ---------------

func TestHandleSetUserStatus_Success(t *testing.T) {
	repo := &mockPresenceRepo{
		setUserPresenceFunc: func(ctx context.Context, p *biz.UserPresence) error {
			return nil
		},
	}
	s := setupPresenceTestServer(repo)

	body := bytes.NewReader([]byte(`{"status":"online","custom_status":"Available"}`))
	req := httptest.NewRequest(http.MethodPut, "/api/v1/presence/42/status", body)
	req = mux.SetURLVars(req, map[string]string{"userID": "42"})
	w := httptest.NewRecorder()

	s.handleSetUserStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "updated" {
		t.Errorf("expected 'updated', got '%s'", resp["status"])
	}
}

func TestHandleSetUserStatus_InvalidStatus(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	body := bytes.NewReader([]byte(`{"status":"invalid_status"}`))
	req := httptest.NewRequest(http.MethodPut, "/api/v1/presence/42/status", body)
	req = mux.SetURLVars(req, map[string]string{"userID": "42"})
	w := httptest.NewRecorder()

	s.handleSetUserStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Invalid status" {
		t.Errorf("expected 'Invalid status', got '%s'", resp["error"])
	}
}

func TestHandleSetUserStatus_InvalidJSON(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	body := bytes.NewReader([]byte(`not-json`))
	req := httptest.NewRequest(http.MethodPut, "/api/v1/presence/42/status", body)
	req = mux.SetURLVars(req, map[string]string{"userID": "42"})
	w := httptest.NewRecorder()

	s.handleSetUserStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleSetUserStatus_InvalidUserID(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	body := bytes.NewReader([]byte(`{"status":"online"}`))
	req := httptest.NewRequest(http.MethodPut, "/api/v1/presence/abc/status", body)
	req = mux.SetURLVars(req, map[string]string{"userID": "abc"})
	w := httptest.NewRecorder()

	s.handleSetUserStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --------------- GetMultipleUserPresence Tests ---------------

func TestHandleGetMultipleUserPresence_Success(t *testing.T) {
	now := time.Now()
	repo := &mockPresenceRepo{
		getMultipleUserPresenceFunc: func(ctx context.Context, uids []int) (map[int]*biz.UserPresence, error) {
			result := make(map[int]*biz.UserPresence)
			for _, uid := range uids {
				result[uid] = &biz.UserPresence{
					UserID:   uid,
					Status:   biz.StatusOnline,
					LastSeen: now,
				}
			}
			return result, nil
		},
	}
	s := setupPresenceTestServer(repo)

	body := bytes.NewReader([]byte(`{"user_ids":["1","2","3"]}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/presence/bulk", body)
	w := httptest.NewRecorder()

	s.handleGetMultipleUserPresence(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]*biz.UserPresence
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 3 {
		t.Errorf("expected 3 results, got %d", len(resp))
	}
}

func TestHandleGetMultipleUserPresence_NoIDs(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	body := bytes.NewReader([]byte(`{"user_ids":[]}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/presence/bulk", body)
	w := httptest.NewRecorder()

	s.handleGetMultipleUserPresence(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "No user IDs provided" {
		t.Errorf("expected 'No user IDs provided', got '%s'", resp["error"])
	}
}

func TestHandleGetMultipleUserPresence_TooManyIDs(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	ids := make([]string, 101)
	for i := range ids {
		ids[i] = "1"
	}
	payload, _ := json.Marshal(map[string][]string{"user_ids": ids})
	body := bytes.NewReader(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/presence/bulk", body)
	w := httptest.NewRecorder()

	s.handleGetMultipleUserPresence(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleGetMultipleUserPresence_InvalidUserID(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	body := bytes.NewReader([]byte(`{"user_ids":["1","abc"]}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/presence/bulk", body)
	w := httptest.NewRecorder()

	s.handleGetMultipleUserPresence(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleGetMultipleUserPresence_InvalidJSON(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	body := bytes.NewReader([]byte(`not-json`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/presence/bulk", body)
	w := httptest.NewRecorder()

	s.handleGetMultipleUserPresence(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --------------- GetUserSessions Tests ---------------

func TestHandleGetUserSessions_Success(t *testing.T) {
	now := time.Now()
	repo := &mockPresenceRepo{
		getUserDeviceSessionsFunc: func(ctx context.Context, uid int) ([]*biz.DeviceSession, error) {
			return []*biz.DeviceSession{
				{
					ID:            uuid.New(),
					UserID:        uid,
					ClientID:      "client-1",
					DeviceInfo:    "iPhone 15",
					IP:            "192.168.1.1",
					ConnectedAt:   now,
					LastHeartbeat: now,
				},
			}, nil
		},
	}
	s := setupPresenceTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/presence/42/sessions", nil)
	req = mux.SetURLVars(req, map[string]string{"userID": "42"})
	w := httptest.NewRecorder()

	s.handleGetUserSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp []*biz.DeviceSession
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 session, got %d", len(resp))
	}
	if resp[0].ClientID != "client-1" {
		t.Errorf("expected ClientID 'client-1', got '%s'", resp[0].ClientID)
	}
}

func TestHandleGetUserSessions_InvalidUserID(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/presence/abc/sessions", nil)
	req = mux.SetURLVars(req, map[string]string{"userID": "abc"})
	w := httptest.NewRecorder()

	s.handleGetUserSessions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --------------- ClientConnect Tests ---------------

func TestHandleClientConnect_Success(t *testing.T) {
	repo := &mockPresenceRepo{
		createDeviceSessionFunc: func(ctx context.Context, s *biz.DeviceSession) error {
			return nil
		},
		setUserPresenceFunc: func(ctx context.Context, p *biz.UserPresence) error {
			return nil
		},
	}
	s := setupPresenceTestServer(repo)

	body := bytes.NewReader([]byte(`{"client_id":"client-abc","user_id":42,"device_info":"Chrome"}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/presence/connect", body)
	w := httptest.NewRecorder()

	s.handleClientConnect(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "connected" {
		t.Errorf("expected 'connected', got '%s'", resp["status"])
	}
}

func TestHandleClientConnect_MissingFields(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	body := bytes.NewReader([]byte(`{"client_id":"","user_id":0}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/presence/connect", body)
	w := httptest.NewRecorder()

	s.handleClientConnect(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleClientConnect_InvalidJSON(t *testing.T) {
	s := setupPresenceTestServer(&mockPresenceRepo{})

	body := bytes.NewReader([]byte(`not-json`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/presence/connect", body)
	w := httptest.NewRecorder()

	s.handleClientConnect(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
