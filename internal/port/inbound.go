// Package port declares the inbound (driving) ports that form the
// application's public API.
package port

import (
	"context"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
)

// ForwardEngineer pushes a Project's schema outward — applying it to a live
// database, or emitting a MigrationPlan for later execution. dbName selects
// which Database inside the Project is the target.
type ForwardEngineer interface {
	ApplyToDatabase(ctx context.Context, project domain.Project, dbName string) error
	EmitMigration(ctx context.Context, project domain.Project, dbName string) (domain.MigrationPlan, error)
}

// ReverseEngineer builds a Project by reading existing structure, either from
// a live database (selected by logical dbName, resolved through the project's
// ConnectionProfile via the vault — Plan B), or from a directory of migration
// files.
type ReverseEngineer interface {
	FromDatabase(ctx context.Context, dbName string) (domain.Project, error)
	FromMigrations(ctx context.Context, dir string) (domain.Project, error)
}

// DriftDetector compares two Projects and reports every structural difference
// between them.
type DriftDetector interface {
	Compare(left, right domain.Project) domain.DriftReport
}

// ProjectService handles opening and saving Project files from the local
// filesystem. Dictionaries are saved alongside as a separate document.
type ProjectService interface {
	Open(ctx context.Context, path string) (domain.Project, error)
	Save(ctx context.Context, path string, project domain.Project) error
}

// Validator checks a Project for logical and structural problems under a
// selected ValidationProfile, returning one ValidationIssue per finding.
type Validator interface {
	Validate(project domain.Project, profile domain.ValidationProfile) []domain.ValidationIssue
}

// History manages the undo/redo stack of Commands. Per-mode scoping of the
// stack happens in Plan C inside adapter/ui; the port itself is mode-agnostic.
type History interface {
	Push(cmd domain.Command)
	Undo() (domain.Command, bool)
	Redo() (domain.Command, bool)
}

// DictionaryService gets and sets data-dictionary entries keyed by schema-
// element reference.
type DictionaryService interface {
	Get(ref domain.DictionaryRef) (domain.DictionaryEntry, bool)
	Set(ref domain.DictionaryRef, entry domain.DictionaryEntry) error
}
