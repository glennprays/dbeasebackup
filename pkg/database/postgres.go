package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/glennprays/dbeasebackup/config"
	"github.com/glennprays/log"
	_ "github.com/lib/pq"
)

// PostgresDatabase implements the Database interface for PostgreSQL
type PostgresDatabase struct {
	db     *sql.DB
	cfg    *config.DatabaseConfig
	logger *log.Logger
}

// NewPostgresDatabase creates a new PostgreSQL database instance
func NewPostgresDatabase(cfg *config.DatabaseConfig, logger *log.Logger) *PostgresDatabase {
	return &PostgresDatabase{
		cfg:    cfg,
		logger: logger,
	}
}

// Connect establishes a connection to the PostgreSQL database
func (p *PostgresDatabase) Connect(ctx context.Context) error {
	traceID := "db-connect"

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		p.cfg.Host, p.cfg.Port, p.cfg.User, p.cfg.Password, p.cfg.Database,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		p.logger.Error(traceID, "Failed to open database connection", nil, log.Error(err))
		return fmt.Errorf("unable to connect to database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		p.logger.Error(traceID, "Failed to ping database", nil, log.Error(err))
		return fmt.Errorf("unable to ping database: %w", err)
	}

	p.db = db
	p.logger.Info(traceID, "Database connection established", nil,
		log.String("host", p.cfg.Host),
		log.String("database", p.cfg.Database),
	)

	return nil
}

// Close closes the database connection
func (p *PostgresDatabase) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// Ping verifies the connection is still alive
func (p *PostgresDatabase) Ping(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("database not connected")
	}
	return p.db.PingContext(ctx)
}

// Exec executes a query without returning any rows
func (p *PostgresDatabase) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	return p.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows
func (p *PostgresDatabase) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	return p.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row
func (p *PostgresDatabase) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return p.db.QueryRowContext(ctx, query, args...)
}

// GetDB returns the underlying sql.DB instance
func (p *PostgresDatabase) GetDB() *sql.DB {
	return p.db
}
