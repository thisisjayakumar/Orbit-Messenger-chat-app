package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/chat-api/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/auth"
	sharedserver "github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/server"
)

type ChatHTTPServer struct {
	chatUc    *biz.ChatUsecase
	router    *mux.Router
	jwtSecret string
}

func NewChatHTTPServer(chatUc *biz.ChatUsecase, jwtSecret string) *ChatHTTPServer {
	s := &ChatHTTPServer{
		chatUc:    chatUc,
		router:    mux.NewRouter(),
		jwtSecret: jwtSecret,
	}
	s.setupRoutes()
	return s
}

func (s *ChatHTTPServer) setupRoutes() {
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Conversations
	api.HandleFunc("/conversations", s.authMiddleware(s.handleCreateConversation)).Methods("POST")
	api.HandleFunc("/conversations", s.authMiddleware(s.handleGetUserConversations)).Methods("GET")
	api.HandleFunc("/conversations/{conversationID}", s.authMiddleware(s.handleGetConversation)).Methods("GET")
	api.HandleFunc("/conversations/{conversationID}", s.authMiddleware(s.handleUpdateConversation)).Methods("PUT")

	// Participants
	api.HandleFunc("/conversations/{conversationID}/participants", s.authMiddleware(s.handleGetParticipants)).Methods("GET")
	api.HandleFunc("/conversations/{conversationID}/participants", s.authMiddleware(s.handleAddParticipant)).Methods("POST")
	api.HandleFunc("/conversations/{conversationID}/participants/{userID}", s.authMiddleware(s.handleRemoveParticipant)).Methods("DELETE")

	// Messages
	api.HandleFunc("/conversations/{conversationID}/messages", s.authMiddleware(s.handleGetMessages)).Methods("GET")
	api.HandleFunc("/conversations/{conversationID}/messages", s.authMiddleware(s.handleSendMessage)).Methods("POST")
	api.HandleFunc("/conversations/{conversationID}/read", s.authMiddleware(s.handleMarkAsRead)).Methods("POST")
	api.HandleFunc("/conversations/{conversationID}/typing", s.authMiddleware(s.handleTypingIndicator)).Methods("POST")
}

func (s *ChatHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *ChatHTTPServer) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	orgID := s.getOrgIDFromContext(r.Context())

	var req biz.CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	conversation, err := s.chatUc.CreateConversation(r.Context(), &req, userID, orgID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusCreated, conversation)
}

func (s *ChatHTTPServer) handleGetUserConversations(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())

	conversations, err := s.chatUc.GetUserConversations(r.Context(), userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, conversations)
}

func (s *ChatHTTPServer) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	conversationID := s.getConversationIDFromPath(r)

	conversation, err := s.chatUc.GetConversation(r.Context(), conversationID, userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, conversation)
}

func (s *ChatHTTPServer) handleUpdateConversation(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	conversationID := s.getConversationIDFromPath(r)

	var req biz.UpdateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	conversation, err := s.chatUc.UpdateConversation(r.Context(), conversationID, userID, &req)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, conversation)
}

func (s *ChatHTTPServer) handleGetParticipants(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	conversationID := s.getConversationIDFromPath(r)

	participants, err := s.chatUc.GetConversationParticipants(r.Context(), conversationID, userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, participants)
}

func (s *ChatHTTPServer) handleAddParticipant(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	conversationID := s.getConversationIDFromPath(r)

	var req biz.AddParticipantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := s.chatUc.AddParticipant(r.Context(), conversationID, userID, &req); err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (s *ChatHTTPServer) handleRemoveParticipant(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	conversationID := s.getConversationIDFromPath(r)

	vars := mux.Vars(r)
	targetUserIDStr := vars["userID"]
	targetUserID, err := strconv.Atoi(targetUserIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if err := s.chatUc.RemoveParticipant(r.Context(), conversationID, userID, targetUserID); err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (s *ChatHTTPServer) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	conversationID := s.getConversationIDFromPath(r)

	// Parse pagination parameters
	limit := 50
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	messages, err := s.chatUc.GetConversationMessages(r.Context(), conversationID, userID, limit, offset)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, messages)
}

func (s *ChatHTTPServer) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	conversationID := s.getConversationIDFromPath(r)

	var req biz.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	req.ConversationID = conversationID

	message, err := s.chatUc.SendMessage(r.Context(), &req, userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusCreated, message)
}

func (s *ChatHTTPServer) handleMarkAsRead(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	conversationID := s.getConversationIDFromPath(r)

	if err := s.chatUc.MarkAsRead(r.Context(), conversationID, userID); err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "marked_as_read"})
}

func (s *ChatHTTPServer) handleTypingIndicator(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	conversationID := s.getConversationIDFromPath(r)

	var req struct {
		IsTyping bool `json:"is_typing"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := s.chatUc.SendTypingIndicator(r.Context(), conversationID, userID, req.IsTyping); err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// Helper methods
func (s *ChatHTTPServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

		claims, err := auth.ValidateToken(tokenString, s.jwtSecret)
		if err != nil {
			sharedserver.WriteError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		ctx := auth.SetClaims(r.Context(), claims)
		next(w, r.WithContext(ctx))
	}
}

func (s *ChatHTTPServer) getUserIDFromContext(ctx context.Context) int {
	id, err := auth.GetUserID(ctx)
	if err != nil {
		return 0
	}
	return id
}

func (s *ChatHTTPServer) getOrgIDFromContext(ctx context.Context) uuid.UUID {
	orgID, err := auth.GetOrgID(ctx)
	if err != nil {
		return uuid.Nil
	}
	return orgID
}

func (s *ChatHTTPServer) getConversationIDFromPath(r *http.Request) uuid.UUID {
	vars := mux.Vars(r)
	conversationIDStr := vars["conversationID"]
	conversationID, _ := uuid.Parse(conversationIDStr)
	return conversationID
}

func (s *ChatHTTPServer) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, biz.ErrConversationNotFound):
		sharedserver.WriteError(w, http.StatusNotFound, "Conversation not found")
	case errors.Is(err, biz.ErrNotParticipant):
		sharedserver.WriteError(w, http.StatusForbidden, "Not a participant in this conversation")
	case errors.Is(err, biz.ErrInsufficientPermissions):
		sharedserver.WriteError(w, http.StatusForbidden, "Insufficient permissions")
	case errors.Is(err, biz.ErrInvalidRequest):
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid request")
	case errors.Is(err, biz.ErrInvalidDMParticipants):
		sharedserver.WriteError(w, http.StatusBadRequest, "DM conversations must have exactly 2 participants")
	case errors.Is(err, biz.ErrMessageNotFound):
		sharedserver.WriteError(w, http.StatusNotFound, "Message not found")
	default:
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
