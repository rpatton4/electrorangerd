package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// ProjectStore persists and retrieves Schema values from disk.
type ProjectStore interface {
	Load(path string) (domain.Schema, error)
	Save(path string, schema domain.Schema) error
}

// ConfigStore persists and retrieves application Config values from disk.
type ConfigStore interface {
	Load() (domain.Config, error)
	Save(domain.Config) error
}

type projectService struct {
	store ProjectStore
	cfg   ConfigStore
	log   *slog.Logger
}

// NewProjectService returns a port.ProjectService backed by the provided
// outbound adapters.
func NewProjectService(store ProjectStore, cfg ConfigStore, log *slog.Logger) port.ProjectService {
	return &projectService{
		store: store,
		cfg:   cfg,
		log:   log,
	}
}

func (s *projectService) Open(ctx context.Context, path string) (domain.Schema, error) {
	return domain.Schema{}, fmt.Errorf("project: %w", errs.ErrNotImplemented)
}

func (s *projectService) Save(ctx context.Context, path string, schema domain.Schema) error {
	return fmt.Errorf("project: %w", errs.ErrNotImplemented)
}
