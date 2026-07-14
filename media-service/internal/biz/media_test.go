package biz

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Mocks
// ──────────────────────────────────────────────

type mockMediaRepo struct {
	createAttachmentFunc        func(ctx context.Context, a *Attachment) error
	getAttachmentFunc           func(ctx context.Context, id uuid.UUID) (*Attachment, error)
	updateAttachmentFunc        func(ctx context.Context, a *Attachment) error
	deleteAttachmentFunc        func(ctx context.Context, id uuid.UUID) error
	getAttachmentsByMessageFunc func(ctx context.Context, mid uuid.UUID) ([]*Attachment, error)
}

func (m *mockMediaRepo) CreateAttachment(ctx context.Context, a *Attachment) error {
	if m.createAttachmentFunc != nil {
		return m.createAttachmentFunc(ctx, a)
	}
	return nil
}
func (m *mockMediaRepo) GetAttachment(ctx context.Context, id uuid.UUID) (*Attachment, error) {
	if m.getAttachmentFunc != nil {
		return m.getAttachmentFunc(ctx, id)
	}
	return nil, ErrAttachmentNotFound
}
func (m *mockMediaRepo) UpdateAttachment(ctx context.Context, a *Attachment) error {
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
func (m *mockMediaRepo) GetAttachmentsByMessage(ctx context.Context, mid uuid.UUID) ([]*Attachment, error) {
	if m.getAttachmentsByMessageFunc != nil {
		return m.getAttachmentsByMessageFunc(ctx, mid)
	}
	return nil, nil
}

type mockStorage struct {
	generateUploadURLFunc   func(ctx context.Context, key, ct string, exp time.Duration) (string, error)
	generateDownloadURLFunc func(ctx context.Context, key string, exp time.Duration) (string, error)
	uploadFileFunc          func(ctx context.Context, key string, r io.Reader, ct string) error
	deleteFileFunc          func(ctx context.Context, key string) error
	getFileInfoFunc         func(ctx context.Context, key string) (int64, error)
}

func (m *mockStorage) GenerateUploadURL(ctx context.Context, key, ct string, exp time.Duration) (string, error) {
	if m.generateUploadURLFunc != nil {
		return m.generateUploadURLFunc(ctx, key, ct, exp)
	}
	return "https://upload.example.com/" + key, nil
}
func (m *mockStorage) GenerateDownloadURL(ctx context.Context, key string, exp time.Duration) (string, error) {
	if m.generateDownloadURLFunc != nil {
		return m.generateDownloadURLFunc(ctx, key, exp)
	}
	return "https://download.example.com/" + key, nil
}
func (m *mockStorage) UploadFile(ctx context.Context, key string, r io.Reader, ct string) error {
	if m.uploadFileFunc != nil {
		return m.uploadFileFunc(ctx, key, r, ct)
	}
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

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func defaultAllowedTypes() []string {
	return []string{
		"image/jpeg", "image/png", "image/gif", "image/webp",
		"application/pdf", "text/plain",
	}
}

func newUsecase(repo MediaRepo, storage StorageProvider, antivirus AntivirusScanner) *MediaUsecase {
	return NewMediaUsecase(repo, storage, antivirus, 10*1024*1024, defaultAllowedTypes(), false)
}

func newUsecaseWithAV(repo MediaRepo, storage StorageProvider, antivirus AntivirusScanner) *MediaUsecase {
	return NewMediaUsecase(repo, storage, antivirus, 10*1024*1024, defaultAllowedTypes(), true)
}

func validUploadReq() *UploadRequest {
	return &UploadRequest{
		FileName:    "photo.jpg",
		ContentType: "image/jpeg",
		Size:        1024,
	}
}

// ──────────────────────────────────────────────
// Tests: InitiateUpload
// ──────────────────────────────────────────────

func TestInitiateUpload_Success_CreatesAttachment(t *testing.T) {
	var created *Attachment
	repo := &mockMediaRepo{
		createAttachmentFunc: func(ctx context.Context, a *Attachment) error {
			created = a
			return nil
		},
	}
	uc := newUsecase(repo, &mockStorage{}, &mockAntivirus{})

	resp, err := uc.InitiateUpload(context.Background(), validUploadReq(), uuid.New())
	if err != nil {
		t.Fatalf("InitiateUpload() error = %v", err)
	}
	if resp == nil {
		t.Fatal("InitiateUpload() returned nil response")
	}
	if resp.AttachmentID == uuid.Nil {
		t.Error("AttachmentID should not be nil")
	}
	if resp.UploadURL == "" {
		t.Error("UploadURL should not be empty")
	}
	if resp.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
	if created == nil {
		t.Fatal("attachment was not created")
	}
	if created.Status != FileStatusUploading {
		t.Errorf("attachment.Status = %q, want %q", created.Status, FileStatusUploading)
	}
	if created.ObjectKey == "" {
		t.Error("ObjectKey should not be empty")
	}
}

func TestInitiateUpload_FileTooLarge_ReturnsError(t *testing.T) {
	uc := newUsecase(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})

	req := validUploadReq()
	req.Size = 20 * 1024 * 1024 // 20MB > 10MB max

	_, err := uc.InitiateUpload(context.Background(), req, uuid.New())
	if err != ErrFileTooLarge {
		t.Errorf("error = %v, want %v", err, ErrFileTooLarge)
	}
}

func TestInitiateUpload_InvalidContentType_ReturnsError(t *testing.T) {
	uc := newUsecase(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})

	req := validUploadReq()
	req.ContentType = "application/x-shockwave-flash"

	_, err := uc.InitiateUpload(context.Background(), req, uuid.New())
	if err != ErrInvalidFileType {
		t.Errorf("error = %v, want %v", err, ErrInvalidFileType)
	}
}

