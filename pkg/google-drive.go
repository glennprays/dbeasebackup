package pkg

import (
	"context"
	"fmt"
	"log"

	"os"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type GoogleDrive interface {
	initService(ctx context.Context, keyPath string) error
	UploadToDrive(ctx context.Context, file *os.File, folderID string) error
}

type googleDrive struct {
	service *drive.Service
}

func NewGoogleDrive(keyPath string) GoogleDrive {
	g := &googleDrive{}

	err := g.initService(context.Background(), keyPath)
	if err != nil {
		log.Fatalf("unable to initialize Drive service: %v", err)
	}

	return g
}

func (g *googleDrive) initService(ctx context.Context, keyPath string) error {

	b, err := os.ReadFile(keyPath)

	if err != nil {
		return fmt.Errorf("unable to read service account key file: %v", err)
	}

	srv, err := drive.NewService(ctx, option.WithCredentialsJSON(b))

	if err != nil {
		return fmt.Errorf("unable to create Drive client: %v", err)
	}

	g.service = srv
	return nil
}

func (g *googleDrive) UploadToDrive(ctx context.Context, file *os.File, folderID string) error {
	if g.service == nil {
		return fmt.Errorf("Drive service not initialized")

	}

	defer file.Close() // Ensure file is closed after the upload

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("unable to get file info: %v", err)
	}

	f := &drive.File{
		Name: fileInfo.Name(),
	}

	if folderID != "" {
		f.Parents = []string{folderID}
	}

	_, err = g.service.Files.Create(f).Media(file).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to upload file: %v", err)
	}

	return nil
}
