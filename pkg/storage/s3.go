package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/glennprays/log"
)

// S3Storage implements the Storage interface for S3-compatible storage
type S3Storage struct {
	client *s3.Client
	bucket string
	logger *log.Logger
}

// S3Config holds S3-specific configuration
type S3Config struct {
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string // For S3-compatible services like MinIO
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
		logger: logger,
	}

	logger.Info(traceID, "S3 storage initialized", nil,
		log.String("bucket", cfg.Bucket),
		log.String("region", cfg.Region),
	)

	return storage, nil
}

// Upload uploads a file to S3
func (s *S3Storage) Upload(ctx context.Context, file io.Reader, filename string, opts UploadOptions) error {
	traceID := "s3-upload"

	// Read the file content to determine size and for retry capability
	content, err := io.ReadAll(file)
	if err != nil {
		s.logger.Error(traceID, "Failed to read file content", nil, log.Error(err))
		return fmt.Errorf("unable to read file: %w", err)
	}

	// Build the key path
	key := filename
	if opts.FolderID != "" {
		key = filepath.Join(opts.FolderID, filename)
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
		)
		return fmt.Errorf("unable to upload file to S3: %w", err)
	}

	s.logger.Info(traceID, "File uploaded to S3", nil,
		log.String("key", key),
		log.Int("size", len(content)),
	)

	return nil
}

// Download downloads a file from S3
func (s *S3Storage) Download(ctx context.Context, filename string) (io.ReadCloser, error) {
	traceID := "s3-download"

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(filename),
	})
	if err != nil {
		s.logger.Error(traceID, "Failed to download file from S3", nil,
			log.Error(err),
			log.String("key", filename),
		)
		return nil, fmt.Errorf("unable to download file from S3: %w", err)
	}

	s.logger.Info(traceID, "File downloaded from S3", nil,
		log.String("key", filename),
	)

	return output.Body, nil
}

// Delete deletes a file from S3
func (s *S3Storage) Delete(ctx context.Context, filename string) error {
	traceID := "s3-delete"

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(filename),
	})
	if err != nil {
		s.logger.Error(traceID, "Failed to delete file from S3", nil,
			log.Error(err),
			log.String("key", filename),
		)
		return fmt.Errorf("unable to delete file from S3: %w", err)
	}

	s.logger.Info(traceID, "File deleted from S3", nil,
		log.String("key", filename),
	)

	return nil
}

// List lists files in S3
func (s *S3Storage) List(ctx context.Context, opts ListOptions) ([]FileInfo, error) {
	traceID := "s3-list"

	var files []FileInfo

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	}

	if opts.FolderID != "" {
		input.Prefix = aws.String(opts.FolderID + "/")
	}

	if opts.Prefix != "" {
		prefix := opts.Prefix
		if opts.FolderID != "" {
			prefix = opts.FolderID + "/" + opts.Prefix
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
			s.logger.Error(traceID, "Failed to list files from S3", nil, log.Error(err))
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

	s.logger.Info(traceID, "Files listed from S3", nil,
		log.Int("count", len(files)),
	)

	return files, nil
}

// Health checks if the S3 storage service is healthy
func (s *S3Storage) Health(ctx context.Context) error {
	traceID := "s3-health"

	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		s.logger.Error(traceID, "S3 health check failed", nil, log.Error(err))
		return fmt.Errorf("S3 health check failed: %w", err)
	}

	return nil
}

// Ensure S3Storage implements Storage interface
var _ Storage = (*S3Storage)(nil)
