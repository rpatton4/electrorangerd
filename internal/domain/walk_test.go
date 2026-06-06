package domain

import (
	"reflect"
	"testing"
)

// fixtureProject returns a project with multiple databases and schemas so
// the walk helpers are exercised against the multi-DB / multi-schema shape
// they exist to support.
func fixtureProject() Project {
	return Project{
		Name: "Fixture",
		Databases: []Database{
			{
				Name: "primary",
				Schemas: []Schema{
					{Name: "public", Entities: []Entity{
						{ID: 1, Name: "Customer"},
						{ID: 2, Name: "Order"},
					}},
					{Name: "billing", Entities: []Entity{
						{ID: 3, Name: "Invoice"},
					}},
				},
			},
			{
				Name: "analytics",
				Schemas: []Schema{
					{Name: "public", Entities: []Entity{
						{ID: 4, Name: "Event"},
					}},
				},
			},
		},
		Relationships: []Relationship{
			{ID: 10, Name: "customer_orders", From: RelationshipEndpoint{Entity: 1}, To: RelationshipEndpoint{Entity: 2}},
			{ID: 11, Name: "order_invoice", From: RelationshipEndpoint{Entity: 2}, To: RelationshipEndpoint{Entity: 3}},
		},
	}
}

func TestEntitiesIn(t *testing.T) {
	p := fixtureProject()
	got := EntitiesIn(p)
	want := []EntityRecord{
		{Entity: Entity{ID: 1, Name: "Customer"}, Database: "primary", Schema: "public"},
		{Entity: Entity{ID: 2, Name: "Order"}, Database: "primary", Schema: "public"},
		{Entity: Entity{ID: 3, Name: "Invoice"}, Database: "primary", Schema: "billing"},
		{Entity: Entity{ID: 4, Name: "Event"}, Database: "analytics", Schema: "public"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EntitiesIn:\n  got:  %#v\n  want: %#v", got, want)
	}
}

func TestEntitiesInEmpty(t *testing.T) {
	if got := EntitiesIn(Project{}); got != nil {
		t.Fatalf("EntitiesIn(empty) = %#v, want nil", got)
	}
}

func TestFindEntity(t *testing.T) {
	p := fixtureProject()
	cases := []struct {
		name  string
		id    EntityID
		want  Entity
		db    string
		sch   string
		found bool
	}{
		{"primary public", 1, Entity{ID: 1, Name: "Customer"}, "primary", "public", true},
		{"primary billing", 3, Entity{ID: 3, Name: "Invoice"}, "primary", "billing", true},
		{"second database", 4, Entity{ID: 4, Name: "Event"}, "analytics", "public", true},
		{"missing", 99, Entity{}, "", "", false},
		{"zero id", 0, Entity{}, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, db, sch, ok := FindEntity(p, tc.id)
			if ok != tc.found {
				t.Fatalf("FindEntity ok = %v, want %v", ok, tc.found)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("FindEntity entity = %#v, want %#v", got, tc.want)
			}
			if db != tc.db || sch != tc.sch {
				t.Errorf("FindEntity db/schema = %q/%q, want %q/%q", db, sch, tc.db, tc.sch)
			}
		})
	}
}

func TestFindRelationship(t *testing.T) {
	p := fixtureProject()
	cases := []struct {
		name  string
		id    RelationshipID
		want  string
		index int
		found bool
	}{
		{"first", 10, "customer_orders", 0, true},
		{"second", 11, "order_invoice", 1, true},
		{"missing", 99, "", -1, false},
		{"zero id", 0, "", -1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rel, idx, ok := FindRelationship(p, tc.id)
			if ok != tc.found {
				t.Fatalf("FindRelationship ok = %v, want %v", ok, tc.found)
			}
			if rel.Name != tc.want {
				t.Errorf("FindRelationship name = %q, want %q", rel.Name, tc.want)
			}
			if idx != tc.index {
				t.Errorf("FindRelationship index = %d, want %d", idx, tc.index)
			}
		})
	}
}
