package domain

// MigrationFile holds the parsed contents of a single SQL migration file,
// split into its forward (Up) and rollback (Down) halves. Flyway Community
// Edition has no undo command, so Down is NOT executed by Flyway — it is
// reserved for our own forward "compensation" migration emission.
type MigrationFile struct {
	Version     string
	Description string
	Up          string
	Down        string
}

// MigrationPlan is the unit of forward-engineering work — the artifact users
// review before pushing schema changes to a database. It groups one or more
// migration files targeting a single database, along with a human-readable
// summary and any warnings raised during plan generation.
type MigrationPlan struct {
	TargetDatabase string
	Files          []MigrationFile
	Summary        string
	Warnings       []string
}
