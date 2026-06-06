package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/errs"
	"github.com/rpatton4/electrorangerd/internal/port"
)

// ProjectStore loads and saves Project documents from on-disk storage.
// Implemented by adapter/projectfile.
type ProjectStore interface {
	Load(path string) (domain.Project, error)
	Save(path string, project domain.Project) error
}

// ConfigStore loads and saves user preferences (window state, recent files,
// connection-profile names). Implemented by adapter/config.
type ConfigStore interface {
	Load() (domain.Config, error)
	Save(domain.Config) error
}

// DictionaryStore is declared in dictionary.go (same package). The project
// service uses it so Open and Save can lift and persist the dictionary
// document alongside the Project.

type projectService struct {
	store     ProjectStore
	dictStore DictionaryStore
	cfg       ConfigStore
	log       *slog.Logger
}

// NewProjectService constructs a ProjectService wired to a Project file
// store, a Dictionary store, a Config store, and a structured logger. Either
// dictStore or cfg may be nil during scaffolding; stub methods do not
// dereference them.
func NewProjectService(store ProjectStore, dictStore DictionaryStore, cfg ConfigStore, log *slog.Logger) port.ProjectService {
	return &projectService{store: store, dictStore: dictStore, cfg: cfg, log: log}
}

func (s *projectService) Open(_ context.Context, _ string) (domain.Project, error) {
	return domain.Project{}, fmt.Errorf("project open: %w", errs.ErrNotImplemented)
}

func (s *projectService) Save(_ context.Context, _ string, _ domain.Project) error {
	return fmt.Errorf("project save: %w", errs.ErrNotImplemented)
}
