package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/auth-service/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/auth"
)

// mockAuthRepo implements biz.AuthRepo for testing
type mockAuthRepo struct {
	getUserByIDFunc func(ctx context.Context, id int) (*biz.User, error)
}

func (m *mockAuthRepo) CreateUser(ctx context.Context, user *biz.User) error {
	return nil
}

func (m *mockAuthRepo) GetUserByEmail(ctx context.Context, email string, orgID uuid.UUID) (*biz.User, error) {
	return nil, biz.ErrUserNotFound
}

func (m *mockAuthRepo) GetUserByEmailAnyOrg(ctx context.Context, email string) (*biz.User, error) {
	return nil, biz.ErrUserNotFound
}

func (m *mockAuthRepo) GetUserByID(ctx context.Context, id int) (*biz.User, error) {
	if m.getUserByIDFunc != nil {
		return m.getUserByIDFunc(ctx, id)
	}
	return nil, biz.ErrUserNotFound
}

func (m *mockAuthRepo) GetUserByUsername(ctx context.Context, username string, orgID uuid.UUID) (*biz.User, error) {
	return nil, biz.ErrUserNotFound
}

func (m *mockAuthRepo) GetUserByKeycloakID(ctx context.Context, keycloakID string) (*biz.User, error) {
	return nil, biz.ErrUserNotFound
}

func (m *mockAuthRepo) SearchUsersByUsername(ctx context.Context, query string, orgID uuid.UUID, limit int) ([]*biz.User, error) {
	return nil, nil
}

func (m *mockAuthRepo) GetOrganizationUsers(ctx context.Context, orgID uuid.UUID) ([]*biz.User, error) {
	return nil, nil
}

func (m *mockAuthRepo) UpdateUser(ctx context.Context, userID int, req *biz.UpdateUserRequest) error {
	return nil
}

func (m *mockAuthRepo) DeleteUser(ctx context.Context, userID int) error {
	return nil
}

func (m *mockAuthRepo) UpdateLastSeen(ctx context.Context, userID int) error {
	return nil
}

func (m *mockAuthRepo) CreateOrganization(ctx context.Context, org *biz.Organization) error {
	return nil
}

func (m *mockAuthRepo) GetOrganization(ctx context.Context, id uuid.UUID) (*biz.Organization, error) {
	return nil, nil
}

// testUser returns a standard test user
func testUser(id int, orgID uuid.UUID) *biz.User {
	return &biz.User{
		ID:             id,
		OrganizationID: orgID,
		Email:          "test@example.com",
		Username:       "testuser",
		DisplayName:    "Test User",
		Role:           biz.UserRoleMember,
		CreatedAt:      time.Now(),
	}
}

// setupTestServer creates an HTTPServer with a mock repo for testing
func setupTestServer(repo *mockAuthRepo) (*HTTPServer, *biz.AuthUsecase) {
	uc, _ := biz.NewAuthUsecase(repo, "test-secret", time.Hour, biz.KeycloakConfig{})
	s := NewHTTPServer(uc)
	return s, uc
}

