package flyway

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/core"
	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
)

type reader struct {
	log *slog.Logger
}

// NewReader returns a core.MigrationReader that reads Flyway-style SQL files.
func NewReader(log *slog.Logger) core.MigrationReader {
	return &reader{log: log}
}

// Read scans dir for versioned migration files and returns their parsed contents.
func (r *reader) Read(_ context.Context, _ string) ([]domain.MigrationFile, error) {
	return nil, fmt.Errorf("flyway read: %w", errs.ErrNotImplemented)
}
