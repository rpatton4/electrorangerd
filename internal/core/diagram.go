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
	dbIdx, schemaIdx, err := locateSchema(project, db, schema, "add entity")
	if err != nil {
		return project, err
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

func (s *diagramEditorService) UpdateEntity(_ context.Context, project domain.Project, db, schema string, entityIdx int, entity domain.Entity) (domain.Project, error) {
	dbIdx, schemaIdx, err := locateSchema(project, db, schema, "update entity")
	if err != nil {
		return project, err
	}
	entities := project.Databases[dbIdx].Schemas[schemaIdx].Entities
	if entityIdx < 0 || entityIdx >= len(entities) {
		return project, fmt.Errorf("update entity: index %d out of range [0,%d): %w", entityIdx, len(entities), errs.ErrNotFound)
	}

	newDBs := append([]domain.Database(nil), project.Databases...)
	newSchemas := append([]domain.Schema(nil), newDBs[dbIdx].Schemas...)
	target := newSchemas[schemaIdx]
	newEntities := append([]domain.Entity(nil), target.Entities...)
	newEntities[entityIdx] = entity
	target.Entities = newEntities
	newSchemas[schemaIdx] = target
	newDBs[dbIdx].Schemas = newSchemas

	out := project
	out.Databases = newDBs

	s.log.Info("update entity", "db", db, "schema", schema, "idx", entityIdx, "name", entity.Name)
	return out, nil
}

func locateSchema(project domain.Project, db, schema, op string) (int, int, error) {
	dbIdx := -1
	for i := range project.Databases {
		if project.Databases[i].Name == db {
			dbIdx = i
			break
		}
	}
	if dbIdx < 0 {
		return -1, -1, fmt.Errorf("%s: database %q: %w", op, db, errs.ErrNotFound)
	}
	schemaIdx := -1
	for i := range project.Databases[dbIdx].Schemas {
		if project.Databases[dbIdx].Schemas[i].Name == schema {
			schemaIdx = i
			break
		}
	}
	if schemaIdx < 0 {
		return -1, -1, fmt.Errorf("%s: schema %q in database %q: %w", op, schema, db, errs.ErrNotFound)
	}
	return dbIdx, schemaIdx, nil
}
