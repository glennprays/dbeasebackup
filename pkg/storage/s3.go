package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/glennprays/dbeasebackup/pkg/traceid"
	"github.com/glennprays/log"
)

// S3Storage implements the Storage interface for S3-compatible storage
type S3Storage struct {
	client *s3.Client
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

// NewS3Storage creates a new S3 storage provider
func NewS3Storage(ctx context.Context, cfg S3Config, logger *log.Logger) (*S3Storage, error) {
	traceID := "s3-storage-init"

	// Create custom options for AWS config
	var opts []func(*config.LoadOptions) error

	// Set region
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	// Set credentials
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)))
	}

	// Load default config
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		logger.Error(traceID, "Failed to load AWS config", nil, log.Error(err))
		return nil, fmt.Errorf("unable to load AWS config: %w", err)
	}

	// Create S3 client with optional custom endpoint
	var clientOpts []func(*s3.Options)
	if cfg.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := s3.NewFromConfig(awsCfg, clientOpts...)

	storage := &S3Storage{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
		logger: logger,
	}

	logger.Info(traceID, "S3 storage initialized", nil,
		log.String("bucket", cfg.Bucket),
		log.String("region", cfg.Region),
		log.String("prefix", cfg.Prefix),
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

	// Read the file content to determine size and for retry capability
	content, err := io.ReadAll(file)
	if err != nil {
		s.logger.Error(traceID, "Failed to read file content", nil,
			log.Error(err),
			log.String("component", "s3-storage"),
		)
		return fmt.Errorf("unable to read file: %w", err)
	}

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

	// Upload to S3
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(content),
		ContentLength: aws.Int64(int64(len(content))),
		ContentType:   aws.String(contentType),
	})
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
		log.Int("size", len(content)),
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

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
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

	return output.Body, nil
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

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
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

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	}

	// Use configured prefix as base
	if s.prefix != "" {
		input.Prefix = aws.String(s.prefix + "/")
	}

	// Append additional prefix if provided in opts
	if opts.Prefix != "" {
		prefix := opts.Prefix
		if s.prefix != "" {
			prefix = s.prefix + "/" + opts.Prefix
		}
		input.Prefix = aws.String(prefix)
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, input)

	count := 0
	for paginator.HasMorePages() {
		if opts.MaxFiles > 0 && count >= opts.MaxFiles {
			break
		}

		page, err := paginator.NextPage(ctx)
		if err != nil {
			s.logger.Error(traceID, "Failed to list files from S3", nil,
				log.Error(err),
				log.String("component", "s3-storage"),
			)
			return nil, fmt.Errorf("unable to list files from S3: %w", err)
		}

		for _, obj := range page.Contents {
			if opts.MaxFiles > 0 && count >= opts.MaxFiles {
				break
			}

			files = append(files, FileInfo{
				Name:       filepath.Base(*obj.Key),
				Size:       *obj.Size,
				CreatedAt:  *obj.LastModified,
				ModifiedAt: *obj.LastModified,
				ID:         *obj.Key,
			})
			count++
		}
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

	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		s.logger.Error(traceID, "S3 health check failed", nil,
			log.Error(err),
			log.String("component", "s3-storage"),
		)
		return fmt.Errorf("S3 health check failed: %w", err)
	}

	return nil
}

// Ensure S3Storage implements Storage interface
var _ Storage = (*S3Storage)(nil)
