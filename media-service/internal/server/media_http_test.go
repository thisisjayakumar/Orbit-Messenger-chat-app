package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/media-service/internal/biz"
	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/shared/auth"
)

// --------------- mocks ---------------

type mockMediaRepo struct {
	createAttachmentFunc        func(ctx context.Context, a *biz.Attachment) error
	getAttachmentFunc           func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error)
	updateAttachmentFunc        func(ctx context.Context, a *biz.Attachment) error
	deleteAttachmentFunc        func(ctx context.Context, id uuid.UUID) error
	getAttachmentsByMessageFunc func(ctx context.Context, mid uuid.UUID) ([]*biz.Attachment, error)
}

func (m *mockMediaRepo) CreateAttachment(ctx context.Context, a *biz.Attachment) error {
	if m.createAttachmentFunc != nil {
		return m.createAttachmentFunc(ctx, a)
	}
	return nil
}

func (m *mockMediaRepo) GetAttachment(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
	if m.getAttachmentFunc != nil {
		return m.getAttachmentFunc(ctx, id)
	}
	return nil, biz.ErrAttachmentNotFound
}

func (m *mockMediaRepo) UpdateAttachment(ctx context.Context, a *biz.Attachment) error {
	if m.updateAttachmentFunc != nil {
		return m.updateAttachmentFunc(ctx, a)
	}
	return nil
}

func (m *mockMediaRepo) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	if m.deleteAttachmentFunc != nil {
		return m.deleteAttachmentFunc(ctx, id)
	}
	return nil
}

func (m *mockMediaRepo) GetAttachmentsByMessage(ctx context.Context, mid uuid.UUID) ([]*biz.Attachment, error) {
	if m.getAttachmentsByMessageFunc != nil {
		return m.getAttachmentsByMessageFunc(ctx, mid)
	}
	return nil, nil
}

type mockStorage struct {
	generateUploadURLFunc   func(ctx context.Context, key, ct string, d time.Duration) (string, error)
	generateDownloadURLFunc func(ctx context.Context, key string, d time.Duration) (string, error)
	deleteFileFunc          func(ctx context.Context, key string) error
	getFileInfoFunc         func(ctx context.Context, key string) (int64, error)
}

func (m *mockStorage) GenerateUploadURL(ctx context.Context, key, ct string, d time.Duration) (string, error) {
	if m.generateUploadURLFunc != nil {
		return m.generateUploadURLFunc(ctx, key, ct, d)
	}
	return "https://upload.example.com/" + key, nil
}

func (m *mockStorage) GenerateDownloadURL(ctx context.Context, key string, d time.Duration) (string, error) {
	if m.generateDownloadURLFunc != nil {
		return m.generateDownloadURLFunc(ctx, key, d)
	}
	return "https://download.example.com/" + key, nil
}

func (m *mockStorage) UploadFile(ctx context.Context, key string, r io.Reader, ct string) error {
	return nil
}

func (m *mockStorage) DeleteFile(ctx context.Context, key string) error {
	if m.deleteFileFunc != nil {
		return m.deleteFileFunc(ctx, key)
	}
	return nil
}

func (m *mockStorage) GetFileInfo(ctx context.Context, key string) (int64, error) {
	if m.getFileInfoFunc != nil {
		return m.getFileInfoFunc(ctx, key)
	}
	return 1024, nil
}

type mockAntivirus struct {
	scanFileFunc func(ctx context.Context, key string) (bool, error)
}

func (m *mockAntivirus) ScanFile(ctx context.Context, key string) (bool, error) {
	if m.scanFileFunc != nil {
		return m.scanFileFunc(ctx, key)
	}
	return true, nil
}

// --------------- helpers ---------------

func setupMediaTestServer(repo biz.MediaRepo, storage biz.StorageProvider, av biz.AntivirusScanner) *MediaHTTPServer {
	uc := biz.NewMediaUsecase(repo, storage, av, 10*1024*1024, []string{"image/jpeg", "image/png", "application/pdf"}, false)
	return NewMediaHTTPServer(uc, "test-secret")
}

