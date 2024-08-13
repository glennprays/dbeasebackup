package pkg

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"os"
)

type PostgresDatabase interface {
	InitDatabase() (*sql.DB, error)
}

type postgresDatabase struct {
}

func NewPostgresDatabase() PostgresDatabase {
	return &postgresDatabase{}
}

func (p *postgresDatabase) InitDatabase() (*sql.DB, error) {
	var err error

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", os.Getenv("PG_HOST"), os.Getenv("PG_PORT"), os.Getenv("PG_USER"), os.Getenv("PG_PASSWORD"), os.Getenv("PG_DATABASE"))
	DB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	if err = DB.Ping(); err != nil {
		return nil, fmt.Errorf("unable to ping database: %v", err)
	}

	return DB, nil
}