func TestInitiateUpload_WithMessageID_SetsMessageID(t *testing.T) {
	msgID := uuid.New()
	var captured *Attachment
	repo := &mockMediaRepo{
		createAttachmentFunc: func(ctx context.Context, a *Attachment) error {
			captured = a
			return nil
		},
	}
	uc := newUsecase(repo, &mockStorage{}, &mockAntivirus{})

	req := validUploadReq()
	req.MessageID = &msgID

	_, err := uc.InitiateUpload(context.Background(), req, uuid.New())
	if err != nil {
		t.Fatalf("InitiateUpload() error = %v", err)
	}
	if captured == nil || captured.MessageID == nil {
		t.Fatal("MessageID should be set")
	}
	if *captured.MessageID != msgID {
		t.Errorf("MessageID = %v, want %v", *captured.MessageID, msgID)
	}
}

// ──────────────────────────────────────────────
// Tests: CompleteUpload
// ──────────────────────────────────────────────

func TestCompleteUpload_ReadyDirectly_WhenAVDisabled(t *testing.T) {
	attachmentID := uuid.New()
	var updated *Attachment
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*Attachment, error) {
			return &Attachment{
				ID:        attachmentID,
				ObjectKey: "obj-key",
				Status:    FileStatusUploading,
				Size:      1024,
				Meta:      make(map[string]interface{}),
			}, nil
		},
		updateAttachmentFunc: func(ctx context.Context, a *Attachment) error {
			updated = a
			return nil
		},
	}
	// antivirusEnabled = false in default newUsecase
	uc := newUsecase(repo, &mockStorage{}, &mockAntivirus{})

	err := uc.CompleteUpload(context.Background(), attachmentID)
	if err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}
	if updated == nil {
		t.Fatal("attachment was not updated")
	}
	if updated.Status != FileStatusReady {
		t.Errorf("status = %q, want %q", updated.Status, FileStatusReady)
	}
}

func TestCompleteUpload_AttachmentNotFound_ReturnsError(t *testing.T) {
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*Attachment, error) {
			return nil, ErrAttachmentNotFound
		},
	}
	uc := newUsecase(repo, &mockStorage{}, &mockAntivirus{})

	err := uc.CompleteUpload(context.Background(), uuid.New())
	if err != ErrAttachmentNotFound {
		t.Errorf("error = %v, want %v", err, ErrAttachmentNotFound)
	}
}

