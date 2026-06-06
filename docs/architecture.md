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
