package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/auth-service/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/auth"
	sharedserver "github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/server"
)

type HTTPServer struct {
	authUc *biz.AuthUsecase
	router *mux.Router
}

func NewHTTPServer(authUc *biz.AuthUsecase) *HTTPServer {
	s := &HTTPServer{
		authUc: authUc,
		router: mux.NewRouter(),
	}
	s.setupRoutes()
	return s
}

func (s *HTTPServer) setupRoutes() {
	api := s.router.PathPrefix("/api/v1").Subrouter()

	api.HandleFunc("/auth/register", s.handleRegister).Methods("POST")
	api.HandleFunc("/auth/login", s.handleLogin).Methods("POST")
	api.HandleFunc("/auth/oidc/login", s.handleOIDCLogin).Methods("POST")
	api.HandleFunc("/auth/validate", s.handleValidateToken).Methods("POST")
	api.HandleFunc("/auth/me", s.authMiddleware(s.handleGetMe)).Methods("GET")
	api.HandleFunc("/auth/mqtt-credentials", s.authMiddleware(s.handleMQTTCredentials)).Methods("GET")

	// User management endpoints
	api.HandleFunc("/auth/users", s.authMiddleware(s.handleGetOrganizationUsers)).Methods("GET")
	api.HandleFunc("/auth/users/search", s.authMiddleware(s.handleSearchUsers)).Methods("GET")
	api.HandleFunc("/auth/users/username/{username}", s.authMiddleware(s.handleGetUserByUsername)).Methods("GET")
	api.HandleFunc("/auth/users/{id}", s.authMiddleware(s.handleGetUser)).Methods("GET")
	api.HandleFunc("/auth/users/{id}", s.authMiddleware(s.handleUpdateUser)).Methods("PUT")
	api.HandleFunc("/auth/users/{id}", s.authMiddleware(s.handleDeleteUser)).Methods("DELETE")

	// Health check
	s.router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *HTTPServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req biz.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	user, token, err := s.authUc.Register(r.Context(), &req)
	if err != nil {
		if errors.Is(err, biz.ErrUserExists) {
			sharedserver.WriteError(w, http.StatusConflict, "User already exists")
			return
		}
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"user":  user,
		"token": token,
	}
	sharedserver.WriteJSON(w, http.StatusCreated, response)
}

func (s *HTTPServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req biz.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Get organization ID from header or query param (optional)
	orgIDStr := r.Header.Get("X-Organization-ID")
	if orgIDStr == "" {
		orgIDStr = r.URL.Query().Get("org_id")
	}

	var orgID uuid.UUID
	if orgIDStr != "" && orgIDStr != "00000000-0000-0000-0000-000000000000" {
		var err error
		orgID, err = uuid.Parse(orgIDStr)
		if err != nil {
			sharedserver.WriteError(w, http.StatusBadRequest, "Invalid organization ID")
			return
		}
	}

	user, token, err := s.authUc.Login(r.Context(), &req, orgID)
	if err != nil {
		if errors.Is(err, biz.ErrUserNotFound) || errors.Is(err, biz.ErrInvalidPassword) {
			sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"user":  user,
		"token": token,
	}
	sharedserver.WriteJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleValidateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	claims, err := s.authUc.ValidateToken(r.Context(), req.Token)
	if err != nil {
		sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, claims)
}

func (s *HTTPServer) handleGetMe(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserID(r.Context())
	if err != nil {
		sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}

	user, err := s.authUc.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, biz.ErrUserNotFound) {
			sharedserver.WriteError(w, http.StatusNotFound, "User not found")
			return
		}
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, user)
}