// setClaimsContext adds JWT claims to the request context, bypassing the auth middleware
func setClaimsContext(r *http.Request, userID int, orgID uuid.UUID) *http.Request {
	claims := &auth.JWTClaims{
		UserID:         userID,
		OrganizationID: orgID.String(),
		Email:          "test@example.com",
		Role:           "member",
	}
	ctx := auth.SetClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// ------------------------
// handleGetMe tests
// ------------------------

func TestHandleGetMe_UserFound(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{
		getUserByIDFunc: func(ctx context.Context, id int) (*biz.User, error) {
			if id == 1 {
				return testUser(1, orgID), nil
			}
			return nil, biz.ErrUserNotFound
		},
	}
	s, _ := setupTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req = setClaimsContext(req, 1, orgID)
	w := httptest.NewRecorder()

	s.handleGetMe(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var respUser biz.User
	if err := json.NewDecoder(w.Body).Decode(&respUser); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if respUser.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", respUser.Email)
	}
	// Password hash should never be returned
	if respUser.PasswordHash != "" {
		t.Error("password hash should be empty in response")
	}
}

func TestHandleGetMe_UserNotFound(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{
		getUserByIDFunc: func(ctx context.Context, id int) (*biz.User, error) {
			return nil, biz.ErrUserNotFound
		},
	}
	s, _ := setupTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req = setClaimsContext(req, 1, orgID)
	w := httptest.NewRecorder()

	s.handleGetMe(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for not-found user, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "User not found" {
		t.Errorf("expected 'User not found', got '%s'", errResp["error"])
	}
}

func TestHandleGetMe_DBError(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{
		getUserByIDFunc: func(ctx context.Context, id int) (*biz.User, error) {
			return nil, errors.New("connection refused")
		},
	}
	s, _ := setupTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req = setClaimsContext(req, 1, orgID)
	w := httptest.NewRecorder()

	s.handleGetMe(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for DB error, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "connection refused" {
		t.Errorf("expected 'connection refused' error message, got '%s'", errResp["error"])
	}
}

// ------------------------
// handleGetUser tests
// ------------------------

func TestHandleGetUser_UserFoundSameOrg(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{
		getUserByIDFunc: func(ctx context.Context, id int) (*biz.User, error) {
			return testUser(id, orgID), nil
		},
	}
	s, _ := setupTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/users/2", nil)
	req = setClaimsContext(req, 1, orgID)
	req = mux.SetURLVars(req, map[string]string{"id": "2"})
	w := httptest.NewRecorder()

	s.handleGetUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var respUser biz.User
	if err := json.NewDecoder(w.Body).Decode(&respUser); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if respUser.ID != 2 {
		t.Errorf("expected user ID 2, got %d", respUser.ID)
	}
}

func TestHandleGetUser_TargetUserNotFound(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{
		getUserByIDFunc: func(ctx context.Context, id int) (*biz.User, error) {
			// First call is for target user (id=999) — not found, handler returns 404 immediately
			return nil, biz.ErrUserNotFound
		},
	}
	s, _ := setupTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/users/999", nil)
	req = setClaimsContext(req, 1, orgID)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	w := httptest.NewRecorder()

	s.handleGetUser(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for not-found target user, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "User not found" {
		t.Errorf("expected 'User not found', got '%s'", errResp["error"])
	}
}

func TestHandleGetUser_TargetUserDBError(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{
		getUserByIDFunc: func(ctx context.Context, id int) (*biz.User, error) {
			// First call is for target user (id=2) — DB error, handler returns 500 immediately
			return nil, errors.New("database timeout")
		},
	}
	s, _ := setupTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/users/2", nil)
	req = setClaimsContext(req, 1, orgID)
	req = mux.SetURLVars(req, map[string]string{"id": "2"})
	w := httptest.NewRecorder()

	s.handleGetUser(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for DB error, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "database timeout" {
		t.Errorf("expected 'database timeout', got '%s'", errResp["error"])
	}
}

func TestHandleGetUser_RequesterNotFound(t *testing.T) {
	orgID := uuid.New()
	callCount := 0
	repo := &mockAuthRepo{
		getUserByIDFunc: func(ctx context.Context, id int) (*biz.User, error) {
			callCount++
			if callCount == 1 {
				// First call is for the target user (id=2) — succeed
				return testUser(2, orgID), nil
			}
			// Second call is for requester (id=1) — not found
			return nil, biz.ErrUserNotFound
		},
	}
	s, _ := setupTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/users/2", nil)
	req = setClaimsContext(req, 1, orgID)
	req = mux.SetURLVars(req, map[string]string{"id": "2"})
	w := httptest.NewRecorder()

	s.handleGetUser(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for missing requester, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "Requester account not found" {
		t.Errorf("expected 'Requester account not found', got '%s'", errResp["error"])
	}
}

func TestHandleGetUser_RequesterDBError(t *testing.T) {
	orgID := uuid.New()
	callCount := 0
	repo := &mockAuthRepo{
		getUserByIDFunc: func(ctx context.Context, id int) (*biz.User, error) {
			callCount++
			if callCount == 1 {
				// First call is for target user (id=2) — succeed
				return testUser(2, orgID), nil
			}
			// Second call is for requester (id=1) — DB error
			return nil, errors.New("connection pool exhausted")
		},
	}
	s, _ := setupTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/users/2", nil)
	req = setClaimsContext(req, 1, orgID)
	req = mux.SetURLVars(req, map[string]string{"id": "2"})
	w := httptest.NewRecorder()

	s.handleGetUser(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for DB error, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "connection pool exhausted" {
		t.Errorf("expected 'connection pool exhausted', got '%s'", errResp["error"])
	}
}

func TestHandleGetUser_InvalidUserID(t *testing.T) {
	orgID := uuid.New()
	repo := &mockAuthRepo{}
	s, _ := setupTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/users/abc", nil)
	req = setClaimsContext(req, 1, orgID)
	req = mux.SetURLVars(req, map[string]string{"id": "abc"})
	w := httptest.NewRecorder()

	s.handleGetUser(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid user ID, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "Invalid user ID" {
		t.Errorf("expected 'Invalid user ID', got '%s'", errResp["error"])
	}
}

func TestHandleGetUser_DifferentOrg(t *testing.T) {
	orgID := uuid.New()
	otherOrgID := uuid.New()
	callCount := 0
	repo := &mockAuthRepo{
		getUserByIDFunc: func(ctx context.Context, id int) (*biz.User, error) {
			callCount++
			if callCount == 1 {
				// First call is for target user (id=2) — different org
				return testUser(2, otherOrgID), nil
			}
			// Second call is for requester (id=1)
			return testUser(1, orgID), nil
		},
	}
	s, _ := setupTestServer(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/users/2", nil)
	req = setClaimsContext(req, 1, orgID)
	req = mux.SetURLVars(req, map[string]string{"id": "2"})
	w := httptest.NewRecorder()

	s.handleGetUser(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for different org, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "Cannot view users from other organizations" {
		t.Errorf("expected 'Cannot view users from other organizations', got '%s'", errResp["error"])
	}
}
