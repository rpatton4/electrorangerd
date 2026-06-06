package flyway

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rpatton4/electrorangerd/internal/core"
	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/errs"
)

type writer struct {
	log *slog.Logger
}

// NewWriter returns a core.MigrationWriter that writes Flyway-style SQL files.
func NewWriter(log *slog.Logger) core.MigrationWriter {
	return &writer{log: log}
}

// Write serializes file to disk under dir using Flyway naming conventions.
func (w *writer) Write(_ context.Context, _ string, _ domain.MigrationFile) error {
	return fmt.Errorf("flyway write: %w", errs.ErrNotImplemented)
}
