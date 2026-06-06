package core

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/rpatton4/electrorangerd/internal/domain"
	"github.com/rpatton4/electrorangerd/internal/errs"
)

func newTestEditor() *diagramEditorService {
	return &diagramEditorService{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// emptyProject returns a project with one database and one schema, matching
// the shape the UI starts up with. No entities yet.
func emptyProject() domain.Project {
	return domain.Project{
		Name: "Test",
		Databases: []domain.Database{
			{Name: "default", Schemas: []domain.Schema{{Name: "public"}}},
		},
	}
}

func TestAddEntityAssignsMonotonicIDs(t *testing.T) {
	s := newTestEditor()
	p := emptyProject()

	p, id1, err := s.AddEntity(context.Background(), p, "default", "public", domain.Entity{Name: "A"})
	if err != nil {
		t.Fatalf("AddEntity A: %v", err)
	}
	p, id2, err := s.AddEntity(context.Background(), p, "default", "public", domain.Entity{Name: "B"})
	if err != nil {
		t.Fatalf("AddEntity B: %v", err)
	}
	if id1 == 0 || id2 == 0 {
		t.Fatalf("ids must be non-zero, got %d / %d", id1, id2)
	}
	if id1 == id2 {
		t.Fatalf("ids must be unique, got %d twice", id1)
	}
	if id2 <= id1 {
		t.Fatalf("ids must be monotonic, got %d then %d", id1, id2)
	}
	entities := p.Databases[0].Schemas[0].Entities
	if len(entities) != 2 || entities[0].ID != id1 || entities[1].ID != id2 {
		t.Fatalf("entities not appended with expected ids: %+v", entities)
	}
}

func TestAddEntityRejectsDuplicateAttributeNames(t *testing.T) {
	s := newTestEditor()
	p := emptyProject()
	_, _, err := s.AddEntity(context.Background(), p, "default", "public", domain.Entity{
		Name: "Dup",
		Attributes: []domain.Attribute{
			{Name: "id"}, {Name: "ID"},
		},
	})
	if !errors.Is(err, errs.ErrInvalidSchema) {
		t.Fatalf("expected ErrInvalidSchema, got %v", err)
	}
}

func TestAddEntityIDsPrimedByExistingMax(t *testing.T) {
	s := newTestEditor()
	p := emptyProject()
	// Simulate a project loaded with pre-existing IDs.
	p.Databases[0].Schemas[0].Entities = []domain.Entity{{ID: 42, Name: "Old"}}
	_, id, err := s.AddEntity(context.Background(), p, "default", "public", domain.Entity{Name: "New"})
	if err != nil {
		t.Fatalf("AddEntity: %v", err)
	}
	if id != 43 {
		t.Fatalf("expected id 43 (max+1), got %d", id)
	}
}

func TestAddEntityDoesNotMutateInput(t *testing.T) {
	s := newTestEditor()
	original := emptyProject()
	originalCopy := emptyProject()
	_, _, err := s.AddEntity(context.Background(), original, "default", "public", domain.Entity{Name: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, originalCopy) {
		t.Fatal("AddEntity mutated input project")
	}
}

func TestUpdateEntityByID(t *testing.T) {
	s := newTestEditor()
	p, id, _ := s.AddEntity(context.Background(), emptyProject(), "default", "public", domain.Entity{Name: "Before"})
	p, err := s.UpdateEntity(context.Background(), p, id, domain.Entity{Name: "After"})
	if err != nil {
		t.Fatalf("UpdateEntity: %v", err)
	}
	got, _, _, ok := domain.FindEntity(p, id)
	if !ok || got.Name != "After" || got.ID != id {
		t.Fatalf("expected updated entity with id %d, name After; got %+v ok=%v", id, got, ok)
	}
}

func TestUpdateEntityMissingID(t *testing.T) {
	s := newTestEditor()
	_, err := s.UpdateEntity(context.Background(), emptyProject(), 99, domain.Entity{Name: "Ghost"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteEntityCascadesRelationshipsAndPlacement(t *testing.T) {
	s := newTestEditor()
	p, idA, _ := s.AddEntity(context.Background(), emptyProject(), "default", "public", domain.Entity{Name: "A"})
	p, idB, _ := s.AddEntity(context.Background(), p, "default", "public", domain.Entity{Name: "B"})
	p, idC, _ := s.AddEntity(context.Background(), p, "default", "public", domain.Entity{Name: "C"})

	p, _ = s.UpdatePlacement(context.Background(), p, idB, domain.Position{X: 50, Y: 60})

	p, _, err := s.AddRelationship(context.Background(), p, domain.Relationship{
		Name: "ab",
		From: domain.RelationshipEndpoint{Entity: idA},
		To:   domain.RelationshipEndpoint{Entity: idB},
	})
	if err != nil {
		t.Fatalf("AddRelationship ab: %v", err)
	}
	p, _, err = s.AddRelationship(context.Background(), p, domain.Relationship{
		Name: "ac",
		From: domain.RelationshipEndpoint{Entity: idA},
		To:   domain.RelationshipEndpoint{Entity: idC},
	})
	if err != nil {
		t.Fatalf("AddRelationship ac: %v", err)
	}

	p, err = s.DeleteEntity(context.Background(), p, idB)
	if err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}

	if _, _, _, ok := domain.FindEntity(p, idB); ok {
		t.Fatal("entity B should be gone")
	}
	if _, ok := p.Diagram.Placements[idB]; ok {
		t.Fatal("placement for B should be gone")
	}
	if len(p.Relationships) != 1 || p.Relationships[0].Name != "ac" {
		t.Fatalf("only ac relationship should remain, got %+v", p.Relationships)
	}
	// Untouched entities still there
	if _, _, _, ok := domain.FindEntity(p, idA); !ok {
		t.Fatal("entity A should remain")
	}
	if _, _, _, ok := domain.FindEntity(p, idC); !ok {
		t.Fatal("entity C should remain")
	}
}

func TestDeleteEntityMissing(t *testing.T) {
	s := newTestEditor()
	_, err := s.DeleteEntity(context.Background(), emptyProject(), 99)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdatePlacement(t *testing.T) {
	s := newTestEditor()
	p, id, _ := s.AddEntity(context.Background(), emptyProject(), "default", "public", domain.Entity{Name: "X"})
	p, err := s.UpdatePlacement(context.Background(), p, id, domain.Position{X: 1, Y: 2})
	if err != nil {
		t.Fatalf("UpdatePlacement: %v", err)
	}
	if got := p.Diagram.Placements[id]; got != (domain.Position{X: 1, Y: 2}) {
		t.Fatalf("placement not stored, got %+v", got)
	}
}

func TestUpdatePlacementDoesNotMutateOldProject(t *testing.T) {
	s := newTestEditor()
	p, id, _ := s.AddEntity(context.Background(), emptyProject(), "default", "public", domain.Entity{Name: "X"})
	p, _ = s.UpdatePlacement(context.Background(), p, id, domain.Position{X: 1, Y: 2})

	p2, _ := s.UpdatePlacement(context.Background(), p, id, domain.Position{X: 9, Y: 9})
	if got := p.Diagram.Placements[id]; got != (domain.Position{X: 1, Y: 2}) {
		t.Fatalf("old project's placement mutated, got %+v", got)
	}
	if got := p2.Diagram.Placements[id]; got != (domain.Position{X: 9, Y: 9}) {
		t.Fatalf("new project's placement not updated, got %+v", got)
	}
}

func TestUpdatePlacementMissingEntity(t *testing.T) {
	s := newTestEditor()
	_, err := s.UpdatePlacement(context.Background(), emptyProject(), 99, domain.Position{})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAddRelationshipRequiresExistingEndpoints(t *testing.T) {
	s := newTestEditor()
	p := emptyProject()
	p, idA, _ := s.AddEntity(context.Background(), p, "default", "public", domain.Entity{Name: "A"})

	_, _, err := s.AddRelationship(context.Background(), p, domain.Relationship{
		Name: "dangling",
		From: domain.RelationshipEndpoint{Entity: idA},
		To:   domain.RelationshipEndpoint{Entity: 99},
	})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAddRelationshipAssignsID(t *testing.T) {
	s := newTestEditor()
	p, idA, _ := s.AddEntity(context.Background(), emptyProject(), "default", "public", domain.Entity{Name: "A"})
	p, idB, _ := s.AddEntity(context.Background(), p, "default", "public", domain.Entity{Name: "B"})

	p, rid1, err := s.AddRelationship(context.Background(), p, domain.Relationship{
		Name: "r1",
		From: domain.RelationshipEndpoint{Entity: idA},
		To:   domain.RelationshipEndpoint{Entity: idB},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, rid2, err := s.AddRelationship(context.Background(), p, domain.Relationship{
		Name: "r2",
		From: domain.RelationshipEndpoint{Entity: idA},
		To:   domain.RelationshipEndpoint{Entity: idB},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rid1 == 0 || rid2 == 0 || rid1 == rid2 {
		t.Fatalf("relationship IDs unique non-zero, got %d / %d", rid1, rid2)
	}
}

func TestUpdateRelationship(t *testing.T) {
	s := newTestEditor()
	p, idA, _ := s.AddEntity(context.Background(), emptyProject(), "default", "public", domain.Entity{Name: "A"})
	p, idB, _ := s.AddEntity(context.Background(), p, "default", "public", domain.Entity{Name: "B"})
	p, rid, _ := s.AddRelationship(context.Background(), p, domain.Relationship{
		Name: "old",
		From: domain.RelationshipEndpoint{Entity: idA},
		To:   domain.RelationshipEndpoint{Entity: idB},
	})
	p, err := s.UpdateRelationship(context.Background(), p, rid, domain.Relationship{
		Name: "new",
		From: domain.RelationshipEndpoint{Entity: idA},
		To:   domain.RelationshipEndpoint{Entity: idB},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _, ok := domain.FindRelationship(p, rid)
	if !ok || got.Name != "new" || got.ID != rid {
		t.Fatalf("expected updated relationship, got %+v ok=%v", got, ok)
	}
}

func TestUpdateRelationshipMissing(t *testing.T) {
	s := newTestEditor()
	_, err := s.UpdateRelationship(context.Background(), emptyProject(), 99, domain.Relationship{})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteRelationship(t *testing.T) {
	s := newTestEditor()
	p, idA, _ := s.AddEntity(context.Background(), emptyProject(), "default", "public", domain.Entity{Name: "A"})
	p, idB, _ := s.AddEntity(context.Background(), p, "default", "public", domain.Entity{Name: "B"})
	p, rid, _ := s.AddRelationship(context.Background(), p, domain.Relationship{
		Name: "rel",
		From: domain.RelationshipEndpoint{Entity: idA},
		To:   domain.RelationshipEndpoint{Entity: idB},
	})

	p, err := s.DeleteRelationship(context.Background(), p, rid)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Relationships) != 0 {
		t.Fatalf("relationship not removed, got %+v", p.Relationships)
	}
}

func TestDeleteRelationshipMissing(t *testing.T) {
	s := newTestEditor()
	_, err := s.DeleteRelationship(context.Background(), emptyProject(), 99)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestImmutabilityAcrossDelete(t *testing.T) {
	s := newTestEditor()
	p, id, _ := s.AddEntity(context.Background(), emptyProject(), "default", "public", domain.Entity{Name: "A"})
	before := domain.EntitiesIn(p)

	_, err := s.DeleteEntity(context.Background(), p, id)
	if err != nil {
		t.Fatal(err)
	}
	after := domain.EntitiesIn(p)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("DeleteEntity mutated the source project")
	}
}
