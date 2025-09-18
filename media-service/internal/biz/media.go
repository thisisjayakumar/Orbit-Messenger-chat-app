package biz

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type FileStatus string

const (
	FileStatusUploading FileStatus = "uploading"
	FileStatusScanning  FileStatus = "scanning"
	FileStatusReady     FileStatus = "ready"
	FileStatusQuarantine FileStatus = "quarantine"
	FileStatusError     FileStatus = "error"
)

type Attachment struct {
	ID        uuid.UUID              `json:"id"`
	MessageID *uuid.UUID             `json:"message_id,omitempty"`
	ObjectKey string                 `json:"object_key"`
	FileName  string                 `json:"file_name"`
	MimeType  string                 `json:"mime_type"`
	Size      int64                  `json:"size"`
	Status    FileStatus             `json:"status"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type UploadRequest struct {
	FileName    string `json:"file_name" validate:"required"`
	ContentType string `json:"content_type" validate:"required"`
	Size        int64  `json:"size" validate:"required"`
	MessageID   *uuid.UUID `json:"message_id,omitempty"`
}

type UploadResponse struct {
	AttachmentID uuid.UUID `json:"attachment_id"`
	UploadURL    string    `json:"upload_url"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type DownloadResponse struct {
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type MediaRepo interface {
	CreateAttachment(ctx context.Context, attachment *Attachment) error
	GetAttachment(ctx context.Context, id uuid.UUID) (*Attachment, error)
	UpdateAttachment(ctx context.Context, attachment *Attachment) error
	DeleteAttachment(ctx context.Context, id uuid.UUID) error
	GetAttachmentsByMessage(ctx context.Context, messageID uuid.UUID) ([]*Attachment, error)
}

type StorageProvider interface {
	GenerateUploadURL(ctx context.Context, objectKey string, contentType string, expiresIn time.Duration) (string, error)
	GenerateDownloadURL(ctx context.Context, objectKey string, expiresIn time.Duration) (string, error)
	UploadFile(ctx context.Context, objectKey string, reader io.Reader, contentType string) error
	DeleteFile(ctx context.Context, objectKey string) error
	GetFileInfo(ctx context.Context, objectKey string) (size int64, err error)
}

type AntivirusScanner interface {
	ScanFile(ctx context.Context, objectKey string) (bool, error) // returns true if clean
}

type MediaUsecase struct {
	repo            MediaRepo
	storage         StorageProvider
	antivirus       AntivirusScanner
	maxFileSize     int64
	allowedTypes    []string
	antivirusEnabled bool
}

func NewMediaUsecase(repo MediaRepo, storage StorageProvider, antivirus AntivirusScanner, maxFileSize int64, allowedTypes []string, antivirusEnabled bool) *MediaUsecase {
	return &MediaUsecase{
		repo:            repo,
		storage:         storage,
		antivirus:       antivirus,
		maxFileSize:     maxFileSize,
		allowedTypes:    allowedTypes,
		antivirusEnabled: antivirusEnabled,
	}
}

func (uc *MediaUsecase) InitiateUpload(ctx context.Context, req *UploadRequest, userID uuid.UUID) (*UploadResponse, error) {
	// Validate file size
	if req.Size > uc.maxFileSize {
		return nil, ErrFileTooLarge
	}

	// Validate content type
	if !uc.isAllowedContentType(req.ContentType) {
		return nil, ErrInvalidFileType
	}

	// Validate file extension matches content type
	if !uc.validateFileExtension(req.FileName, req.ContentType) {
		return nil, ErrInvalidFileType
	}

	// Generate unique object key
	objectKey := uc.generateObjectKey(userID, req.FileName)

	// Create attachment record
	attachment := &Attachment{
		ID:        uuid.New(),
		MessageID: nil, // Will be set when message is created
		ObjectKey: objectKey,
		FileName:  req.FileName,
		MimeType:  req.ContentType,
		Size:      req.Size,
		Status:    FileStatusUploading,
		Meta:      make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if req.MessageID != nil {
		attachment.MessageID = req.MessageID
	}

	if err := uc.repo.CreateAttachment(ctx, attachment); err != nil {
		return nil, err
	}

	// Generate upload URL (valid for 1 hour)
	uploadURL, err := uc.storage.GenerateUploadURL(ctx, objectKey, req.ContentType, time.Hour)
	if err != nil {
		return nil, err
	}

	return &UploadResponse{
		AttachmentID: attachment.ID,
		UploadURL:    uploadURL,
		ExpiresAt:    time.Now().Add(time.Hour),
	}, nil
}

func (uc *MediaUsecase) CompleteUpload(ctx context.Context, attachmentID uuid.UUID) error {
	attachment, err := uc.repo.GetAttachment(ctx, attachmentID)
	if err != nil {
		return err
	}

	if attachment.Status != FileStatusUploading {
		return ErrInvalidFileStatus
	}

	// Verify file was uploaded
	actualSize, err := uc.storage.GetFileInfo(ctx, attachment.ObjectKey)
	if err != nil {
		attachment.Status = FileStatusError
		attachment.UpdatedAt = time.Now()
		uc.repo.UpdateAttachment(ctx, attachment)
		return err
	}

	// Update size if different
	if actualSize != attachment.Size {
		attachment.Size = actualSize
	}

	// Start antivirus scan if enabled
	if uc.antivirusEnabled && uc.antivirus != nil {
		attachment.Status = FileStatusScanning
		attachment.UpdatedAt = time.Now()
		if err := uc.repo.UpdateAttachment(ctx, attachment); err != nil {
			return err
		}

		// Perform scan asynchronously
		go uc.performAntivirusScan(context.Background(), attachmentID)
	} else {
		// Mark as ready
		attachment.Status = FileStatusReady
		attachment.UpdatedAt = time.Now()
		if err := uc.repo.UpdateAttachment(ctx, attachment); err != nil {
			return err
		}
	}

	return nil
}

func (uc *MediaUsecase) performAntivirusScan(ctx context.Context, attachmentID uuid.UUID) {
	attachment, err := uc.repo.GetAttachment(ctx, attachmentID)
	if err != nil {
		return
	}

	isClean, err := uc.antivirus.ScanFile(ctx, attachment.ObjectKey)
	if err != nil {
		attachment.Status = FileStatusError
	} else if isClean {
		attachment.Status = FileStatusReady
	} else {
		attachment.Status = FileStatusQuarantine
	}

	attachment.UpdatedAt = time.Now()
	uc.repo.UpdateAttachment(ctx, attachment)
}

func (uc *MediaUsecase) GetDownloadURL(ctx context.Context, attachmentID uuid.UUID, userID uuid.UUID) (*DownloadResponse, error) {
	attachment, err := uc.repo.GetAttachment(ctx, attachmentID)
	if err != nil {
		return nil, err
	}

	if attachment.Status != FileStatusReady {
		return nil, ErrFileNotReady
	}

	// TODO: Add permission check - verify user has access to this attachment

	// Generate download URL (valid for 1 hour)
	downloadURL, err := uc.storage.GenerateDownloadURL(ctx, attachment.ObjectKey, time.Hour)
	if err != nil {
		return nil, err
	}

	return &DownloadResponse{
		DownloadURL: downloadURL,
		ExpiresAt:   time.Now().Add(time.Hour),
	}, nil
}

func (uc *MediaUsecase) GetAttachment(ctx context.Context, attachmentID uuid.UUID) (*Attachment, error) {
	return uc.repo.GetAttachment(ctx, attachmentID)
}

func (uc *MediaUsecase) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]*Attachment, error) {
	return uc.repo.GetAttachmentsByMessage(ctx, messageID)
}

func (uc *MediaUsecase) DeleteAttachment(ctx context.Context, attachmentID uuid.UUID, userID uuid.UUID) error {
	attachment, err := uc.repo.GetAttachment(ctx, attachmentID)
	if err != nil {
		return err
	}

	// TODO: Add permission check - verify user owns this attachment or has admin rights

	// Delete from storage
	if err := uc.storage.DeleteFile(ctx, attachment.ObjectKey); err != nil {
		// Log error but continue with database deletion
	}

	// Delete from database
	return uc.repo.DeleteAttachment(ctx, attachmentID)
}

func (uc *MediaUsecase) AssociateWithMessage(ctx context.Context, attachmentID, messageID uuid.UUID) error {
	attachment, err := uc.repo.GetAttachment(ctx, attachmentID)
	if err != nil {
		return err
	}

	attachment.MessageID = &messageID
	attachment.UpdatedAt = time.Now()

	return uc.repo.UpdateAttachment(ctx, attachment)
}

// Helper methods
func (uc *MediaUsecase) isAllowedContentType(contentType string) bool {
	for _, allowed := range uc.allowedTypes {
		if allowed == contentType {
			return true
		}
	}
	return false
}

func (uc *MediaUsecase) validateFileExtension(fileName, contentType string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	expectedType := mime.TypeByExtension(ext)
	
	// Basic validation - in production you might want more sophisticated checks
	return expectedType == contentType || 
		   (strings.HasPrefix(contentType, "image/") && strings.HasPrefix(expectedType, "image/")) ||
		   (strings.HasPrefix(contentType, "application/") && strings.HasPrefix(expectedType, "application/"))
}

func (uc *MediaUsecase) generateObjectKey(userID uuid.UUID, fileName string) string {
	timestamp := time.Now().Unix()
	fileID := uuid.New().String()
	ext := filepath.Ext(fileName)
	
	return fmt.Sprintf("attachments/%s/%d_%s%s", userID.String(), timestamp, fileID, ext)
}

// GenerateThumbnail generates a thumbnail for image files
func (uc *MediaUsecase) GenerateThumbnail(ctx context.Context, attachmentID uuid.UUID) error {
	attachment, err := uc.repo.GetAttachment(ctx, attachmentID)
	if err != nil {
		return err
	}

	// Only generate thumbnails for images
	if !strings.HasPrefix(attachment.MimeType, "image/") {
		return nil
	}

	// TODO: Implement thumbnail generation
	// This would involve:
	// 1. Download the original image
	// 2. Resize it to thumbnail size
	// 3. Upload thumbnail to storage
	// 4. Update attachment metadata with thumbnail info

	return nil
}
