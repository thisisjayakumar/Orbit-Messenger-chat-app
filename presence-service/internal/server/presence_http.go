package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/presence-service/internal/biz"
	sharedserver "github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/server"
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
	api.HandleFunc("/presence/connect", s.handleClientConnect).Methods("POST")
}

func (s *PresenceHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *PresenceHTTPServer) handleGetUserPresence(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["userID"]

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	presence, err := s.presenceUc.GetUserPresence(r.Context(), userID)
	if err != nil {
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, presence)
}

func (s *PresenceHTTPServer) handleSetUserStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["userID"]

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		Status       biz.PresenceStatus `json:"status"`
		CustomStatus string             `json:"custom_status,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
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
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid status")
		return
	}

	if err := s.presenceUc.SetUserStatus(r.Context(), userID, req.Status, req.CustomStatus); err != nil {
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Publish presence update via MQTT
	if s.mqttServer != nil {
		s.mqttServer.PublishPresenceUpdate(userID, req.Status, req.CustomStatus)
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *PresenceHTTPServer) handleGetMultipleUserPresence(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserIDs []string `json:"user_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if len(req.UserIDs) == 0 {
		sharedserver.WriteError(w, http.StatusBadRequest, "No user IDs provided")
		return
	}

	if len(req.UserIDs) > 100 {
		sharedserver.WriteError(w, http.StatusBadRequest, "Too many user IDs (max 100)")
		return
	}

	userIDs := make([]int, len(req.UserIDs))
	for i, userIDStr := range req.UserIDs {
		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			sharedserver.WriteError(w, http.StatusBadRequest, "Invalid user ID: "+userIDStr)
			return
		}
		userIDs[i] = userID
	}

	presenceMap, err := s.presenceUc.GetMultipleUserPresence(r.Context(), userIDs)
	if err != nil {
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert map keys to strings for JSON response
	response := make(map[string]*biz.UserPresence)
	for userID, presence := range presenceMap {
		response[strconv.Itoa(userID)] = presence
	}

	sharedserver.WriteJSON(w, http.StatusOK, response)
}

func (s *PresenceHTTPServer) handleGetUserSessions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["userID"]

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	sessions, err := s.presenceUc.GetUserDeviceSessions(r.Context(), userID)
	if err != nil {
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, sessions)
}

func (s *PresenceHTTPServer) handleClientConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID    string `json:"client_id"`
		UserID      int    `json:"user_id"`
		DeviceInfo  string `json:"device_info"`
		IPAddress   string `json:"ip_address"`
		ConnectedAt string `json:"connected_at"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.ClientID == "" || req.UserID == 0 {
		sharedserver.WriteError(w, http.StatusBadRequest, "client_id and user_id are required")
		return
	}

	// Get client IP if not provided
	if req.IPAddress == "" || req.IPAddress == "unknown" {
		req.IPAddress = r.RemoteAddr
	}

	// Handle client connection
	if err := s.presenceUc.HandleClientConnected(r.Context(), req.ClientID, req.UserID, req.DeviceInfo, req.IPAddress); err != nil {
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Publish presence update via MQTT
	if s.mqttServer != nil {
		s.mqttServer.PublishPresenceUpdate(req.UserID, biz.StatusOnline, "Available")
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}
