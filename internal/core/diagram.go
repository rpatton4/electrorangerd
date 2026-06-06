package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/errs"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// diagramEditorService applies pure transformations to a domain.Project in
// response to diagram-edit operations. Entity and Relationship identity is
// assigned by the service via monotonically increasing counters; counters
// are primed on first use to one past the highest existing ID so projects
// loaded with pre-existing IDs do not collide with newly-assigned ones.
type diagramEditorService struct {
	log               *slog.Logger
	nextEntityID      domain.EntityID
	nextRelationshipID domain.RelationshipID
}

// NewDiagramEditor returns a port.DiagramEditor that transforms Projects
// purely (no in-place mutation) in response to diagram-edit operations.
func NewDiagramEditor(log *slog.Logger) port.DiagramEditor {
	return &diagramEditorService{log: log}
}

// AddEntity inserts a new entity into the named schema and assigns it a
// fresh project-unique EntityID, which is returned alongside the new
// Project. The entity's Name must be non-empty and its attribute names
// must be unique (case-insensitive) within the entity.
func (s *diagramEditorService) AddEntity(_ context.Context, project domain.Project, db, schema string, entity domain.Entity) (domain.Project, domain.EntityID, error) {
	if err := validateAttributes(entity.Attributes); err != nil {
		return project, 0, fmt.Errorf("add entity: %w", err)
	}
	id := s.allocateEntityID(project)
	entity.ID = id

	out, err := withSchema(project, db, schema, "add entity", func(sch domain.Schema) (domain.Schema, error) {
		sch.Entities = append(append([]domain.Entity(nil), sch.Entities...), entity)
		return sch, nil
	})
	if err != nil {
		return project, 0, err
	}

	s.log.Info("add entity", "db", db, "schema", schema, "id", uint32(id), "name", entity.Name)
	return out, id, nil
}

// UpdateEntity replaces the entity identified by id with the supplied
// entity. The entity's existing database/schema location is preserved
// (the caller does not have to know where it lives). The new entity's ID
// must match the target.
func (s *diagramEditorService) UpdateEntity(_ context.Context, project domain.Project, id domain.EntityID, entity domain.Entity) (domain.Project, error) {
	if id == 0 {
		return project, fmt.Errorf("update entity: zero id: %w", errs.ErrNotFound)
	}
	if err := validateAttributes(entity.Attributes); err != nil {
		return project, fmt.Errorf("update entity: %w", err)
	}
	_, db, schema, ok := domain.FindEntity(project, id)
	if !ok {
		return project, fmt.Errorf("update entity: id %d: %w", uint32(id), errs.ErrNotFound)
	}
	entity.ID = id

	out, err := withSchema(project, db, schema, "update entity", func(sch domain.Schema) (domain.Schema, error) {
		newEntities := append([]domain.Entity(nil), sch.Entities...)
		for i, e := range newEntities {
			if e.ID == id {
				newEntities[i] = entity
				sch.Entities = newEntities
				return sch, nil
			}
		}
		return sch, fmt.Errorf("update entity: id %d disappeared mid-operation: %w", uint32(id), errs.ErrNotFound)
	})
	if err != nil {
		return project, err
	}

	s.log.Info("update entity", "id", uint32(id), "name", entity.Name)
	return out, nil
}

// DeleteEntity removes the entity identified by id from its schema, along
// with its placement entry and every relationship whose endpoint refers to
// it. Returns the new Project.
func (s *diagramEditorService) DeleteEntity(_ context.Context, project domain.Project, id domain.EntityID) (domain.Project, error) {
	if id == 0 {
		return project, fmt.Errorf("delete entity: zero id: %w", errs.ErrNotFound)
	}
	_, db, schema, ok := domain.FindEntity(project, id)
	if !ok {
		return project, fmt.Errorf("delete entity: id %d: %w", uint32(id), errs.ErrNotFound)
	}

	out, err := withSchema(project, db, schema, "delete entity", func(sch domain.Schema) (domain.Schema, error) {
		newEntities := make([]domain.Entity, 0, len(sch.Entities))
		for _, e := range sch.Entities {
			if e.ID != id {
				newEntities = append(newEntities, e)
			}
		}
		sch.Entities = newEntities
		return sch, nil
	})
	if err != nil {
		return project, err
	}

	out = withRelationships(out, func(rels []domain.Relationship) []domain.Relationship {
		kept := make([]domain.Relationship, 0, len(rels))
		for _, r := range rels {
			if r.From.Entity == id || r.To.Entity == id {
				continue
			}
			kept = append(kept, r)
		}
		return kept
	})

	out = withPlacements(out, func(p map[domain.EntityID]domain.Position) map[domain.EntityID]domain.Position {
		delete(p, id)
		return p
	})

	s.log.Info("delete entity", "id", uint32(id))
	return out, nil
}

