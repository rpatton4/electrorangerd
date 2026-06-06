package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// PostgresIntrospector reads the live schema from a Postgres database.
type PostgresIntrospector interface {
	Introspect(ctx context.Context) (domain.Schema, error)
}

// MigrationReader loads all migration files from a directory on disk.
type MigrationReader interface {
	Read(ctx context.Context, dir string) ([]domain.MigrationFile, error)
}

type reverseService struct {
	pg  PostgresIntrospector
	fly MigrationReader
	log *slog.Logger
}

// NewReverseService returns a port.ReverseEngineer backed by the provided
// outbound adapters.
func NewReverseService(pg PostgresIntrospector, fly MigrationReader, log *slog.Logger) port.ReverseEngineer {
	return &reverseService{
		pg:  pg,
		fly: fly,
		log: log,
	}
}

func (s *reverseService) FromDatabase(ctx context.Context) (domain.Schema, error) {
	return domain.Schema{}, fmt.Errorf("reverse: %w", errs.ErrNotImplemented)
}

func (s *reverseService) FromMigrations(ctx context.Context, dir string) (domain.Schema, error) {
	return domain.Schema{}, fmt.Errorf("reverse: %w", errs.ErrNotImplemented)
}