func TestCompleteUpload_WrongStatus_ReturnsError(t *testing.T) {
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*Attachment, error) {
			return &Attachment{ID: id, Status: FileStatusReady, Meta: make(map[string]interface{})}, nil
		},
	}
	uc := newUsecase(repo, &mockStorage{}, &mockAntivirus{})

	err := uc.CompleteUpload(context.Background(), uuid.New())
	if err != ErrInvalidFileStatus {
		t.Errorf("error = %v, want %v", err, ErrInvalidFileStatus)
	}
}

func TestCompleteUpload_StorageFails_MarksError(t *testing.T) {
	attachmentID := uuid.New()
	var updated *Attachment
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*Attachment, error) {
			return &Attachment{
				ID:        attachmentID,
				ObjectKey: "obj-key",
				Status:    FileStatusUploading,
				Size:      1024,
				Meta:      make(map[string]interface{}),
			}, nil
		},
		updateAttachmentFunc: func(ctx context.Context, a *Attachment) error {
			updated = a
			return nil
		},
	}
	storage := &mockStorage{
		getFileInfoFunc: func(ctx context.Context, key string) (int64, error) {
			return 0, errors.New("file not found in storage")
		},
	}
	uc := newUsecase(repo, storage, &mockAntivirus{})

	err := uc.CompleteUpload(context.Background(), attachmentID)
	if err == nil {
		t.Fatal("CompleteUpload() expected error, got nil")
	}
	if updated == nil {
		t.Fatal("attachment should have been updated on failure")
	}
	if updated.Status != FileStatusError {
		t.Errorf("status = %q, want %q (should be marked error when storage check fails)", updated.Status, FileStatusError)
	}
}

// ──────────────────────────────────────────────
// Tests: GetDownloadURL
// ──────────────────────────────────────────────

func TestGetDownloadURL_ReturnsURL(t *testing.T) {
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*Attachment, error) {
			return &Attachment{ID: id, Status: FileStatusReady, Meta: make(map[string]interface{})}, nil
		},
	}
	uc := newUsecase(repo, &mockStorage{}, &mockAntivirus{})

	resp, err := uc.GetDownloadURL(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("GetDownloadURL() error = %v", err)
	}
	if resp.DownloadURL == "" {
		t.Error("DownloadURL should not be empty")
	}
	if resp.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
}

func TestGetDownloadURL_NotReady_ReturnsError(t *testing.T) {
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*Attachment, error) {
			return &Attachment{ID: id, Status: FileStatusUploading, Meta: make(map[string]interface{})}, nil
		},
	}
	uc := newUsecase(repo, &mockStorage{}, &mockAntivirus{})

	_, err := uc.GetDownloadURL(context.Background(), uuid.New(), uuid.New())
	if err != ErrFileNotReady {
		t.Errorf("error = %v, want %v", err, ErrFileNotReady)
	}
}

// ──────────────────────────────────────────────
// Tests: DeleteAttachment
// ──────────────────────────────────────────────

func TestDeleteAttachment_DeletesFromStorageAndDB(t *testing.T) {
	var storageDeleted bool
	var dbDeleted bool
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*Attachment, error) {
			return &Attachment{ID: id, ObjectKey: "obj-key", Meta: make(map[string]interface{})}, nil
		},
		deleteAttachmentFunc: func(ctx context.Context, id uuid.UUID) error {
			dbDeleted = true
			return nil
		},
	}
	storage := &mockStorage{
		deleteFileFunc: func(ctx context.Context, key string) error {
			storageDeleted = true
			return nil
		},
	}
	uc := newUsecase(repo, storage, &mockAntivirus{})

	err := uc.DeleteAttachment(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("DeleteAttachment() error = %v", err)
	}
	if !storageDeleted {
		t.Error("storage.DeleteFile was not called")
	}
	if !dbDeleted {
		t.Error("repo.DeleteAttachment was not called")
	}
}

