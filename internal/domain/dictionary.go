package domain

// Dictionary is the data-dictionary document for a Project. It is a top-level
// sibling of Project (NOT embedded), so dictionary entries have their own
// lifecycle independent from the schema definitions they describe.
type Dictionary struct {
	Entries map[DictionaryRef]DictionaryEntry
}

// DictionaryRef identifies the schema element a DictionaryEntry describes.
// Kind selects which of the qualifier fields are meaningful: RefEntity uses
// Database+Schema+Entity, RefAttribute additionally uses Attribute, and
// RefRelationship uses Relationship (the Relationship.Name).
type DictionaryRef struct {
	Kind         DictionaryRefKind
	Database     string
	Schema       string
	Entity       string
	Attribute    string
	Relationship string
}

// DictionaryRefKind selects what kind of schema element a reference targets.
type DictionaryRefKind int

const (
	RefEntity DictionaryRefKind = iota
	RefAttribute
	RefRelationship
)

// DictionaryEntry is the documentation attached to a schema element.
// Description is rendered as markdown via gioui.org/x/markdown in the UI.
type DictionaryEntry struct {
	Description     string
	Classification  DataClassification
	GlossaryAliases []string
}

// DataClassification is the sensitivity classification for the data held by
// the entity, attribute, or relationship. Internal is the default; PII and
// CUI (Controlled Unclassified Information) carry stricter handling
// requirements and surface in the diagram with distinct styling.
type DataClassification int

const (
	ClassificationInternal DataClassification = iota
	ClassificationPII
	ClassificationCUI
)
