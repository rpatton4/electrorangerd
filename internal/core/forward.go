package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// PostgresApplier executes DDL statements against a live Postgres database.
type PostgresApplier interface {
	Apply(ctx context.Context, ddl string) error
}

// MigrationWriter persists a MigrationFile to a directory on disk.
type MigrationWriter interface {
	Write(ctx context.Context, dir string, file domain.MigrationFile) error
}

type forwardService struct {
	applier   PostgresApplier
	migrator  MigrationWriter
	validator port.Validator
	log       *slog.Logger
}

// NewForwardService returns a port.ForwardEngineer backed by the provided
// outbound adapters and validator.
func NewForwardService(applier PostgresApplier, migrator MigrationWriter, validator port.Validator, log *slog.Logger) port.ForwardEngineer {
	return &forwardService{
		applier:   applier,
		migrator:  migrator,
		validator: validator,
		log:       log,
	}
}

func (s *forwardService) ApplyToDatabase(ctx context.Context, schema domain.Schema) error {
	return fmt.Errorf("forward: %w", errs.ErrNotImplemented)
}

func (s *forwardService) EmitMigration(ctx context.Context, schema domain.Schema) (string, []byte, error) {
	return "", nil, fmt.Errorf("forward: %w", errs.ErrNotImplemented)
}