func TestDeleteAttachment_StorageFails_StillDeletesFromDB(t *testing.T) {
	var dbDeleted bool
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*Attachment, error) {
			return &Attachment{ID: id, ObjectKey: "obj-key", Meta: make(map[string]interface{})}, nil
		},
		deleteAttachmentFunc: func(ctx context.Context, id uuid.UUID) error {
			dbDeleted = true
			return nil
		},
	}
	storage := &mockStorage{
		deleteFileFunc: func(ctx context.Context, key string) error {
			return errors.New("storage unavailable")
		},
	}
	uc := newUsecase(repo, storage, &mockAntivirus{})

	err := uc.DeleteAttachment(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("DeleteAttachment() error = %v (should not fail when storage delete fails)", err)
	}
	if !dbDeleted {
		t.Error("should still delete from DB even if storage delete fails")
	}
}

// ──────────────────────────────────────────────
// Tests: AssociateWithMessage
// ──────────────────────────────────────────────

func TestAssociateWithMessage_SetsMessageID(t *testing.T) {
	attachmentID := uuid.New()
	messageID := uuid.New()
	var updated *Attachment
	repo := &mockMediaRepo{
		getAttachmentFunc: func(ctx context.Context, id uuid.UUID) (*Attachment, error) {
			return &Attachment{ID: id, Meta: make(map[string]interface{})}, nil
		},
		updateAttachmentFunc: func(ctx context.Context, a *Attachment) error {
			updated = a
			return nil
		},
	}
	uc := newUsecase(repo, &mockStorage{}, &mockAntivirus{})

	err := uc.AssociateWithMessage(context.Background(), attachmentID, messageID)
	if err != nil {
		t.Fatalf("AssociateWithMessage() error = %v", err)
	}
	if updated == nil || updated.MessageID == nil {
		t.Fatal("MessageID should be set")
	}
	if *updated.MessageID != messageID {
		t.Errorf("MessageID = %v, want %v", *updated.MessageID, messageID)
	}
}

// ──────────────────────────────────────────────
// Tests: GetMessageAttachments
// ──────────────────────────────────────────────

func TestGetMessageAttachments_ReturnsList(t *testing.T) {
	messageID := uuid.New()
	repo := &mockMediaRepo{
		getAttachmentsByMessageFunc: func(ctx context.Context, mid uuid.UUID) ([]*Attachment, error) {
			return []*Attachment{
				{ID: uuid.New(), FileName: "a.jpg"},
				{ID: uuid.New(), FileName: "b.jpg"},
			}, nil
		},
	}
	uc := newUsecase(repo, &mockStorage{}, &mockAntivirus{})

	atts, err := uc.GetMessageAttachments(context.Background(), messageID)
	if err != nil {
		t.Fatalf("GetMessageAttachments() error = %v", err)
	}
	if len(atts) != 2 {
		t.Errorf("len(atts) = %d, want 2", len(atts))
	}
}

// ──────────────────────────────────────────────
// Tests: isAllowedContentType
// ──────────────────────────────────────────────

func TestIsAllowedContentType_JPEG_ReturnsTrue(t *testing.T) {
	uc := newUsecase(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})
	if !uc.isAllowedContentType("image/jpeg") {
		t.Error("image/jpeg should be allowed")
	}
}

func TestIsAllowedContentType_Unknown_ReturnsFalse(t *testing.T) {
	uc := newUsecase(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})
	if uc.isAllowedContentType("application/x-binary") {
		t.Error("unknown type should not be allowed")
	}
}

// ──────────────────────────────────────────────
// Tests: NewMediaUsecaseFromConfig
// ──────────────────────────────────────────────

func TestNewMediaUsecaseFromConfig_ReturnsValid(t *testing.T) {
	uc := NewMediaUsecaseFromConfig(&mockMediaRepo{}, &mockStorage{}, &mockAntivirus{})
	if uc == nil {
		t.Fatal("NewMediaUsecaseFromConfig() returned nil")
	}
	if uc.maxFileSize != 100*1024*1024 {
		t.Errorf("maxFileSize = %d, want %d", uc.maxFileSize, 100*1024*1024)
	}
}
