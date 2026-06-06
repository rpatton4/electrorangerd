# electrorangerd

A desktop ERD (Entity-Relationship Diagram) tool for PostgreSQL schemas, built with Go and Gio.

## Build and Run

```bash
go build ./...
go run ./cmd/electrorangerd
```

Requires Go 1.26+ and a C compiler (for Gio's native windowing layer on macOS/Linux).

## Documentation

See [docs/architecture.md](docs/architecture.md) for the hexagonal architecture overview, dependency rules, and package map.

## Package Map

| Path | Role |
|------|------|
| `internal/domain` | Pure value types — Schema, Entity, Attribute, Relationship |
| `internal/port` | Inbound port interfaces consumed by the UI adapter |
| `internal/core` | Domain services: forward/reverse engineering, drift detection, project I/O |
| `internal/diagram` | Pure canvas layout and hit-testing — no Gio dependency |
| `internal/adapter/ui` | Primary adapter: GIOUI desktop window and event loop |
| `internal/adapter/postgres` | Outbound adapter: pgx-backed introspector and DDL applier |
| `internal/adapter/flyway` | Outbound adapter: Flyway-style migration file reader/writer |
| `cmd/electrorangerd` | Composition root — wires adapters to core and starts the UI |
