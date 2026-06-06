# ElectroRangerD — Project Guidance for Claude

## What this is

A Go desktop tool for creating and managing Entity Relationship Diagrams (ERDs). Single-developer, private GitHub repo. Reads/writes PostgreSQL via pgx (forward/reverse engineering), reads/writes Flyway migration files, and detects drift between any two schema snapshots (project file ↔ live PG ↔ Flyway replay). UI is GIOUI (immediate-mode desktop).

- **Module**: `github.com/InfiniteSkye/electrorangerd`
- **Repo root**: `/Users/rpatton/work/repositories/erd_workroot/main/` (worktree-style — branch name matches directory)
- **`go.work`**: lives at `/Users/rpatton/work/repositories/erd_workroot/go.work` for IDE indexing — **DO NOT move it inside `main/`**
- **Go**: 1.26.3
- **Key deps**: `gioui.org` v0.10.0, `github.com/jackc/pgx/v5` v5.9.2
- **License**: Proprietary / All Rights Reserved (InfiniteSkye)

## Architecture: hexagonal (ports & adapters)

```
adapter/  ──►  core/  ──►  port/
                  │
                  ▼
              domain/   (errs/ is a peer of domain, also no internal deps)
```

### Dependency rule — STRICTLY ENFORCED

- `domain/` imports **nothing internal** (pure types, value semantics, no mutating methods)
- `errs/` imports **nothing internal** (sentinel errors, peer of domain)
- `port/` imports **only `domain/`** (driving / inbound interfaces — the app's public API)
- `core/` imports **`domain/` + `port/` + `errs/`** (services + outbound interfaces declared at point of use)
- `adapter/*` imports **`domain/` + `core/` + `errs/` + external libs** (concrete implementations)
- `adapter/ui/` additionally imports **`port/`** (it consumes inbound ports as the primary / driving adapter)
- `cmd/electrorangerd/main.go` is the **only** file that touches both `core/` and concrete adapter packages

### Go idiom choices, locked in

- **Inbound port interfaces** (what the app offers) live in `internal/port/inbound.go` — they ARE the app's public API and every primary adapter touches them.
- **Outbound port interfaces** (what the app needs from outside) live **at the top of the `internal/core/<service>.go` file that consumes them** (Go's "define interfaces at point of use" — the gRPC-Go pattern). Adapters import `core` for these types. **DO NOT centralize outbound interfaces in a single `port/outbound.go` file.**
- **Service structs are lowercase**, exposed via `NewXxx(...)` constructors returning the matching `port.<Interface>`. E.g., `core.NewForwardService(...) port.ForwardEngineer`.

## Where does X go? Decision tree

| What you're adding | Where it goes | Why |
|---|---|---|
| New entity / value type | `internal/domain/<file>.go` | Pure types only, no mutating methods |
| New sentinel error | `internal/errs/errs.go` | Wrap with `fmt.Errorf("ctx: %w", errs.ErrXxx)` |
| New use case the UI will call | New interface in `internal/port/inbound.go` + new service in `internal/core/` | The inbound API expands |
| New driven dependency a service needs | Declare interface AT THE TOP of the service's `core/` file, implement in new `internal/adapter/<name>/` | Point-of-use; adapter satisfies the interface |
| New PG introspection / DDL logic | `internal/adapter/postgres/` | One package until ~600 lines/file or second dialect lands |
| New Flyway file IO | `internal/adapter/flyway/` | |
| Pure canvas math (layout, hit-testing, edge routing) | `internal/diagram/` | Must NOT import Gio — keeps it unit-testable |
| Anything Gio-specific (rendering, events, widgets) | `internal/adapter/ui/` | Flat until ~8 files, then split by concern (`canvas/`, `panel/`, `dialog/`) — NEVER by Gio primitive |
| Logging | Inject `*slog.Logger` via constructor — NEVER global, NEVER package-level | |
| Project JSON shape | `internal/adapter/projectfile/` consumes `domain.Schema` | |
| User pref / connection profile | `internal/domain/config.go` types + `internal/adapter/config/` IO | |
| Composition wiring | `cmd/electrorangerd/main.go` only | Only place allowed to import concrete adapters + core simultaneously |

## Conventions

- **Logging**: stdlib `log/slog`, injected as `*slog.Logger`. Never global. Never package-level. Never `slog.Default()` outside `main()`.
- **Errors**: wrap with `fmt.Errorf("<context>: %w", errs.ErrXxx)`. Match with `errors.Is`.
- **`domain.Schema` discipline**: NO mutating methods on domain types. Services produce *new* schemas; never mutate in place. Value types over pointer fields. This is the architect's loudest warning — Schema becomes a god-struct if we let it.
- **Comments**: default to none. Add only when WHY is non-obvious (hidden constraint, subtle invariant, workaround). Names explain WHAT.
- **No `util/`, `common/`, `helpers/` packages.** Reaching for one means avoiding a real domain decision.
- **No package-level mutable state.** No `var Global = ...`.
- **No backwards-compat shims.** Private code — break it, fix it, move forward.
- **Tests** land per-package with real fixtures (`testdata/`) when the corresponding logic is implemented. Scaffolding currently has none.

## Boundary verification

Run these from `main/` after any structural change. All four must return empty:

```sh
grep -RE "internal/adapter" internal/domain internal/port internal/core internal/errs
grep -RE "internal/" internal/domain | grep -v "internal/domain"
grep -RE "internal/" internal/errs | grep -v "internal/errs"
grep -RE "internal/port" internal/adapter | grep -v "internal/adapter/ui"
```

Plus the usual: `go build ./...`, `go vet ./...`, `go test ./...` from `main/`.

## SQL dialect future-note

`core.PostgresIntrospector` and `core.PostgresApplier` are PG-specific. When a second DB lands, they generalize into a `SQLDialect` abstraction declared in `core/` (still point-of-use), and `adapter/postgres/` becomes one of multiple dialect adapters. Don't preemptively build this — note the seam when adding PG-only behavior.

## Out of scope (don't bolt on without asking)

- Telemetry beyond `slog`
- Backwards-compat layers / migration shims
- CI workflows
- Packaging (.app, .exe, signing)
- Second SQL dialect

## Current state

All scaffolding committed at root commit `04ee086` ("Scaffold ElectroRangerD with hexagonal architecture"). Every service method body returns `errs.ErrNotImplemented`. No business logic anywhere yet. Filling in real logic is per-package and needs its own plan.
