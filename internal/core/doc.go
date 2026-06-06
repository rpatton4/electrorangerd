// Package core contains the application services that implement the inbound
// port interfaces defined in internal/port. Each service declares the outbound
// interfaces it depends on at the top of its own source file (Go idiom:
// interfaces at point of use), so callers and adapters need only satisfy those
// narrow contracts. This package imports domain, port, and errs only — it
// never imports any adapter package.
package core
