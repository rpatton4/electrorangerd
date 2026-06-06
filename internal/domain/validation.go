package domain

// IssueSeverity indicates how serious a validation finding is.
type IssueSeverity int

const (
	SeverityInfo    IssueSeverity = iota
	SeverityWarning
	SeverityError
)

// ValidationIssue describes a single problem found during schema validation.
type ValidationIssue struct {
	Severity IssueSeverity
	Target   string
	Message  string
}
