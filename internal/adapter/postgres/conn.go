package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connection holds the pgx connection pool and a structured logger for all
// postgres adapter operations.
type Connection struct {
	dsn  string
	pool *pgxpool.Pool
	log  *slog.Logger
}

// New constructs a Connection. This stub stores the DSN and logger but does not
// open an actual database connection; that is left for the real implementation.
func New(_ context.Context, dsn string, log *slog.Logger) (*Connection, error) {
	return &Connection{
		dsn: dsn,
		log: log,
	}, nil
}

// Close releases the connection pool. This stub is a no-op.
func (c *Connection) Close() error {
	return nil
}
