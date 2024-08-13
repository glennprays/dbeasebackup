package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/glennprays/dbeasebackup/pkg"
	"github.com/robfig/cron/v3"
)

type PostgresBackup interface {
	backupDatabase(ctx context.Context) error
	uploadToDrive(ctx context.Context, file *os.File, folderID string) error
	BackupDatabaseWithCRON(ctx context.Context, cronExpression string) error
	recordBackup(ctx context.Context, backupFile string, backupTime time.Time) error
	ensureTableExists(ctx context.Context) error
}

type postgresBackup struct {
	backupDir   string
	GoogleDrive pkg.GoogleDrive
	db          *sql.DB
}

func NewPostgresBackup(
	googleDrive pkg.GoogleDrive,
	db *sql.DB,
) PostgresBackup {

	p := &postgresBackup{
		backupDir:   "backups/postgres",
		GoogleDrive: googleDrive,
		db:          db,
	}

	// Ensure the backup table exists
	err := p.ensureTableExists(context.Background())
	if err != nil {
		log.Fatalf("unable to ensure table exists: %v", err)
	}

	return p
}

func (p *postgresBackup) backupDatabase(ctx context.Context) error {
	dbHost := os.Getenv("PG_HOST")
	dbPort := os.Getenv("PG_PORT")
	dbName := os.Getenv("PG_DATABASE")
	dbUser := os.Getenv("PG_USER")
	dbPassword := os.Getenv("PG_PASSWORD")

	if dbHost == "" || dbPort == "" || dbName == "" || dbUser == "" || dbPassword == "" {
		return fmt.Errorf("some environment variables are not set")
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(p.backupDir, os.ModePerm); err != nil {
		return fmt.Errorf("unable to create backup directory: %v", err)
	}

	backupTime := time.Now()

	backupFile := fmt.Sprintf("backup_%s.tar", backupTime.Format("2006-01-02_15-04-05"))
	backupFileDir := fmt.Sprintf("%s/%s", p.backupDir, backupFile)

	err := p.recordBackup(ctx, backupFile, backupTime)
	if err != nil {
		return fmt.Errorf("unable to record backup: %v", err)
	}

	cmd := exec.Command("pg_dump", "-h", dbHost, "-p", dbPort, "-U", dbUser, "-d", dbName, "-F", "t", "-f", backupFileDir)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", dbPassword))
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("unable to backup database: %v", err)
	}

	log.Printf("Database backup created at %s", backupFileDir)

	file, err := os.Open(backupFileDir)
	if err != nil {
		return fmt.Errorf("unable to open backup file: %v", err)
	}
	defer file.Close()

	folderID := os.Getenv("GOOGLE_DRIVE_FOLDER_ID")
	if folderID == "" {
		return fmt.Errorf("GOOGLE_DRIVE_FOLDER_ID environment variable is not set")
	}

	err = p.uploadToDrive(ctx, file, folderID)
	if err != nil {
		return fmt.Errorf("unable to upload backup file to Google Drive: %v", err)
	}

	// Delete the backup file after successful upload
	err = os.Remove(backupFileDir)
	if err != nil {
		return fmt.Errorf("unable to delete backup file: %v", err)
	}

	return nil
}

func (p *postgresBackup) ensureTableExists(ctx context.Context) error {
	// Define the table creation query
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS database_backups (
		id SERIAL PRIMARY KEY,
		backup_file TEXT NOT NULL,
		backup_time TIMESTAMP NOT NULL
	);`

	// Execute the query
	_, err := p.db.ExecContext(ctx, createTableQuery)
	if err != nil {
		return fmt.Errorf("unable to create table: %v", err)
	}

	return nil
}

func (p *postgresBackup) uploadToDrive(ctx context.Context, file *os.File, folderID string) error {
	err := p.GoogleDrive.UploadToDrive(ctx, file, folderID)
	if err != nil {
		return fmt.Errorf("unable to upload backup file to Google Drive: %v", err)
	}

	log.Println("Backup uploaded to Google Drive")
	return nil
}

func (p *postgresBackup) recordBackup(ctx context.Context, backupFile string, backupTime time.Time) error {
	_, err := p.db.ExecContext(ctx, "INSERT INTO database_backups (backup_file, backup_time) VALUES ($1, $2)", backupFile, backupTime)
	if err != nil {
		return fmt.Errorf("unable to record backup: %v", err)
	}

	log.Printf("Backup recorded in database: %s", backupFile)
	return nil
}

func (p *postgresBackup) BackupDatabaseWithCRON(ctx context.Context, cronExpression string) error {
	c := cron.New(cron.WithLocation(time.UTC))

	_, err := c.AddFunc(cronExpression, func() {
		fmt.Println("\n******* Running backup")
		backupCtx := context.Background()
		err := p.backupDatabase(backupCtx)
		if err != nil {
			log.Printf("unable to backup database: %v", err)
		}
		fmt.Println("******* Backup completed")
	})

	if err != nil {
		return fmt.Errorf("unable to add cron job: %v", err)
	}

	c.Start()
	return nil
}
