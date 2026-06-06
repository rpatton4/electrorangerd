package domain

// EntityRecord pairs an Entity with the Database and Schema names that
// contain it. EntitiesIn yields one record per entity in the project,
// flattening the Database / Schema nesting so callers don't have to walk
// the tree themselves.
type EntityRecord struct {
	Entity   Entity
	Database string
	Schema   string
}

// EntitiesIn walks the project's databases and schemas and returns one
// EntityRecord per entity. Order is database-then-schema-then-entity,
// matching the underlying slice traversal.
func EntitiesIn(p Project) []EntityRecord {
	var out []EntityRecord
	for _, db := range p.Databases {
		for _, sch := range db.Schemas {
			for _, e := range sch.Entities {
				out = append(out, EntityRecord{Entity: e, Database: db.Name, Schema: sch.Name})
			}
		}
	}
	return out
}

// FindEntity locates the entity with the given ID in the project, returning
// the entity itself and the names of the database and schema that contain
// it. ok is false if no entity in the project has that ID.
func FindEntity(p Project, id EntityID) (entity Entity, database, schema string, ok bool) {
	if id == 0 {
		return Entity{}, "", "", false
	}
	for _, db := range p.Databases {
		for _, sch := range db.Schemas {
			for _, e := range sch.Entities {
				if e.ID == id {
					return e, db.Name, sch.Name, true
				}
			}
		}
	}
	return Entity{}, "", "", false
}

// FindRelationship locates the relationship with the given ID in the
// project and returns it along with its index in Project.Relationships.
// ok is false if no relationship has that ID.
func FindRelationship(p Project, id RelationshipID) (rel Relationship, index int, ok bool) {
	if id == 0 {
		return Relationship{}, -1, false
	}
	for i, r := range p.Relationships {
		if r.ID == id {
			return r, i, true
		}
	}
	return Relationship{}, -1, false
}