func setClaimsCtx(r *http.Request, userID int) *http.Request {
	claims := &auth.JWTClaims{
		UserID:         userID,
		OrganizationID: uuid.New().String(),
		Email:          "test@example.com",
		Role:           "member",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	ctx := auth.SetClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

func defaultAttachment() *biz.Attachment {
	return &biz.Attachment{
		ID:        uuid.New(),
		ObjectKey: "attachments/test/123_file.jpg",
		FileName:  "photo.jpg",
		MimeType:  "image/jpeg",
		Size:      1024,
		Status:    biz.FileStatusUploading,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// --------------- Auth Middleware Tests ---------------

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/initiate", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Authorization header required" {
		t.Errorf("expected 'Authorization header required', got '%s'", resp["error"])
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/initiate", nil)
	req.Header.Set("Authorization", "NotBearer token")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/initiate", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})

	// Generate a valid JWT
	claims := &auth.JWTClaims{
		UserID:         1,
		OrganizationID: uuid.New().String(),
		Email:          "test@example.com",
		Role:           "member",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("test-secret"))

	body := bytes.NewReader([]byte(`{"file_name":"test.jpg","content_type":"image/jpeg","size":1024}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/initiate", body)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	// Should pass auth and get 200 (not 401)
	if w.Code == http.StatusUnauthorized {
		t.Error("got 401 despite valid JWT")
	}
}

// --------------- InitiateUpload Tests ---------------

func TestHandleInitiateUpload_Success(t *testing.T) {
	repo := &mockMediaRepo{
		createAttachmentFunc: func(ctx context.Context, a *biz.Attachment) error { return nil },
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	body := bytes.NewReader([]byte(`{"file_name":"photo.jpg","content_type":"image/jpeg","size":1024}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/initiate", body)
	req = setClaimsCtx(req, 1)
	w := httptest.NewRecorder()

	s.handleInitiateUpload(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp biz.UploadResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.AttachmentID == uuid.Nil {
		t.Error("expected non-nil attachment ID")
	}
	if resp.UploadURL == "" {
		t.Error("expected non-empty upload URL")
	}
}

func TestHandleInitiateUpload_FileTooLarge(t *testing.T) {
	// maxFileSize is 10MB; send 11MB
	body := bytes.NewReader([]byte(`{"file_name":"big.jpg","content_type":"image/jpeg","size":11534336}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/initiate", body)
	req = setClaimsCtx(req, 1)
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})
	w := httptest.NewRecorder()

	s.handleInitiateUpload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "File too large" {
		t.Errorf("expected 'File too large', got '%s'", resp["error"])
	}
}

func TestHandleInitiateUpload_InvalidType(t *testing.T) {
	body := bytes.NewReader([]byte(`{"file_name":"bad.exe","content_type":"application/x-msdownload","size":1024}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/initiate", body)
	req = setClaimsCtx(req, 1)
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})
	w := httptest.NewRecorder()

	s.handleInitiateUpload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleInitiateUpload_InvalidJSON(t *testing.T) {
	body := bytes.NewReader([]byte(`not-json`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/initiate", body)
	req = setClaimsCtx(req, 1)
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})
	w := httptest.NewRecorder()

	s.handleInitiateUpload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Invalid JSON" {
		t.Errorf("expected 'Invalid JSON', got '%s'", resp["error"])
	}
}

// --------------- CompleteUpload Tests ---------------

func TestHandleCompleteUpload_Success(t *testing.T) {
	att := defaultAttachment()
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return att, nil
		},
		updateAttachmentFunc: func(ctx context.Context, a *biz.Attachment) error { return nil },
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/"+att.ID.String()+"/complete", nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": att.ID.String()})
	w := httptest.NewRecorder()

	s.handleCompleteUpload(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "completed" {
		t.Errorf("expected 'completed', got '%s'", resp["status"])
	}
}

func TestHandleCompleteUpload_InvalidID(t *testing.T) {
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/not-a-uuid/complete", nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": "not-a-uuid"})
	w := httptest.NewRecorder()

	s.handleCompleteUpload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleCompleteUpload_NotFound(t *testing.T) {
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return nil, biz.ErrAttachmentNotFound
		},
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	aid := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload/"+aid.String()+"/complete", nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": aid.String()})
	w := httptest.NewRecorder()

	s.handleCompleteUpload(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --------------- GetAttachment Tests ---------------

func TestHandleGetAttachment_Success(t *testing.T) {
	att := defaultAttachment()
	att.Status = biz.FileStatusReady
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return att, nil
		},
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/attachments/"+att.ID.String(), nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": att.ID.String()})
	w := httptest.NewRecorder()

	s.handleGetAttachment(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp biz.Attachment
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ID != att.ID {
		t.Errorf("expected attachment ID %s, got %s", att.ID, resp.ID)
	}
}

func TestHandleGetAttachment_NotFound(t *testing.T) {
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return nil, biz.ErrAttachmentNotFound
		},
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	aid := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/attachments/"+aid.String(), nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": aid.String()})
	w := httptest.NewRecorder()

	s.handleGetAttachment(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --------------- GetDownloadURL Tests ---------------

func TestHandleGetDownloadURL_Success(t *testing.T) {
	att := defaultAttachment()
	att.Status = biz.FileStatusReady
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return att, nil
		},
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/attachments/"+att.ID.String()+"/download", nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": att.ID.String()})
	w := httptest.NewRecorder()

	s.handleGetDownloadURL(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp biz.DownloadResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.DownloadURL == "" {
		t.Error("expected non-empty download URL")
	}
}

