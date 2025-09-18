package data

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/media-service/internal/biz"
)

type minioStorage struct {
	client *minio.Client
	bucket string
}

type MinIOConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
	UseSSL    bool   `yaml:"use_ssl"`
}

func NewMinIOStorage(config MinIOConfig) (biz.StorageProvider, error) {
	// Initialize minio client
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	storage := &minioStorage{
		client: client,
		bucket: config.Bucket,
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, config.Bucket)
	if err != nil {
		return nil, err
	}

	if !exists {
		err = client.MakeBucket(ctx, config.Bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, err
		}
	}

	return storage, nil
}

func (s *minioStorage) GenerateUploadURL(ctx context.Context, objectKey string, contentType string, expiresIn time.Duration) (string, error) {
	// Set request parameters for content-type
	reqParams := make(url.Values)
	reqParams.Set("response-content-type", contentType)

	// Generate presigned URL for PUT operation
	presignedURL, err := s.client.PresignedPutObject(ctx, s.bucket, objectKey, expiresIn)
	if err != nil {
		return "", err
	}

	return presignedURL.String(), nil
}

func (s *minioStorage) GenerateDownloadURL(ctx context.Context, objectKey string, expiresIn time.Duration) (string, error) {
	// Set request parameters
	reqParams := make(url.Values)
	
	// Generate presigned URL for GET operation
	presignedURL, err := s.client.PresignedGetObject(ctx, s.bucket, objectKey, expiresIn, reqParams)
	if err != nil {
		return "", err
	}

	return presignedURL.String(), nil
}

func (s *minioStorage) UploadFile(ctx context.Context, objectKey string, reader io.Reader, contentType string) error {
	// Upload file with content type
	_, err := s.client.PutObject(ctx, s.bucket, objectKey, reader, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *minioStorage) DeleteFile(ctx context.Context, objectKey string) error {
	return s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
}

func (s *minioStorage) GetFileInfo(ctx context.Context, objectKey string) (int64, error) {
	objInfo, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return 0, err
	}
	return objInfo.Size, nil
}

// GetObject returns an object reader for direct access
func (s *minioStorage) GetObject(ctx context.Context, objectKey string) (*minio.Object, error) {
	return s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
}

// ListObjects lists objects with a given prefix
func (s *minioStorage) ListObjects(ctx context.Context, prefix string) <-chan minio.ObjectInfo {
	return s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
}
