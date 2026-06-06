// Package postgres is a secondary adapter that implements core.PostgresIntrospector
// and core.PostgresApplier. It communicates with a PostgreSQL server via pgx and
// is driven by the application core; the UI layer never imports this package.
package postgres
