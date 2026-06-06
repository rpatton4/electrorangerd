package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// PostgresApplier executes raw DDL against a live PostgreSQL connection.
// Implemented by adapter/postgres.
type PostgresApplier interface {
	Apply(ctx context.Context, ddl string) error
}

// MigrationWriter writes a single migration file to a Flyway-style directory.
// The forward service iterates over a MigrationPlan and calls Write per file —
// keeping the adapter dumb and the orchestration in core.
type MigrationWriter interface {
	Write(ctx context.Context, dir string, file domain.MigrationFile) error
}

type forwardService struct {
	applier   PostgresApplier
	migrator  MigrationWriter
	validator port.Validator
	log       *slog.Logger
}

// NewForwardService constructs a ForwardEngineer wired to a live-database
// applier, a migration writer, a validator, and a structured logger.
func NewForwardService(applier PostgresApplier, migrator MigrationWriter, validator port.Validator, log *slog.Logger) port.ForwardEngineer {
	return &forwardService{applier: applier, migrator: migrator, validator: validator, log: log}
}

func (s *forwardService) ApplyToDatabase(_ context.Context, _ domain.Project, _ string) error {
	return fmt.Errorf("forward apply: %w", errs.ErrNotImplemented)
}

func (s *forwardService) EmitMigration(_ context.Context, _ domain.Project, _ string) (domain.MigrationPlan, error) {
	return domain.MigrationPlan{}, fmt.Errorf("forward emit: %w", errs.ErrNotImplemented)
}
