package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/auth-service/internal/biz"
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
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID, X-Organization-ID")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.router.ServeHTTP(w, r)
}

func (s *HTTPServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req biz.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	user, token, err := s.authUc.Register(r.Context(), &req)
	if err != nil {
		if err == biz.ErrUserExists {
			s.writeError(w, http.StatusConflict, "User already exists")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"user":  user,
		"token": token,
	}
	s.writeJSON(w, http.StatusCreated, response)
}

func (s *HTTPServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req biz.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
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
			s.writeError(w, http.StatusBadRequest, "Invalid organization ID")
			return
		}
	}

	user, token, err := s.authUc.Login(r.Context(), &req, orgID)
	if err != nil {
		if err == biz.ErrUserNotFound || err == biz.ErrInvalidPassword {
			s.writeError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"user":  user,
		"token": token,
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleValidateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	claims, err := s.authUc.ValidateToken(r.Context(), req.Token)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	s.writeJSON(w, http.StatusOK, claims)
}

func (s *HTTPServer) handleGetMe(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*biz.JWTClaims)
	userID := claims.UserID

	user, err := s.authUc.GetUser(r.Context(), userID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, user)
}

func (s *HTTPServer) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	var req biz.OIDCLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Get organization ID from header or query param
	orgIDStr := r.Header.Get("X-Organization-ID")
	if orgIDStr == "" {
		orgIDStr = r.URL.Query().Get("org_id")
	}
	if orgIDStr == "" {
		s.writeError(w, http.StatusBadRequest, "Organization ID is required")
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid organization ID")
		return
	}

	user, token, err := s.authUc.OIDCLogin(r.Context(), &req, orgID)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "OIDC authentication failed")
		return
	}

	response := map[string]interface{}{
		"user":  user,
		"token": token,
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleMQTTCredentials(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*biz.JWTClaims)
	userID := claims.UserID

	username, password, err := s.authUc.GenerateMQTTCredentials(r.Context(), userID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"username": username,
		"password": password,
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *HTTPServer) handleGetOrganizationUsers(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*biz.JWTClaims)
	orgID, _ := uuid.Parse(claims.OrganizationID)

	users, err := s.authUc.GetOrganizationUsers(r.Context(), orgID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, users)
}

func (s *HTTPServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.writeError(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			s.writeError(w, http.StatusUnauthorized, "Invalid authorization format")
			return
		}

		claims, err := s.authUc.ValidateToken(r.Context(), tokenString)
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), "claims", claims)
		next(w, r.WithContext(ctx))
	}
}

func (s *HTTPServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *HTTPServer) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// handleGetUser gets a specific user by ID
func (s *HTTPServer) handleGetUser(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*biz.JWTClaims)
	requesterID := claims.UserID

	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	user, err := s.authUc.GetUser(r.Context(), userID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "User not found")
		return
	}

	// Check if requester can view this user (same organization)
	requester, err := s.authUc.GetUser(r.Context(), requesterID)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if user.OrganizationID != requester.OrganizationID {
		s.writeError(w, http.StatusForbidden, "Cannot view users from other organizations")
		return
	}

	s.writeJSON(w, http.StatusOK, user)
}

// handleUpdateUser updates a user
func (s *HTTPServer) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*biz.JWTClaims)
	requesterID := claims.UserID

	vars := mux.Vars(r)
	targetUserID, err := strconv.Atoi(vars["id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req biz.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := s.authUc.UpdateUser(r.Context(), requesterID, targetUserID, &req); err != nil {
		if err.Error() == "insufficient permissions" {
			s.writeError(w, http.StatusForbidden, "Insufficient permissions")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return updated user
	user, err := s.authUc.GetUser(r.Context(), targetUserID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to get updated user")
		return
	}

	s.writeJSON(w, http.StatusOK, user)
}

// handleDeleteUser deletes a user (admin only)
func (s *HTTPServer) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*biz.JWTClaims)
	requesterID := claims.UserID

	vars := mux.Vars(r)
	targetUserID, err := strconv.Atoi(vars["id"])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if err := s.authUc.DeleteUser(r.Context(), requesterID, targetUserID); err != nil {
		if err.Error() == "insufficient permissions" {
			s.writeError(w, http.StatusForbidden, "Insufficient permissions")
			return
		}
		if err.Error() == "cannot delete yourself" {
			s.writeError(w, http.StatusBadRequest, "Cannot delete yourself")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "User deleted successfully"})
}
