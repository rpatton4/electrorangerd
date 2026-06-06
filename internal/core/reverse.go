package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// PostgresIntrospector reads a live PostgreSQL database and produces a
// Project describing its structure (typically with a single Database
// populated). Implemented by adapter/postgres.
type PostgresIntrospector interface {
	Introspect(ctx context.Context) (domain.Project, error)
}

// MigrationReader reads Flyway migration files from a directory and returns
// them in version order. Implemented by adapter/flyway.
type MigrationReader interface {
	Read(ctx context.Context, dir string) ([]domain.MigrationFile, error)
}

type reverseService struct {
	pg  PostgresIntrospector
	fly MigrationReader
	log *slog.Logger
}

// NewReverseService constructs a ReverseEngineer wired to a live-database
// introspector, a migration reader, and a structured logger.
func NewReverseService(pg PostgresIntrospector, fly MigrationReader, log *slog.Logger) port.ReverseEngineer {
	return &reverseService{pg: pg, fly: fly, log: log}
}

func (s *reverseService) FromDatabase(_ context.Context, _ string) (domain.Project, error) {
	return domain.Project{}, fmt.Errorf("reverse from-database: %w", errs.ErrNotImplemented)
}

func (s *reverseService) FromMigrations(_ context.Context, _ string) (domain.Project, error) {
	return domain.Project{}, fmt.Errorf("reverse from-migrations: %w", errs.ErrNotImplemented)
}
