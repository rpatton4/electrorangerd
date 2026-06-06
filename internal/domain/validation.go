package domain

// ValidationProfile selects which subset of validation rules apply. Different
// modes tolerate different classes of issue — for example, a Reverse-
// engineering draft accepts missing PRIMARY KEY warnings that strict
// diagram editing would refuse.
type ValidationProfile int

const (
	ProfileStrict ValidationProfile = iota
	ProfileReverseDraft
	ProfileDiagramEdit
)

// ValidationIssue is one finding from a Validator run.
type ValidationIssue struct {
	Severity IssueSeverity
	Target   string
	Message  string
}

// IssueSeverity classifies a ValidationIssue.
type IssueSeverity int

const (
	SeverityInfo IssueSeverity = iota
	SeverityWarning
	SeverityError
)
