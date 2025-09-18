package server

import (
    "encoding/json"
    "net/http"

    "github.com/google/uuid"
    "github.com/gorilla/mux"

    "github.com/thisisjayakumar/Orbit-Messenger-chat-app/presence-service/internal/biz"
)

type PresenceHTTPServer struct {
	presenceUc *biz.PresenceUsecase
	mqttServer *MQTTServer
	router     *mux.Router
}

func NewPresenceHTTPServer(presenceUc *biz.PresenceUsecase, mqttServer *MQTTServer) *PresenceHTTPServer {
	s := &PresenceHTTPServer{
		presenceUc: presenceUc,
		mqttServer: mqttServer,
		router:     mux.NewRouter(),
	}
	s.setupRoutes()
	return s
}

func (s *PresenceHTTPServer) setupRoutes() {
	api := s.router.PathPrefix("/api/v1").Subrouter()

	api.HandleFunc("/presence/{userID}", s.handleGetUserPresence).Methods("GET")
	api.HandleFunc("/presence/{userID}/status", s.handleSetUserStatus).Methods("PUT")
	api.HandleFunc("/presence/bulk", s.handleGetMultipleUserPresence).Methods("POST")
	api.HandleFunc("/presence/{userID}/sessions", s.handleGetUserSessions).Methods("GET")
}

func (s *PresenceHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (s *PresenceHTTPServer) handleGetUserPresence(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["userID"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	presence, err := s.presenceUc.GetUserPresence(r.Context(), userID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, presence)
}

func (s *PresenceHTTPServer) handleSetUserStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["userID"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		Status       biz.PresenceStatus `json:"status"`
		CustomStatus string             `json:"custom_status,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate status
	validStatuses := []biz.PresenceStatus{
		biz.StatusOnline,
		biz.StatusAway,
		biz.StatusOffline,
		biz.StatusDoNotDisturb,
	}
	
	isValid := false
	for _, validStatus := range validStatuses {
		if req.Status == validStatus {
			isValid = true
			break
		}
	}
	
	if !isValid {
		s.writeError(w, http.StatusBadRequest, "Invalid status")
		return
	}

	if err := s.presenceUc.SetUserStatus(r.Context(), userID, req.Status, req.CustomStatus); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Publish presence update via MQTT
	if s.mqttServer != nil {
		s.mqttServer.PublishPresenceUpdate(userID, req.Status, req.CustomStatus)
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *PresenceHTTPServer) handleGetMultipleUserPresence(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserIDs []string `json:"user_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if len(req.UserIDs) == 0 {
		s.writeError(w, http.StatusBadRequest, "No user IDs provided")
		return
	}

	if len(req.UserIDs) > 100 {
		s.writeError(w, http.StatusBadRequest, "Too many user IDs (max 100)")
		return
	}

	userIDs := make([]uuid.UUID, len(req.UserIDs))
	for i, userIDStr := range req.UserIDs {
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "Invalid user ID: "+userIDStr)
			return
		}
		userIDs[i] = userID
	}

	presenceMap, err := s.presenceUc.GetMultipleUserPresence(r.Context(), userIDs)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert map keys to strings for JSON response
	response := make(map[string]*biz.UserPresence)
	for userID, presence := range presenceMap {
		response[userID.String()] = presence
	}

	s.writeJSON(w, http.StatusOK, response)
}

func (s *PresenceHTTPServer) handleGetUserSessions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["userID"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	sessions, err := s.presenceUc.GetUserDeviceSessions(r.Context(), userID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, sessions)
}

func (s *PresenceHTTPServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *PresenceHTTPServer) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
