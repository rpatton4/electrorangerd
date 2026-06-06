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
│   │   (inbound interfaces ONLY — public API)         │  │
│   └──────────────────────┬───────────────────────────┘  │
│                          │                              │
│                   ┌──────▼───────┐                      │
│                   │    core/     │  declares OUTBOUND   │
│                   │  (services)  │  interfaces at point │
│                   └──────┬───────┘  of use in each file │
│                          │                              │
│                   ┌──────▼───────────────────────────┐  │
│                   │            domain/               │  │
│                   │   Project                        │  │
│                   │   └─ Database [1..N]             │  │
│                   │      └─ Schema [1..N]            │  │
│                   │         └─ Entity                │  │
│                   │            ├─ Attribute          │  │
│                   │            ├─ Index              │  │
│                   │            └─ Constraint         │  │
│                   │   Project.Relationships          │  │
│                   │   (cross-schema, cross-DB)       │  │
│                   │                                  │  │
│                   │   Dictionary  (separate document │  │
│                   │   sibling of Project, keyed by   │  │
│                   │   DictionaryRef)                 │  │
│                   └──────────────────────────────────┘  │
│                                                         │
│   ┌──────────────┐   (helper, no I/O)                   │
│   │   diagram/   │──────────────────► domain/ only      │
│   └──────────────┘                                      │
│                                                         │
│   cmd/electrorangerd/main.go  ← composition root only   │
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

## UI rendering: 2.5D in Gio (decision record)

**Decision**: render the 3D-styled look with 2.5D techniques inside Gio's existing 2D pipeline. No GL bolt-on. No custom 3D engine.

**Context**: user requested entities at angles, angled connectors, drawer-style buttons — "not a game just a UI."

**Alternatives considered**:

1. *Gio + OpenGL ES bolt-on* — embed `gioui.org/example/opengl` pattern for a true 3D canvas. Rejected: 3–6 months solo, ANGLE deployment cost on macOS/Windows, Wayland gap on Linux.
2. *Full custom build* on g3n / raylib-go / raw `go-gl/gl`. Rejected: no Go 3D foundation provides desktop-grade widget machinery; menus / dialogs / file-pickers all re-invented from scratch.
3. *2.5D in Gio* (chosen) — `f32.Affine2D` shear + layered shadows + clipped translate-Y for drawers. Pattern used by Figma, Notion, Linear, Affinity Designer. ~95% of the "feels 3D" impression at minimal cost.

**Mechanism**: axonometric / cabinet projection (parallel projection, not perspective). Entity boxes are sheared at render time in `internal/adapter/ui/canvas/`. Connectors are Bezier paths in sheared space. Drawer buttons are clipped translate-Y animations.

**Gotcha**: Gio rasterizes glyphs upright; sheared text looks broken. Mitigation — un-shear before drawing entity titles; shear only the box chrome.

**Coordinate-space discipline**: `internal/diagram/` is logical 2D only. `internal/adapter/ui/` owns the presentation shear. Mouse interaction inverts the shear before calling `diagram.Hit`.

**When to revisit**: if a 2.5D prototype demonstrably fails to satisfy the user's "3D look" requirement (e.g., perspective foreshortening becomes a hard feature ask), reopen this decision with a fresh architect review before any GL code lands.

## Multi-DB / multi-schema hierarchy (decision record)

**Decision**: a Project models one or more PostgreSQL databases, each holding one or more schema namespaces. `Project ⊃ Database[1..N] ⊃ Schema[1..N] ⊃ Entity`. Relationships live at the Project level and may cross schemas AND databases.

**Context**: target deployment is a microservice architecture where each service typically owns its own database. The user explicitly asked for "multiple schemas in one or more databases."

**Why Project ⊃ Database ⊃ Schema and not Project ⊃ Schema**: even for a single-database project, the explicit `Database` layer is one line of friction (`Databases: [{Name:"primary", ...}]`) compared to a god-painful re-key migration later when a second database is needed. The architect's verdict: "Don't collapse Database. Requirement explicitly says one or more databases."

**Cross-DB relationships**: modelled for documentation only. Flyway cannot enforce foreign keys across databases, so cross-DB relationships are surfaced in the dictionary and the diagram but never emitted as SQL constraints. The forward-engineering plan should warn when it skips a cross-DB relationship.

**Database.ProfileName**: each Database carries a name referencing a vault-encrypted ConnectionProfile (Plan B). The project file never contains plaintext DSNs; profiles resolve through the unlocked vault at runtime.

## Crow's foot cardinality + optionality split (decision record)

**Decision**: each Relationship endpoint carries `Cardinality` (One / Many) and `Optionality` (Required / Optional) as two independent enums, rather than a single four-value `RelationshipKind` enum.

**Why**: three reasons, surfaced by the architect review:

1. **Crow's foot literally draws two glyphs per endpoint** — the inner glyph encodes optionality (circle = optional, bar = required) and the outer glyph encodes cardinality (single line = one, crow's foot = many). The renderer reads each independently.
2. **Forward-engineering cares about each axis separately** — `NOT NULL` (optionality) is one DDL concern; `UNIQUE` (cardinality on the "one" side) is another. Combining them into one enum forces decomposition at every emit site.
3. **Validation rules read one field each** — "missing optionality" and "ambiguous cardinality" are different findings.

A combined four-value enum would force every renderer, validator, and DDL emitter to immediately decompose it. The split keeps the two orthogonal concerns orthogonal.

## Provenance over Inferred bool (decision record)

**Decision**: `Relationship.Provenance` is a struct of `Source` (`Declared` / `Inferred` / `Manual`), `Confidence` (uint8, 0..255), and `Reason` (human-readable text), rather than a simple `Inferred bool`.

**Why**: when reverse-engineering surfaces an implicit FK to the user for review, the UI needs to show *why* the heuristic fired ("column name `customer_id` matches PK pattern; type `bigint` matches target PK type; confidence 0.86"). A boolean is too thin for that conversation. The architect's note: "You'd add this in three weeks anyway — do it now."

**Why not a separate `InferredRelationship` type**: forces parallel handling code at every relationship-aware site (rendering, validation, drift, forward-eng). One Relationship type with a Provenance field keeps the call sites uniform and lets the reverse-engineering review UI filter on `Provenance.Source == SourceInferred`.

**Confidence units**: uint8 chosen for compact storage and because heuristic engines tend to emit normalized 0..1 scores that scale cleanly to 0..255. The exact scoring scheme is reverse-engineering's concern (Plan C / future).
