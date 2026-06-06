// Package domain contains the pure entity and value types that form the core
// of the electrorangerd data model.
package domain

// EntityID is a project-unique, stable identifier for an Entity. It is
// assigned by the DiagramEditor service when an entity is created and
// survives renames, position changes, and cross-database moves. Zero is
// the unassigned sentinel — entities loaded from external sources without
// an ID are assigned one when they enter the editor.
type EntityID uint32

// RelationshipID is the analogous stable identifier for a Relationship.
type RelationshipID uint32

// Position is a 2-D point on the ERD canvas, expressed in device-independent
// pixels. Both axes increase toward the bottom-right.
type Position struct {
	X, Y float32
}

// Diagram is the visual / placement aggregate for a Project. It maps each
// entity (by ID) to its top-left position on the canvas. Entities without a
// placement are still part of the project; they just haven't been drawn yet
// (typical for reverse-engineered projects before the user opens them).
type Diagram struct {
	Placements map[EntityID]Position
}

// Project is the top-level container — an ERD project. A project models one
// or more target PostgreSQL databases, each holding one or more schema
// namespaces. Relationships live at the project level so they may cross
// schemas and databases (modelled for documentation; Flyway cannot enforce
// cross-database foreign keys). The Diagram aggregate carries layout state
// for the project's entities.
type Project struct {
	Name          string
	Databases     []Database
	Relationships []Relationship
	Diagram       Diagram
}

// Database represents a single PostgreSQL database target within a Project.
// ProfileName references a named ConnectionProfile resolved through the vault
// (Plan B) — the project file never contains plaintext DSNs.
type Database struct {
	Name        string
	ProfileName string
	Schemas     []Schema
}

// Schema is a PostgreSQL schema namespace (e.g. "public", "billing") holding
// the entities defined within that namespace.
type Schema struct {
	Name     string
	Entities []Entity
}

// Entity represents a single database table. ID is the project-unique stable
// identifier assigned by the DiagramEditor; it survives renames so placements
// and relationships can reliably refer back to the entity.
type Entity struct {
	ID          EntityID
	Name        string
	Attributes  []Attribute
	Indexes     []Index
	Constraints []Constraint
}

// Attribute describes a single column within an entity.
type Attribute struct {
	Name     string
	DataType string
	Nullable bool
	Default  string
	KeyKind  KeyKind
}

// KeyKind identifies the key marker shown alongside an attribute in the ERD:
// primary key, foreign key, indexed key, or none. The marker is a UI display
// choice that maps to underlying SQL semantics (PK column, FK constraint,
// index) when the entity is forward-engineered.
type KeyKind int

const (
	KeyNone KeyKind = iota
	KeyPrimary
	KeyForeign
	KeyIndex
)

// String returns the two-letter marker ("PK", "FK", "IK") or empty for
// KeyNone, suitable for direct display in an entity row's key column.
func (k KeyKind) String() string {
	switch k {
	case KeyPrimary:
		return "PK"
	case KeyForeign:
		return "FK"
	case KeyIndex:
		return "IK"
	}
	return ""
}

// RelationshipEndpoint identifies one side of a Relationship by the entity's
// stable ID plus, optionally, the participating attribute's name within that
// entity. Endpoints reference entities by ID (not EntityRef) because EntityID
// is project-unique and works naturally across database / schema boundaries.
type RelationshipEndpoint struct {
	Entity    EntityID
	Attribute string
}

// Relationship describes an association between two entities. Crow's foot
// notation requires cardinality and optionality at each endpoint as
// independent axes. Provenance records whether the relationship was declared
// explicitly, inferred by reverse-engineering heuristics, or added manually.
// ID is the project-unique stable identifier.
type Relationship struct {
	ID                RelationshipID
	Name              string
	From              RelationshipEndpoint
	To                RelationshipEndpoint
	SourceCardinality Cardinality
	SourceOptionality Optionality
	TargetCardinality Cardinality
	TargetOptionality Optionality
	OnDelete          string
	OnUpdate          string
	Provenance        Provenance
}

// EntityRef is a fully-qualified reference to an entity or one of its
// attributes within a Project's database / schema hierarchy.
type EntityRef struct {
	Database  string
	Schema    string
	Entity    string
	Attribute string
}

// Cardinality is one half of a crow's foot endpoint marker — how many of
// the related entity participate in the relationship.
type Cardinality int

const (
	CardinalityOne Cardinality = iota
	CardinalityMany
)

// Optionality is the other half of a crow's foot endpoint marker — whether
// participation in the relationship is required or optional.
type Optionality int

const (
	OptionalityRequired Optionality = iota
	OptionalityOptional
)

// Provenance records where a Relationship came from. Inferred relationships
// surface a confidence score and a human-readable reason so the reverse-
// engineering review UI can show why a heuristic fired.
type Provenance struct {
	Source     ProvenanceSource
	Confidence uint8
	Reason     string
}

// ProvenanceSource classifies how a Relationship entered the model.
type ProvenanceSource int

const (
	SourceDeclared ProvenanceSource = iota
	SourceInferred
	SourceManual
)

// Index describes an index on one or more attributes of an entity.
type Index struct {
	Name       string
	Attributes []string
	Unique     bool
}

// Constraint describes a named constraint applied to an entity.
type Constraint struct {
	Name       string
	Kind       ConstraintKind
	Expression string
}

// ConstraintKind identifies the category of a table constraint.
type ConstraintKind int

const (
	ConstraintCheck ConstraintKind = iota
	ConstraintUnique
	ConstraintNotNull
)
