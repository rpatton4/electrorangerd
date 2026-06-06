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

// New returns a core.ProjectStore that persists projects as JSON files.
func New(log *slog.Logger) core.ProjectStore {
	return &store{log: log}
}

// Load reads the project file at path and returns the decoded project.
func (s *store) Load(_ string) (domain.Project, error) {
	return domain.Project{}, fmt.Errorf("projectfile load: %w", errs.ErrNotImplemented)
}

// Save encodes project as JSON and writes it to path.
func (s *store) Save(_ string, _ domain.Project) error {
	return fmt.Errorf("projectfile save: %w", errs.ErrNotImplemented)
}
