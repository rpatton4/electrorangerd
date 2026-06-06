package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

type diagramEditorService struct {
	log *slog.Logger
}

// NewDiagramEditor returns a port.DiagramEditor that transforms Projects
// purely (no in-place mutation) in response to diagram-edit operations.
func NewDiagramEditor(log *slog.Logger) port.DiagramEditor {
	return &diagramEditorService{log: log}
}

func (s *diagramEditorService) AddEntity(_ context.Context, project domain.Project, db, schema string, entity domain.Entity) (domain.Project, error) {
	dbIdx := -1
	for i := range project.Databases {
		if project.Databases[i].Name == db {
			dbIdx = i
			break
		}
	}
	if dbIdx < 0 {
		return project, fmt.Errorf("add entity: database %q: %w", db, errs.ErrNotFound)
	}
	schemaIdx := -1
	for i := range project.Databases[dbIdx].Schemas {
		if project.Databases[dbIdx].Schemas[i].Name == schema {
			schemaIdx = i
			break
		}
	}
	if schemaIdx < 0 {
		return project, fmt.Errorf("add entity: schema %q in database %q: %w", schema, db, errs.ErrNotFound)
	}

	newDBs := append([]domain.Database(nil), project.Databases...)
	newSchemas := append([]domain.Schema(nil), newDBs[dbIdx].Schemas...)
	target := newSchemas[schemaIdx]
	target.Entities = append(append([]domain.Entity(nil), target.Entities...), entity)
	newSchemas[schemaIdx] = target
	newDBs[dbIdx].Schemas = newSchemas

	out := project
	out.Databases = newDBs

	s.log.Info("add entity", "db", db, "schema", schema, "name", entity.Name)
	return out, nil
}
