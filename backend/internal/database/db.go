package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// DB wraps a *sql.DB with the connection config.
type DB struct {
	*sql.DB
}

// Connect opens a PostgreSQL connection using the given DATABASE_URL.
// Railway provides DATABASE_URL in the standard postgres:// format.
func Connect(databaseURL string) (*DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}

	// Connection pool settings tuned for Railway's free tier limits.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	return &DB{db}, nil
}
