package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/core"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
)

type applier struct {
	conn *Connection
	log  *slog.Logger
}

// NewApplier returns a core.PostgresApplier backed by conn.
func NewApplier(conn *Connection, log *slog.Logger) core.PostgresApplier {
	return &applier{conn: conn, log: log}
}

// Apply executes ddl against the database within a transaction.
func (a *applier) Apply(_ context.Context, _ string) error {
	return fmt.Errorf("postgres apply: %w", errs.ErrNotImplemented)
}
