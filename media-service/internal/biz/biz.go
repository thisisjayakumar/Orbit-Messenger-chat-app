package biz

import (
	"errors"
	
	"github.com/google/wire"
)

var (
	ErrAttachmentNotFound = errors.New("attachment not found")
	ErrFileTooLarge       = errors.New("file too large")
	ErrInvalidFileType    = errors.New("invalid file type")
	ErrInvalidFileStatus  = errors.New("invalid file status")
	ErrFileNotReady       = errors.New("file not ready")
	ErrUnauthorized       = errors.New("unauthorized")
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewMediaUsecaseFromConfig)

// NewMediaUsecaseFromConfig creates media usecase with default config
func NewMediaUsecaseFromConfig(repo MediaRepo, storage StorageProvider, antivirus AntivirusScanner) *MediaUsecase {
	allowedTypes := []string{
		"image/jpeg", "image/png", "image/gif", "image/webp",
		"application/pdf", "application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"text/plain", "application/zip", "application/x-rar-compressed",
	}
	return NewMediaUsecase(repo, storage, antivirus, 100*1024*1024, allowedTypes, false) // 100MB max
}
