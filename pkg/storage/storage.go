package storage

import (
	"context"
	"io"
	"time"
)

// UploadOptions contains options for file upload
type UploadOptions struct {
	FolderID  string
	MimeType  string
	Overwrite bool
}

// ListOptions contains options for listing files
type ListOptions struct {
	FolderID string
	MaxFiles int
	Prefix   string
}

// FileInfo contains information about a stored file
type FileInfo struct {
	Name       string
	Size       int64
	CreatedAt  time.Time
	ModifiedAt time.Time
	ID         string
}

// Storage defines the interface for storage operations
type Storage interface {
	// Upload uploads a file to storage
	Upload(ctx context.Context, file io.Reader, filename string, opts UploadOptions) error
	// Download downloads a file from storage
	Download(ctx context.Context, filename string) (io.ReadCloser, error)
	// Delete deletes a file from storage
	Delete(ctx context.Context, filename string) error
	// List lists files in storage
	List(ctx context.Context, opts ListOptions) ([]FileInfo, error)
	// Health checks if the storage service is healthy
	Health(ctx context.Context) error
}
