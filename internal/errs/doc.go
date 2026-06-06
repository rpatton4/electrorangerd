// Package errs defines the sentinel errors shared across all layers of the
// application. It is a peer of the domain package — it imports nothing from
// within this module — and provides a single place to test error identity with
// errors.Is without creating import cycles.
package errs