// UpdatePlacement sets the on-canvas position of the entity identified by
// id. The target entity must exist; otherwise errs.ErrNotFound is returned.
func (s *diagramEditorService) UpdatePlacement(_ context.Context, project domain.Project, id domain.EntityID, pos domain.Position) (domain.Project, error) {
	if id == 0 {
		return project, fmt.Errorf("update placement: zero id: %w", errs.ErrNotFound)
	}
	if _, _, _, ok := domain.FindEntity(project, id); !ok {
		return project, fmt.Errorf("update placement: id %d: %w", uint32(id), errs.ErrNotFound)
	}
	out := withPlacements(project, func(p map[domain.EntityID]domain.Position) map[domain.EntityID]domain.Position {
		p[id] = pos
		return p
	})
	return out, nil
}

// AddRelationship inserts a new relationship into the project-level
// relationship list and assigns it a fresh ID. Both endpoints must
// reference existing entities in the project.
func (s *diagramEditorService) AddRelationship(_ context.Context, project domain.Project, rel domain.Relationship) (domain.Project, domain.RelationshipID, error) {
	if err := validateRelationshipEndpoints(project, rel); err != nil {
		return project, 0, fmt.Errorf("add relationship: %w", err)
	}
	id := s.allocateRelationshipID(project)
	rel.ID = id

	out := withRelationships(project, func(rels []domain.Relationship) []domain.Relationship {
		return append(rels, rel)
	})

	s.log.Info("add relationship", "id", uint32(id), "name", rel.Name)
	return out, id, nil
}

// UpdateRelationship replaces the relationship identified by id with rel.
// rel.ID is forced to match id. Endpoint entities must exist.
func (s *diagramEditorService) UpdateRelationship(_ context.Context, project domain.Project, id domain.RelationshipID, rel domain.Relationship) (domain.Project, error) {
	if id == 0 {
		return project, fmt.Errorf("update relationship: zero id: %w", errs.ErrNotFound)
	}
	if _, _, ok := domain.FindRelationship(project, id); !ok {
		return project, fmt.Errorf("update relationship: id %d: %w", uint32(id), errs.ErrNotFound)
	}
	if err := validateRelationshipEndpoints(project, rel); err != nil {
		return project, fmt.Errorf("update relationship: %w", err)
	}
	rel.ID = id

	out := withRelationships(project, func(rels []domain.Relationship) []domain.Relationship {
		updated := make([]domain.Relationship, len(rels))
		copy(updated, rels)
		for i, r := range updated {
			if r.ID == id {
				updated[i] = rel
				break
			}
		}
		return updated
	})

	s.log.Info("update relationship", "id", uint32(id), "name", rel.Name)
	return out, nil
}

// DeleteRelationship removes the relationship identified by id.
func (s *diagramEditorService) DeleteRelationship(_ context.Context, project domain.Project, id domain.RelationshipID) (domain.Project, error) {
	if id == 0 {
		return project, fmt.Errorf("delete relationship: zero id: %w", errs.ErrNotFound)
	}
	if _, _, ok := domain.FindRelationship(project, id); !ok {
		return project, fmt.Errorf("delete relationship: id %d: %w", uint32(id), errs.ErrNotFound)
	}

	out := withRelationships(project, func(rels []domain.Relationship) []domain.Relationship {
		kept := make([]domain.Relationship, 0, len(rels))
		for _, r := range rels {
			if r.ID != id {
				kept = append(kept, r)
			}
		}
		return kept
	})

	s.log.Info("delete relationship", "id", uint32(id))
	return out, nil
}

