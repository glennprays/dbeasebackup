package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Storage implements the Storage interface for S3-compatible storage
type S3Storage struct {
	client *minio.Client
	bucket string
	prefix string
	logger *log.Logger
}

// S3Config holds S3-specific configuration
type S3Config struct {
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string // For S3-compatible services like MinIO
	Prefix          string // Folder prefix for all uploads (e.g., "backups/postgres")
}

// NewS3Storage creates a new S3 storage provider using MinIO SDK
func NewS3Storage(ctx context.Context, cfg S3Config, logger *log.Logger) (*S3Storage, error) {
	traceID := "s3-storage-init"

	// Parse endpoint to determine if SSL should be used
	useSSL := strings.HasPrefix(cfg.Endpoint, "https://")
	endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "https://"), "http://")

	// Create MinIO client
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: useSSL,
		Region: cfg.Region,
	})
	if err != nil {
		logger.Error(traceID, "Failed to create S3 client", nil, log.Error(err))
		return nil, fmt.Errorf("unable to create S3 client: %w", err)
	}

	storage := &S3Storage{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
		logger: logger,
	}

	logger.Info(traceID, "S3 storage initialized", nil,
		log.String("bucket", cfg.Bucket),
		log.String("region", cfg.Region),
		log.String("endpoint", endpoint),
		log.String("prefix", cfg.Prefix),
		log.Bool("useSSL", useSSL),
	)

	return storage, nil
}

// Upload uploads a file to S3
func (s *S3Storage) Upload(ctx context.Context, file io.Reader, filename string, opts UploadOptions) error {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	s.logger.Info(traceID, "Starting S3 upload", nil,
		log.String("filename", filename),
		log.String("bucket", s.bucket),
		log.String("component", "s3-storage"),
	)

	// Build the key path using configured prefix
	key := filename
	if s.prefix != "" {
		key = filepath.Join(s.prefix, filename)
	}

	// Determine content type
	contentType := opts.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload to S3 using MinIO SDK (supports streaming directly)
	info, err := s.client.PutObject(ctx, s.bucket, key, file, -1,
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		s.logger.Error(traceID, "Failed to upload file to S3", nil,
			log.Error(err),
			log.String("key", key),
			log.String("component", "s3-storage"),
		)
		return fmt.Errorf("unable to upload file to S3: %w", err)
	}

	s.logger.Info(traceID, "S3 upload completed", nil,
		log.String("key", key),
		log.Int64("size", info.Size),
		log.String("etag", info.ETag),
		log.String("component", "s3-storage"),
		log.String("duration", time.Since(startTime).String()),
	)

	return nil
}

// Download downloads a file from S3
func (s *S3Storage) Download(ctx context.Context, filename string) (io.ReadCloser, error) {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	// Build the key path using configured prefix
	key := filename
	if s.prefix != "" {
		key = filepath.Join(s.prefix, filename)
	}

	s.logger.Info(traceID, "Starting S3 download", nil,
		log.String("key", key),
		log.String("bucket", s.bucket),
		log.String("component", "s3-storage"),
	)

	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		s.logger.Error(traceID, "Failed to download file from S3", nil,
			log.Error(err),
			log.String("key", key),
			log.String("component", "s3-storage"),
		)
		return nil, fmt.Errorf("unable to download file from S3: %w", err)
	}

	s.logger.Info(traceID, "S3 download completed", nil,
		log.String("key", key),
		log.String("component", "s3-storage"),
		log.String("duration", time.Since(startTime).String()),
	)

	return object, nil
}

// Delete deletes a file from S3
func (s *S3Storage) Delete(ctx context.Context, filename string) error {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	// Build the key path using configured prefix
	key := filename
	if s.prefix != "" {
		key = filepath.Join(s.prefix, filename)
	}

	s.logger.Info(traceID, "Starting S3 delete", nil,
		log.String("key", key),
		log.String("bucket", s.bucket),
		log.String("component", "s3-storage"),
	)

	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		s.logger.Error(traceID, "Failed to delete file from S3", nil,
			log.Error(err),
			log.String("key", key),
			log.String("component", "s3-storage"),
		)
		return fmt.Errorf("unable to delete file from S3: %w", err)
	}

	s.logger.Info(traceID, "S3 delete completed", nil,
		log.String("key", key),
		log.String("component", "s3-storage"),
		log.String("duration", time.Since(startTime).String()),
	)

	return nil
}

// List lists files in S3
func (s *S3Storage) List(ctx context.Context, opts ListOptions) ([]FileInfo, error) {
	traceID := traceid.FromContext(ctx)
	startTime := time.Now()

	s.logger.Info(traceID, "Starting S3 list", nil,
		log.String("bucket", s.bucket),
		log.String("component", "s3-storage"),
	)

	var files []FileInfo

	// Build prefix for listing
	prefix := ""
	if s.prefix != "" {
		prefix = s.prefix + "/"
	}
	if opts.Prefix != "" {
		prefix = prefix + opts.Prefix
	}

	// Create channel for objects
	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	count := 0
	for object := range objectCh {
		if object.Err != nil {
			s.logger.Error(traceID, "Failed to list files from S3", nil,
				log.Error(object.Err),
				log.String("component", "s3-storage"),
			)
			return nil, fmt.Errorf("unable to list files from S3: %w", object.Err)
		}

		if opts.MaxFiles > 0 && count >= opts.MaxFiles {
			break
		}

		files = append(files, FileInfo{
			Name:       filepath.Base(object.Key),
			Size:       object.Size,
			CreatedAt:  object.LastModified,
			ModifiedAt: object.LastModified,
			ID:         object.Key,
		})
		count++
	}

	s.logger.Info(traceID, "S3 list completed", nil,
		log.Int("count", len(files)),
		log.String("component", "s3-storage"),
		log.String("duration", time.Since(startTime).String()),
	)

	return files, nil
}

// Health checks if the S3 storage service is healthy
func (s *S3Storage) Health(ctx context.Context) error {
	traceID := traceid.FromContext(ctx)

	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		s.logger.Error(traceID, "S3 health check failed", nil,
			log.Error(err),
			log.String("component", "s3-storage"),
		)
		return fmt.Errorf("S3 health check failed: %w", err)
	}

	if !exists {
		return fmt.Errorf("S3 bucket %s does not exist", s.bucket)
	}

	return nil
}

// Ensure S3Storage implements Storage interface
var _ Storage = (*S3Storage)(nil)
