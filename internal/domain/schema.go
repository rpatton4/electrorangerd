// Package domain contains the pure entity and value types that form the core
// of the electrorangerd data model.
package domain

// ConstraintKind identifies the category of a table constraint.
type ConstraintKind int

const (
	ConstraintCheck   ConstraintKind = iota
	ConstraintUnique
	ConstraintNotNull
)

// Schema is the top-level representation of a database schema, grouping all
// entities and the relationships between them.
type Schema struct {
	Name          string
	Entities      []Entity
	Relationships []Relationship
}

// Entity represents a single database table, including its attributes,
// indexes, and constraints.
type Entity struct {
	Name        string
	Schema      string
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

// Relationship describes a foreign-key association between two entities.
type Relationship struct {
	Name          string
	FromEntity    string
	FromAttribute string
	ToEntity      string
	ToAttribute   string
	OnDelete      string
	OnUpdate      string
}

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
