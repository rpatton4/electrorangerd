package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/core"
	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
)

type introspector struct {
	conn *Connection
	log  *slog.Logger
}

// NewIntrospector returns a core.PostgresIntrospector backed by conn.
func NewIntrospector(conn *Connection, log *slog.Logger) core.PostgresIntrospector {
	return &introspector{conn: conn, log: log}
}

// Introspect reads the live database schema and returns it as a domain.Schema.
func (i *introspector) Introspect(_ context.Context) (domain.Schema, error) {
	return domain.Schema{}, fmt.Errorf("postgres introspect: %w", errs.ErrNotImplemented)
}
