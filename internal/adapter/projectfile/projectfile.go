package projectfile

import (
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/core"
	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
)

type store struct {
	log *slog.Logger
}

// New returns a core.ProjectStore that persists schemas as JSON files.
func New(log *slog.Logger) core.ProjectStore {
	return &store{log: log}
}

// Load reads the project file at path and returns the decoded schema.
func (s *store) Load(_ string) (domain.Schema, error) {
	return domain.Schema{}, fmt.Errorf("projectfile load: %w", errs.ErrNotImplemented)
}

// Save encodes schema as JSON and writes it to path.
func (s *store) Save(_ string, _ domain.Schema) error {
	return fmt.Errorf("projectfile save: %w", errs.ErrNotImplemented)
}
