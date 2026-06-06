package domain

// ChangeKind identifies what kind of structural change a drift entry represents.
type ChangeKind int

const (
	ChangeAdded    ChangeKind = iota
	ChangeRemoved
	ChangeModified
)

// Change describes a single structural difference detected between two schemas.
type Change struct {
	Kind    ChangeKind
	Target  string
	Detail  string
}

// DriftReport collects all Changes found when comparing two schemas.
type DriftReport struct {
	Changes []Change
}
