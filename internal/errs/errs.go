// Package errs defines the sentinel errors shared across all layers of the
// application.
package errs

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidSchema  = errors.New("invalid schema")
	ErrConnFailed     = errors.New("connection failed")
	ErrNotImplemented = errors.New("not implemented")
	ErrMigrationParse = errors.New("migration parse error")
	ErrSerialization  = errors.New("serialization error")

	// Vault sentinels (Plan B).
	ErrVaultLocked             = errors.New("vault is locked")
	ErrVaultAlreadyInitialized = errors.New("vault is already initialized")
	ErrVaultUninitialized      = errors.New("vault is uninitialized")
	ErrInvalidPassword         = errors.New("invalid password")
	ErrKeychainUnavailable     = errors.New("OS keychain unavailable")
)
