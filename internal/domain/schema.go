// Package domain contains the pure entity and value types that form the core
// of the electrorangerd data model.
package domain

// Project is the top-level container — an ERD project. A project models one
// or more target PostgreSQL databases, each holding one or more schema
// namespaces. Relationships live at the project level so they may cross
// schemas and databases (modelled for documentation; Flyway cannot enforce
// cross-database foreign keys).
type Project struct {
	Name          string
	Databases     []Database
	Relationships []Relationship
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

// Entity represents a single database table.
type Entity struct {
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
	Primary  bool
}

// Relationship describes an association between two entities. Crow's foot
// notation requires cardinality and optionality at each endpoint as
// independent axes. Provenance records whether the relationship was declared
// explicitly, inferred by reverse-engineering heuristics, or added manually.
type Relationship struct {
	Name              string
	From              EntityRef
	To                EntityRef
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
