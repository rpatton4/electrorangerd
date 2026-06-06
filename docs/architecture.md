# Architecture

## Overview

electrorangerd follows a hexagonal (ports-and-adapters) architecture. All business logic lives in the domain and core packages; adapters translate between the outside world and those packages through typed port interfaces. Nothing in core or domain knows that Gio, pgx, or the filesystem exists.

## Layer Diagram

```
┌─────────────────────────────────────────────────────────┐
│                      Adapters                           │
│                                                         │
│   ┌──────────────┐          ┌────────────────────────┐  │
│   │  adapter/ui  │          │  adapter/postgres      │  │
│   │  (primary /  │          │  adapter/flyway        │  │
│   │   driving)   │          │  adapter/projectfile   │  │
│   │              │          │  adapter/config        │  │
│   └──────┬───────┘          └───────────┬────────────┘  │
│          │ calls inbound ports          │ implements     │
│          │                              │ outbound ports │
│   ┌──────▼──────────────────────────────▼────────────┐  │
│   │                    port/                         │  │
│   │   (inbound interfaces)   (outbound interfaces)   │  │
│   └──────────────────────┬───────────────────────────┘  │
│                          │                              │
│                   ┌──────▼───────┐                      │
│                   │    core/     │                      │
│                   │  (services)  │                      │
│                   └──────┬───────┘                      │
│                          │                              │
│                   ┌──────▼───────┐                      │
│                   │   domain/    │                      │
│                   │ (pure types) │                      │
│                   └──────────────┘                      │
│                                                         │
│   ┌──────────────┐   (helper, no I/O)                   │
│   │   diagram/   │──────────────────► domain/ only      │
│   └──────────────┘                                      │
│                                                         │
│   cmd/electrorangerd/main.go  ← composition root only  │
└─────────────────────────────────────────────────────────┘
```

## Dependency Rule

Imports flow strictly inward:

```
adapters → port → core → domain
diagram  → domain
cmd      → adapters + core (composition root only)
```

No package may import a sibling adapter package. The `diagram` package imports only `domain`. The `cmd` package is the single permitted place where all concrete types are assembled.

## Cross-Cutting Conventions

### Logging

All structured logging uses `log/slog`. Every constructor that performs I/O or domain work accepts a `*slog.Logger`. Log levels:

- `Debug` — internal state transitions useful during development
- `Info` — significant lifecycle events (startup, connection established)
- `Warn` — recoverable anomalies (schema drift detected, nullable FK found)
- `Error` — operation failures that the caller will also receive as an error return

### Error Handling

Errors are wrapped with `fmt.Errorf("component: %w", err)` at every boundary so that `errors.Is` and `errors.As` work across the call stack. Sentinel errors live in `internal/errs`.

## Future: Multiple SQL Dialects

The current outbound interfaces in `core` (`PostgresIntrospector`, `PostgresApplier`) are PostgreSQL-specific. When a second database engine is needed, these interfaces generalise into a `SQLDialect` abstraction and `adapter/postgres` becomes one concrete implementation alongside (for example) `adapter/mysql`. The inbound port interfaces and all core services remain unchanged.