func (s *HTTPServer) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	var req biz.OIDCLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Get organization ID from header or query param
	orgIDStr := r.Header.Get("X-Organization-ID")
	if orgIDStr == "" {
		orgIDStr = r.URL.Query().Get("org_id")
	}
	if orgIDStr == "" {
		sharedserver.WriteError(w, http.StatusBadRequest, "Organization ID is required")
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	user, token, err := s.authUc.OIDCLogin(r.Context(), &req, orgID)
	if err != nil {
		sharedserver.WriteError(w, http.StatusUnauthorized, "OIDC authentication failed")
		return
	}

	response := map[string]interface{}{
		"user":  user,
		"token": token,
	}
	sharedserver.WriteJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleMQTTCredentials(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserID(r.Context())
	if err != nil {
		sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}

	username, password, err := s.authUc.GenerateMQTTCredentials(r.Context(), userID)
	if err != nil {
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"username": username,
		"password": password,
	}
	sharedserver.WriteJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleGetOrganizationUsers(w http.ResponseWriter, r *http.Request) {
	orgID, err := auth.GetOrgID(r.Context())
	if err != nil {
		sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}

	users, err := s.authUc.GetOrganizationUsers(r.Context(), orgID)
	if err != nil {
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, users)
}

func (s *HTTPServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			sharedserver.WriteError(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid authorization format")
			return
		}

		bizClaims, err := s.authUc.ValidateToken(r.Context(), tokenString)
		if err != nil {
			sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Convert biz.JWTClaims to shared auth.JWTClaims for consistent context management
		claims := &auth.JWTClaims{
			UserID:           bizClaims.UserID,
			OrganizationID:   bizClaims.OrganizationID,
			Email:            bizClaims.Email,
			Role:             bizClaims.Role,
			KeycloakID:       bizClaims.KeycloakID,
			RegisteredClaims: bizClaims.RegisteredClaims,
		}
		ctx := auth.SetClaims(r.Context(), claims)
		next(w, r.WithContext(ctx))
	}
}

// handleGetUser gets a specific user by ID
func (s *HTTPServer) handleGetUser(w http.ResponseWriter, r *http.Request) {
	requesterID, err := auth.GetUserID(r.Context())
	if err != nil {
		sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}

	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["id"])
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := s.authUc.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, biz.ErrUserNotFound) {
			sharedserver.WriteError(w, http.StatusNotFound, "User not found")
			return
		}
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Check if requester can view this user (same organization)
	requester, err := s.authUc.GetUser(r.Context(), requesterID)
	if err != nil {
		if errors.Is(err, biz.ErrUserNotFound) {
			sharedserver.WriteError(w, http.StatusForbidden, "Requester account not found")
			return
		}
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if user.OrganizationID != requester.OrganizationID {
		sharedserver.WriteError(w, http.StatusForbidden, "Cannot view users from other organizations")
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, user)
}

// handleUpdateUser updates a user
func (s *HTTPServer) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	requesterID, err := auth.GetUserID(r.Context())
	if err != nil {
		sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}

	vars := mux.Vars(r)
	targetUserID, err := strconv.Atoi(vars["id"])
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req biz.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := s.authUc.UpdateUser(r.Context(), requesterID, targetUserID, &req); err != nil {
		if errors.Is(err, biz.ErrInsufficientPermissions) {
			sharedserver.WriteError(w, http.StatusForbidden, "Insufficient permissions")
			return
		}
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return updated user
	user, err := s.authUc.GetUser(r.Context(), targetUserID)
	if err != nil {
		sharedserver.WriteError(w, http.StatusInternalServerError, "Failed to get updated user")
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, user)
}

// handleDeleteUser deletes a user (admin only)
func (s *HTTPServer) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	requesterID, err := auth.GetUserID(r.Context())
	if err != nil {
		sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}

	vars := mux.Vars(r)
	targetUserID, err := strconv.Atoi(vars["id"])
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if err := s.authUc.DeleteUser(r.Context(), requesterID, targetUserID); err != nil {
		if errors.Is(err, biz.ErrInsufficientPermissions) {
			sharedserver.WriteError(w, http.StatusForbidden, "Insufficient permissions")
			return
		}
		if errors.Is(err, biz.ErrCannotDeleteSelf) {
			sharedserver.WriteError(w, http.StatusBadRequest, "Cannot delete yourself")
			return
		}
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"message": "User deleted successfully"})
}

// handleSearchUsers searches for users by username or display name
func (s *HTTPServer) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	requesterID, err := auth.GetUserID(r.Context())
	if err != nil {
		sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		sharedserver.WriteError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	// Remove @ prefix if present (Instagram-like search)
	query = strings.TrimPrefix(query, "@")

	limitStr := r.URL.Query().Get("limit")
	limit := 10 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}

	users, err := s.authUc.SearchUsersByUsername(r.Context(), query, requesterID, limit)
	if err != nil {
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, users)
}

// handleGetUserByUsername gets a user by their username
func (s *HTTPServer) handleGetUserByUsername(w http.ResponseWriter, r *http.Request) {
	requesterID, err := auth.GetUserID(r.Context())
	if err != nil {
		sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}

	vars := mux.Vars(r)
	username := vars["username"]

	// Remove @ prefix if present
	username = strings.TrimPrefix(username, "@")

	user, err := s.authUc.GetUserByUsername(r.Context(), username, requesterID)
	if err != nil {
		if errors.Is(err, biz.ErrUserNotFound) {
			sharedserver.WriteError(w, http.StatusNotFound, "User not found")
			return
		}
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, user)
}