func TestHandleGetDownloadURL_FileNotReady(t *testing.T) {
	att := defaultAttachment()
	att.Status = biz.FileStatusUploading // not ready
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return att, nil
		},
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/attachments/"+att.ID.String()+"/download", nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": att.ID.String()})
	w := httptest.NewRecorder()

	s.handleGetDownloadURL(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

// --------------- DeleteAttachment Tests ---------------

func TestHandleDeleteAttachment_Success(t *testing.T) {
	att := defaultAttachment()
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return att, nil
		},
		deleteAttachmentFunc: func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/attachments/"+att.ID.String(), nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": att.ID.String()})
	w := httptest.NewRecorder()

	s.handleDeleteAttachment(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleDeleteAttachment_NotFound(t *testing.T) {
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return nil, biz.ErrAttachmentNotFound
		},
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	aid := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/attachments/"+aid.String(), nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": aid.String()})
	w := httptest.NewRecorder()

	s.handleDeleteAttachment(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --------------- AssociateWithMessage Tests ---------------

func TestHandleAssociateWithMessage_Success(t *testing.T) {
	att := defaultAttachment()
	msgID := uuid.New()
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return att, nil
		},
		updateAttachmentFunc: func(ctx context.Context, a *biz.Attachment) error { return nil },
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	body := bytes.NewReader([]byte(`{"message_id":"` + msgID.String() + `"}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/"+att.ID.String()+"/associate", body)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": att.ID.String()})
	w := httptest.NewRecorder()

	s.handleAssociateWithMessage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleAssociateWithMessage_InvalidJSON(t *testing.T) {
	att := defaultAttachment()
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})

	body := bytes.NewReader([]byte(`not-json`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/"+att.ID.String()+"/associate", body)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": att.ID.String()})
	w := httptest.NewRecorder()

	s.handleAssociateWithMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --------------- GetMessageAttachments Tests ---------------

func TestHandleGetMessageAttachments_Success(t *testing.T) {
	msgID := uuid.New()
	atts := []*biz.Attachment{defaultAttachment()}
	repo := &mockMediaRepo{
		getAttachmentsByMessageFunc: func(ctx context.Context, mid uuid.UUID) ([]*biz.Attachment, error) {
			return atts, nil
		},
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/"+msgID.String()+"/attachments", nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"messageID": msgID.String()})
	w := httptest.NewRecorder()

	s.handleGetMessageAttachments(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp []*biz.Attachment
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 attachment, got %d", len(resp))
	}
}

func TestHandleGetMessageAttachments_InvalidMessageID(t *testing.T) {
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/bad-id/attachments", nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"messageID": "bad-id"})
	w := httptest.NewRecorder()

	s.handleGetMessageAttachments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --------------- GenerateThumbnail Tests ---------------

func TestHandleGenerateThumbnail_Success(t *testing.T) {
	att := defaultAttachment()
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return att, nil
		},
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/"+att.ID.String()+"/thumbnail", nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": att.ID.String()})
	w := httptest.NewRecorder()

	s.handleGenerateThumbnail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleGenerateThumbnail_InvalidID(t *testing.T) {
	s := setupMediaTestServer(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/bad-id/thumbnail", nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": "bad-id"})
	w := httptest.NewRecorder()

	s.handleGenerateThumbnail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleGenerateThumbnail_NotFound(t *testing.T) {
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*biz.Attachment, error) {
			return nil, biz.ErrAttachmentNotFound
		},
	}
	s := setupMediaTestServer(repo, &mockStorage{}, &mockAntivirus{})

	aid := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/"+aid.String()+"/thumbnail", nil)
	req = setClaimsCtx(req, 1)
	req = mux.SetURLVars(req, map[string]string{"attachmentID": aid.String()})
	w := httptest.NewRecorder()

	s.handleGenerateThumbnail(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
