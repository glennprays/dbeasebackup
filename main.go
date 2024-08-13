package main

import (
	"context"
	"log"
	"os"

	"github.com/glennprays/database-auto-backup/pkg"
	service "github.com/glennprays/database-auto-backup/services"
	"github.com/joho/godotenv"
)

func init() {
	goenv := os.Getenv("GO_ENV")
	if goenv != "production" {
		log.Println("Loading .env file...")
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
	}
}

func main() {
	db, err := pkg.NewPostgresDatabase().InitDatabase()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Database connection established...")

	googleKeyPath := "service-account-key.json"
	googleDrive := pkg.NewGoogleDrive(googleKeyPath)

	backup := service.NewPostgresBackup(googleDrive, db)

	cronExpression := os.Getenv("CRON_SCHEDULE")
	err = backup.BackupDatabaseWithCRON(context.Background(), cronExpression)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Database Auto Backup Service Started...")
	select {}
}
