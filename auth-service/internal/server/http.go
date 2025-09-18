package server

import (
	"context"
	"encoding/json"
	"net/http"
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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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
	userID, _ := uuid.Parse(claims.UserID)

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
	userID, _ := uuid.Parse(claims.UserID)

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
