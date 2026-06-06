// Package port declares the inbound (driving) ports that form the application's
// public API.
package port

import (
	"context"

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
)

// ForwardEngineer pushes a schema outward — either applying it directly to a
// live database or emitting a migration file for later execution.
type ForwardEngineer interface {
	ApplyToDatabase(ctx context.Context, schema domain.Schema) error
	EmitMigration(ctx context.Context, schema domain.Schema) (filename string, body []byte, err error)
}

// ReverseEngineer builds a Schema by reading existing structure, either from a
// live database connection or from a directory of migration files.
type ReverseEngineer interface {
	FromDatabase(ctx context.Context) (domain.Schema, error)
	FromMigrations(ctx context.Context, dir string) (domain.Schema, error)
}

// DriftDetector compares two schemas and reports every structural difference
// between them.
type DriftDetector interface {
	Compare(left, right domain.Schema) domain.DriftReport
}

// ProjectService handles opening and saving schema projects from the local
// filesystem.
type ProjectService interface {
	Open(ctx context.Context, path string) (domain.Schema, error)
	Save(ctx context.Context, path string, schema domain.Schema) error
}

// Validator checks a schema for logical and structural problems, returning one
// ValidationIssue per finding.
type Validator interface {
	Validate(schema domain.Schema) []domain.ValidationIssue
}

// History manages the undo/redo stack of Commands for a single editing session.
type History interface {
	Push(cmd domain.Command)
	Undo() (domain.Command, bool)
	Redo() (domain.Command, bool)
}
