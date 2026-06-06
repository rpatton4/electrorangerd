package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rpatton4/electrorangerd/internal/core"
	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/errs"
)

type introspector struct {
	conn *Connection
	log  *slog.Logger
}

// NewIntrospector returns a core.PostgresIntrospector backed by conn.
func NewIntrospector(conn *Connection, log *slog.Logger) core.PostgresIntrospector {
	return &introspector{conn: conn, log: log}
}

// Introspect reads the live database and returns it as a domain.Project.
func (i *introspector) Introspect(_ context.Context) (domain.Project, error) {
	return domain.Project{}, fmt.Errorf("postgres introspect: %w", errs.ErrNotImplemented)
}