// allocateEntityID returns the next available EntityID for project. On
// first use the service primes its counter to max(existing IDs) + 1 so
// reverse-engineered or persistence-loaded projects don't collide.
func (s *diagramEditorService) allocateEntityID(project domain.Project) domain.EntityID {
	if s.nextEntityID == 0 {
		var maxID domain.EntityID
		for _, rec := range domain.EntitiesIn(project) {
			if rec.Entity.ID > maxID {
				maxID = rec.Entity.ID
			}
		}
		s.nextEntityID = maxID + 1
	}
	id := s.nextEntityID
	s.nextEntityID++
	return id
}

// allocateRelationshipID returns the next available RelationshipID with
// the same priming logic as allocateEntityID.
func (s *diagramEditorService) allocateRelationshipID(project domain.Project) domain.RelationshipID {
	if s.nextRelationshipID == 0 {
		var maxID domain.RelationshipID
		for _, r := range project.Relationships {
			if r.ID > maxID {
				maxID = r.ID
			}
		}
		s.nextRelationshipID = maxID + 1
	}
	id := s.nextRelationshipID
	s.nextRelationshipID++
	return id
}

// withSchema returns a new Project with fn applied to the identified schema.
// The Databases slice, the target Database's Schemas slice, and the target
// Schema's own embedded slices are copy-on-write — the input project and
// its sub-slices are never mutated.
func withSchema(project domain.Project, db, schema, op string, fn func(domain.Schema) (domain.Schema, error)) (domain.Project, error) {
	dbIdx, schemaIdx, err := locateSchema(project, db, schema, op)
	if err != nil {
		return project, err
	}
	newDBs := append([]domain.Database(nil), project.Databases...)
	newSchemas := append([]domain.Schema(nil), newDBs[dbIdx].Schemas...)
	sch, err := fn(newSchemas[schemaIdx])
	if err != nil {
		return project, err
	}
	newSchemas[schemaIdx] = sch
	newDBs[dbIdx].Schemas = newSchemas
	out := project
	out.Databases = newDBs
	return out, nil
}

// withRelationships returns a new Project with fn applied to a copy of the
// project-level Relationships slice. fn should treat the input slice as
// immutable — return a fresh slice for any mutation.
func withRelationships(project domain.Project, fn func([]domain.Relationship) []domain.Relationship) domain.Project {
	current := append([]domain.Relationship(nil), project.Relationships...)
	out := project
	out.Relationships = fn(current)
	return out
}

// withPlacements returns a new Project with fn applied to a copy of the
// Diagram.Placements map. The input project's map is never mutated. fn
// receives a fresh, writable map and may modify it in place; the returned
// map (which may be the same instance) becomes the new project's map.
func withPlacements(project domain.Project, fn func(map[domain.EntityID]domain.Position) map[domain.EntityID]domain.Position) domain.Project {
	newMap := make(map[domain.EntityID]domain.Position, len(project.Diagram.Placements))
	for k, v := range project.Diagram.Placements {
		newMap[k] = v
	}
	newMap = fn(newMap)
	out := project
	out.Diagram = domain.Diagram{Placements: newMap}
	return out
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

// validateAttributes enforces case-insensitive uniqueness of attribute
// names within an entity. Empty names are allowed (placeholders).
func validateAttributes(attrs []domain.Attribute) error {
	seen := make(map[string]struct{}, len(attrs))
	for _, a := range attrs {
		if a.Name == "" {
			continue
		}
		key := strings.ToLower(a.Name)
		if _, dup := seen[key]; dup {
			return fmt.Errorf("attribute %q duplicated: %w", a.Name, errs.ErrInvalidSchema)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// validateRelationshipEndpoints checks that both endpoints reference
// entities that exist in the project. An endpoint's Attribute, if set, is
// not validated here — Validator surfaces dangling attribute references.
func validateRelationshipEndpoints(project domain.Project, rel domain.Relationship) error {
	if rel.From.Entity == 0 || rel.To.Entity == 0 {
		return fmt.Errorf("endpoint entity unset: %w", errs.ErrNotFound)
	}
	if _, _, _, ok := domain.FindEntity(project, rel.From.Entity); !ok {
		return fmt.Errorf("from entity %d: %w", uint32(rel.From.Entity), errs.ErrNotFound)
	}
	if _, _, _, ok := domain.FindEntity(project, rel.To.Entity); !ok {
		return fmt.Errorf("to entity %d: %w", uint32(rel.To.Entity), errs.ErrNotFound)
	}
	return nil
}
