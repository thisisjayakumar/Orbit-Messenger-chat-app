package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/media-service/internal/biz"
)

type MediaHTTPServer struct {
	mediaUc *biz.MediaUsecase
	router  *mux.Router
}

func NewMediaHTTPServer(mediaUc *biz.MediaUsecase) *MediaHTTPServer {
	s := &MediaHTTPServer{
		mediaUc: mediaUc,
		router:  mux.NewRouter(),
	}
	s.setupRoutes()
	return s
}

func (s *MediaHTTPServer) setupRoutes() {
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Upload endpoints
	api.HandleFunc("/upload/initiate", s.authMiddleware(s.handleInitiateUpload)).Methods("POST")
	api.HandleFunc("/upload/{attachmentID}/complete", s.authMiddleware(s.handleCompleteUpload)).Methods("POST")

	// Attachment endpoints
	api.HandleFunc("/attachments/{attachmentID}", s.authMiddleware(s.handleGetAttachment)).Methods("GET")
	api.HandleFunc("/attachments/{attachmentID}/download", s.authMiddleware(s.handleGetDownloadURL)).Methods("GET")
	api.HandleFunc("/attachments/{attachmentID}", s.authMiddleware(s.handleDeleteAttachment)).Methods("DELETE")
	api.HandleFunc("/attachments/{attachmentID}/associate", s.authMiddleware(s.handleAssociateWithMessage)).Methods("POST")

	// Message attachments
	api.HandleFunc("/messages/{messageID}/attachments", s.authMiddleware(s.handleGetMessageAttachments)).Methods("GET")

	// Thumbnail generation
	api.HandleFunc("/attachments/{attachmentID}/thumbnail", s.authMiddleware(s.handleGenerateThumbnail)).Methods("POST")
}

func (s *MediaHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (s *MediaHTTPServer) handleInitiateUpload(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())

	var req biz.UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	response, err := s.mediaUc.InitiateUpload(r.Context(), &req, userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, response)
}

func (s *MediaHTTPServer) handleCompleteUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	if err := s.mediaUc.CompleteUpload(r.Context(), attachmentID); err != nil {
		s.handleError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (s *MediaHTTPServer) handleGetAttachment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	attachment, err := s.mediaUc.GetAttachment(r.Context(), attachmentID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, attachment)
}

func (s *MediaHTTPServer) handleGetDownloadURL(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	response, err := s.mediaUc.GetDownloadURL(r.Context(), attachmentID, userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, response)
}

func (s *MediaHTTPServer) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	if err := s.mediaUc.DeleteAttachment(r.Context(), attachmentID, userID); err != nil {
		s.handleError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *MediaHTTPServer) handleAssociateWithMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	var req struct {
		MessageID uuid.UUID `json:"message_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := s.mediaUc.AssociateWithMessage(r.Context(), attachmentID, req.MessageID); err != nil {
		s.handleError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "associated"})
}

func (s *MediaHTTPServer) handleGetMessageAttachments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	messageIDStr := vars["messageID"]

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid message ID")
		return
	}

	attachments, err := s.mediaUc.GetMessageAttachments(r.Context(), messageID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, attachments)
}

func (s *MediaHTTPServer) handleGenerateThumbnail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	if err := s.mediaUc.GenerateThumbnail(r.Context(), attachmentID); err != nil {
		s.handleError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "thumbnail_generated"})
}

// Helper methods
func (s *MediaHTTPServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// This is a simplified auth middleware
		// In production, you would validate JWT tokens here
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.writeError(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		// Extract token and validate (simplified)
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			s.writeError(w, http.StatusUnauthorized, "Invalid authorization format")
			return
		}

		// TODO: Validate token with auth service
		// For now, we'll extract user info from headers (for testing)
		userIDStr := r.Header.Get("X-User-ID")

		if userIDStr == "" {
			s.writeError(w, http.StatusUnauthorized, "Missing user ID")
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "Invalid user ID")
			return
		}

		// Add to context
		ctx := context.WithValue(r.Context(), "userID", userID)

		next(w, r.WithContext(ctx))
	}
}

func (s *MediaHTTPServer) getUserIDFromContext(ctx context.Context) uuid.UUID {
	return ctx.Value("userID").(uuid.UUID)
}

func (s *MediaHTTPServer) handleError(w http.ResponseWriter, err error) {
	switch err {
	case biz.ErrAttachmentNotFound:
		s.writeError(w, http.StatusNotFound, "Attachment not found")
	case biz.ErrFileTooLarge:
		s.writeError(w, http.StatusBadRequest, "File too large")
	case biz.ErrInvalidFileType:
		s.writeError(w, http.StatusBadRequest, "Invalid file type")
	case biz.ErrInvalidFileStatus:
		s.writeError(w, http.StatusBadRequest, "Invalid file status")
	case biz.ErrFileNotReady:
		s.writeError(w, http.StatusConflict, "File not ready for download")
	case biz.ErrUnauthorized:
		s.writeError(w, http.StatusForbidden, "Unauthorized")
	default:
		s.writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (s *MediaHTTPServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *MediaHTTPServer) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
