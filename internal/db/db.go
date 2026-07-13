// Package db owns the single *sql.DB connection pool for the service.
// We use database/sql + lib/pq directly (no ORM) so query behavior is
// explicit and easy to reason about under load.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Connect opens a pooled connection to Postgres and verifies it with a
// ping so the process fails fast at startup rather than on first request.
func Connect(databaseURL string) (*sql.DB, error) {
	conn, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}

	// Pool sizing: tuned for a small/medium API instance. Override via
	// env-driven config if you outgrow these on a bigger box.
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(25)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return conn, nil
}
