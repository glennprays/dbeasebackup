package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/glennprays/log"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// GoogleDriveStorage implements the Storage interface for Google Drive
type GoogleDriveStorage struct {
	service *drive.Service
	logger  *log.Logger
}

// NewGoogleDriveStorage creates a new Google Drive storage instance
func NewGoogleDriveStorage(ctx context.Context, keyFilePath string, logger *log.Logger) (*GoogleDriveStorage, error) {
	traceID := "gdrive-init"

	g := &GoogleDriveStorage{
		logger: logger,
	}

	if err := g.initService(ctx, keyFilePath); err != nil {
		logger.Error(traceID, "Failed to initialize Google Drive service", nil, log.Error(err))
		return nil, err
	}

	logger.Info(traceID, "Google Drive service initialized", nil)
	return g, nil
}

// initService initializes the Google Drive service
func (g *GoogleDriveStorage) initService(ctx context.Context, keyPath string) error {
	traceID := "gdrive-init"

	b, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("unable to read service account key file: %w", err)
	}

	srv, err := drive.NewService(ctx, option.WithCredentialsJSON(b))
	if err != nil {
		return fmt.Errorf("unable to create Drive client: %w", err)
	}

	g.service = srv
	g.logger.Debug(traceID, "Google Drive client created", nil, log.String("keyPath", keyPath))
	return nil
}

// Upload uploads a file to Google Drive
func (g *GoogleDriveStorage) Upload(ctx context.Context, file io.Reader, filename string, opts UploadOptions) error {
	traceID := "gdrive-upload"

	if g.service == nil {
		return fmt.Errorf("Drive service not initialized")
	}

	driveFile := &drive.File{
		Name: filename,
	}

	if opts.FolderID != "" {
		driveFile.Parents = []string{opts.FolderID}
	}

	_, err := g.service.Files.Create(driveFile).Media(file).Context(ctx).Do()
	if err != nil {
		g.logger.Error(traceID, "Failed to upload file to Google Drive", nil,
			log.Error(err),
			log.String("filename", filename),
		)
		return fmt.Errorf("unable to upload file: %w", err)
	}

	g.logger.Info(traceID, "File uploaded to Google Drive", nil,
		log.String("filename", filename),
		log.String("folderID", opts.FolderID),
	)

	return nil
}

// Download downloads a file from Google Drive
func (g *GoogleDriveStorage) Download(ctx context.Context, filename string) (io.ReadCloser, error) {
	traceID := "gdrive-download"

	if g.service == nil {
		return nil, fmt.Errorf("Drive service not initialized")
	}

	// Search for the file by name
	query := fmt.Sprintf("name='%s' and trashed=false", filename)
	files, err := g.service.Files.List().Q(query).Context(ctx).Do()
	if err != nil {
		g.logger.Error(traceID, "Failed to search for file", nil,
			log.Error(err),
			log.String("filename", filename),
		)
		return nil, fmt.Errorf("unable to search for file: %w", err)
	}

	if len(files.Files) == 0 {
		return nil, fmt.Errorf("file not found: %s", filename)
	}

	fileID := files.Files[0].Id
	res, err := g.service.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		g.logger.Error(traceID, "Failed to download file", nil,
			log.Error(err),
			log.String("filename", filename),
			log.String("fileID", fileID),
		)
		return nil, fmt.Errorf("unable to download file: %w", err)
	}

	g.logger.Info(traceID, "File downloaded from Google Drive", nil,
		log.String("filename", filename),
	)

	return res.Body, nil
}

// Delete deletes a file from Google Drive
func (g *GoogleDriveStorage) Delete(ctx context.Context, filename string) error {
	traceID := "gdrive-delete"

	if g.service == nil {
		return fmt.Errorf("Drive service not initialized")
	}

	// Search for the file by name
	query := fmt.Sprintf("name='%s' and trashed=false", filename)
	files, err := g.service.Files.List().Q(query).Context(ctx).Do()
	if err != nil {
		g.logger.Error(traceID, "Failed to search for file to delete", nil,
			log.Error(err),
			log.String("filename", filename),
		)
		return fmt.Errorf("unable to search for file: %w", err)
	}

	if len(files.Files) == 0 {
		return fmt.Errorf("file not found: %s", filename)
	}

	fileID := files.Files[0].Id
	err = g.service.Files.Delete(fileID).Context(ctx).Do()
	if err != nil {
		g.logger.Error(traceID, "Failed to delete file", nil,
			log.Error(err),
			log.String("filename", filename),
			log.String("fileID", fileID),
		)
		return fmt.Errorf("unable to delete file: %w", err)
	}

	g.logger.Info(traceID, "File deleted from Google Drive", nil,
		log.String("filename", filename),
	)

	return nil
}

// List lists files in Google Drive
func (g *GoogleDriveStorage) List(ctx context.Context, opts ListOptions) ([]FileInfo, error) {
	traceID := "gdrive-list"

	if g.service == nil {
		return nil, fmt.Errorf("Drive service not initialized")
	}

	query := "trashed=false"
	if opts.FolderID != "" {
		query = fmt.Sprintf("'%s' in parents and trashed=false", opts.FolderID)
	}
	if opts.Prefix != "" {
		query = fmt.Sprintf("%s and name contains '%s'", query, opts.Prefix)
	}

	req := g.service.Files.List().Q(query).Context(ctx)
	if opts.MaxFiles > 0 {
		req = req.PageSize(int64(opts.MaxFiles))
	}

	files, err := req.Do()
	if err != nil {
		g.logger.Error(traceID, "Failed to list files", nil,
			log.Error(err),
			log.String("folderID", opts.FolderID),
		)
		return nil, fmt.Errorf("unable to list files: %w", err)
	}

	var fileInfos []FileInfo
	for _, f := range files.Files {
		createdAt, _ := parseTime(f.CreatedTime)
		modifiedAt, _ := parseTime(f.ModifiedTime)

		fileInfos = append(fileInfos, FileInfo{
			Name:       f.Name,
			Size:       f.Size,
			CreatedAt:  createdAt,
			ModifiedAt: modifiedAt,
			ID:         f.Id,
		})
	}

	g.logger.Info(traceID, "Files listed from Google Drive", nil,
		log.Int("count", len(fileInfos)),
	)

	return fileInfos, nil
}

// Health checks if the Google Drive service is healthy
func (g *GoogleDriveStorage) Health(ctx context.Context) error {
	if g.service == nil {
		return fmt.Errorf("Drive service not initialized")
	}

	// Try to list files with a limit of 1 to verify the service is working
	_, err := g.service.Files.List().PageSize(1).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("Drive service health check failed: %w", err)
	}

	return nil
}

// parseTime parses a time string in RFC3339 format
func parseTime(timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, timeStr)
}
