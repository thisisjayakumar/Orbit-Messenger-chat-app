package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/media-service/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/auth"
	sharedserver "github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/server"
)

type MediaHTTPServer struct {
	mediaUc   *biz.MediaUsecase
	router    *mux.Router
	jwtSecret string
}

func NewMediaHTTPServer(mediaUc *biz.MediaUsecase, jwtSecret string) *MediaHTTPServer {
	s := &MediaHTTPServer{
		mediaUc:   mediaUc,
		router:    mux.NewRouter(),
		jwtSecret: jwtSecret,
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
	s.router.ServeHTTP(w, r)
}

func (s *MediaHTTPServer) handleInitiateUpload(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())

	var req biz.UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	response, err := s.mediaUc.InitiateUpload(r.Context(), &req, userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, response)
}

func (s *MediaHTTPServer) handleCompleteUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	if err := s.mediaUc.CompleteUpload(r.Context(), attachmentID); err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (s *MediaHTTPServer) handleGetAttachment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	attachment, err := s.mediaUc.GetAttachment(r.Context(), attachmentID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, attachment)
}

func (s *MediaHTTPServer) handleGetDownloadURL(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	response, err := s.mediaUc.GetDownloadURL(r.Context(), attachmentID, userID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, response)
}

func (s *MediaHTTPServer) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserIDFromContext(r.Context())
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	if err := s.mediaUc.DeleteAttachment(r.Context(), attachmentID, userID); err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *MediaHTTPServer) handleAssociateWithMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	var req struct {
		MessageID uuid.UUID `json:"message_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := s.mediaUc.AssociateWithMessage(r.Context(), attachmentID, req.MessageID); err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "associated"})
}

func (s *MediaHTTPServer) handleGetMessageAttachments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	messageIDStr := vars["messageID"]

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid message ID")
		return
	}

	attachments, err := s.mediaUc.GetMessageAttachments(r.Context(), messageID)
	if err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, attachments)
}

func (s *MediaHTTPServer) handleGenerateThumbnail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	attachmentIDStr := vars["attachmentID"]

	attachmentID, err := uuid.Parse(attachmentIDStr)
	if err != nil {
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	if err := s.mediaUc.GenerateThumbnail(r.Context(), attachmentID); err != nil {
		s.handleError(w, err)
		return
	}

	sharedserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "thumbnail_generated"})
}

// Helper methods
func (s *MediaHTTPServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

func (s *MediaHTTPServer) getUserIDFromContext(ctx context.Context) uuid.UUID {
	id, err := auth.GetUserID(ctx)
	if err != nil {
		return uuid.Nil
	}
	// Convert int user ID to deterministic UUID for media service compatibility
	// (the media service stores user IDs as UUIDs, consistent with the DB schema)
	namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	return uuid.NewSHA1(namespace, []byte(fmt.Sprintf("user_%d", id)))
}

func (s *MediaHTTPServer) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, biz.ErrAttachmentNotFound):
		sharedserver.WriteError(w, http.StatusNotFound, "Attachment not found")
	case errors.Is(err, biz.ErrFileTooLarge):
		sharedserver.WriteError(w, http.StatusBadRequest, "File too large")
	case errors.Is(err, biz.ErrInvalidFileType):
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid file type")
	case errors.Is(err, biz.ErrInvalidFileStatus):
		sharedserver.WriteError(w, http.StatusBadRequest, "Invalid file status")
	case errors.Is(err, biz.ErrFileNotReady):
		sharedserver.WriteError(w, http.StatusConflict, "File not ready for download")
	case errors.Is(err, biz.ErrUnauthorized):
		sharedserver.WriteError(w, http.StatusForbidden, "Unauthorized")
	default:
		sharedserver.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
