package domain

// MigrationFile holds the parsed contents of a single SQL migration file,
// split into its forward (Up) and rollback (Down) halves.
type MigrationFile struct {
	Version     string
	Description string
	Up          string
	Down        string
}
