// Package port declares the inbound (driving) ports that form the application's
// public API. These interfaces are the only surface the UI layer touches; each
// is implemented by a service in the core package, keeping the UI ignorant of
// persistence, database, or file-system details.
package port
