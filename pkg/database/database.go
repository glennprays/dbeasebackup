package database

import (
	"context"
	"database/sql"
)

// Database defines the interface for database operations
type Database interface {
	// Connect establishes a connection to the database
	Connect(ctx context.Context) error
	// Close closes the database connection
	Close() error
	// Ping verifies the connection is still alive
	Ping(ctx context.Context) error
	// Exec executes a query without returning any rows
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
	// Query executes a query that returns rows
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	// QueryRow executes a query that returns at most one row
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	// GetDB returns the underlying sql.DB instance
	GetDB() *sql.DB
}
