# ElectroRangerD — Project Guidance for Claude

## What this is

A Go desktop tool for creating and managing Entity Relationship Diagrams (ERDs). Single-developer, private GitHub repo. Reads/writes PostgreSQL via pgx (forward/reverse engineering), reads/writes Flyway migration files, and detects drift between any two schema snapshots (project file ↔ live PG ↔ Flyway replay). UI is GIOUI (immediate-mode desktop).

- **Module**: `github.com/InfiniteSkye/electrorangerd`
- **Repo root**: `/Users/rpatton/work/repositories/erd_workroot/main/` (worktree-style — branch name matches directory)
- **`go.work`**: lives at `/Users/rpatton/work/repositories/erd_workroot/go.work` for IDE indexing — **DO NOT move it inside `main/`**
- **Go**: 1.26.3
- **Key deps**: `gioui.org` v0.10.0, `github.com/jackc/pgx/v5` v5.9.2
- **License**: Proprietary / All Rights Reserved (InfiniteSkye)

## Application modes and scope

ElectroRangerD operates in four primary modes, presented as a strict mode switch with a peek panel from any mode (the peek panel can show a slice of another mode for the currently-selected schema element, but the full mode UI never overlaps).

| # | Mode | Purpose | Implementation plan |
|---|------|---------|---------------------|
| 1 | Diagram edit | Create and maintain ER diagrams from scratch, no reverse-engineering | Plan C |
| 2 | Forward engineering | Push diagrams into PostgreSQL databases (multi-schema, multi-DB via Flyway) | Plan C |
| 3 | Data dictionary | Create and maintain dictionary information for each entity, attribute, and relationship | Plan C |
| 4 | Reverse engineering | Build an ER diagram from an existing data source with multiple schemas | Plan C |

### Data model hierarchy

```
Project
└── Database [1..N]            // ProfileName references a vault-encrypted connection (Plan B)
    └── Schema [1..N]          // PostgreSQL schema namespace
        └── Entity
            ├── Attribute
            ├── Index
            └── Constraint
Project.Relationships [...]     // project-level; can cross schemas AND databases
Dictionary (separate document)  // sibling of Project, keyed by DictionaryRef
```

### Three-plan sequence

The scope expansion is delivered as three sequenced plans, NOT one mega-plan:

- **Plan A (foundation, complete)** — domain reshape, port signature updates, doc lock-ins.
- **Plan B (vault, complete)** — `Vault` inbound port + `adapter/vault` with Argon2id KDF + AES-GCM encryption + DEK/KEK wrapping; build-tagged OS keychain integration via `github.com/zalando/go-keyring` (macOS Keychain / Windows Credential Manager / Linux Secret Service). On-disk blob is `<UserConfigDir>/electrorangerd/vault.json` (atomic-rename writes, 0600 perms). `domain.ConnectionProfile.EncryptedDSN` replaces plaintext DSN; profiles are stored inside the vault blob, NOT in `domain.Config`.
- **Plan C (UI shell, next)** — per-mode UI in `adapter/ui/`: 4-mode state machine, peek panel, mode-local `History` stack, markdown rendering via `gioui.org/x/markdown`, 2.5D canvas, drawer-style buttons, master-password prompt at launch (with opt-in keychain auto-unlock).

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
| Project JSON shape | `internal/adapter/projectfile/` consumes `domain.Project` | |
| User pref (window state, recent files) | `internal/domain/config.go` types + `internal/adapter/config/` IO | |
| Connection profile (encrypted DSN) | `internal/domain/vault.go` types + `internal/adapter/vault/` IO; access via `port.Vault` only when unlocked | Plaintext DSNs MUST NOT touch disk |
| Composition wiring | `cmd/electrorangerd/main.go` only | Only place allowed to import concrete adapters + core simultaneously |

## Conventions

- **Logging**: stdlib `log/slog`, injected as `*slog.Logger`. Never global. Never package-level. Never `slog.Default()` outside `main()`.
- **Errors**: wrap with `fmt.Errorf("<context>: %w", errs.ErrXxx)`. Match with `errors.Is`.
- **`domain.Project` discipline**: NO mutating methods on domain types. Services produce *new* Projects; never mutate in place. Value types over pointer fields. This is the architect's loudest warning — Project (the top-level container, multi-DB + multi-schema + relationships + dictionary refs) becomes a god-struct if we let it.
- **Comments**: default to none. Add only when WHY is non-obvious (hidden constraint, subtle invariant, workaround). Names explain WHAT.
- **No `util/`, `common/`, `helpers/` packages.** Reaching for one means avoiding a real domain decision.
- **No package-level mutable state.** No `var Global = ...`.
- **No backwards-compat shims.** Private code — break it, fix it, move forward.
- **Tests** land per-package with real fixtures (`testdata/`) when the corresponding logic is implemented. Scaffolding currently has none.
- **Test policy**: unit tests are written alongside or prior to logic, and **only** test true user-facing functionality — there are NO tests for the sake of coverage metrics. Tests live next to code as `*_test.go` files, are table-driven, and exercise inbound ports (the user-facing API surface). NO central `internal/integration/` package — that becomes a dumping ground and obscures ownership.
- **Multithreading**: `core/` services own their goroutines and decide internally whether to fan out (e.g. reverse-engineering parallelizes per-schema introspection). The UI dispatches work via channels and marshals results back via `op.InvalidateOp`. `context.Context` propagates UI → services for cancellation. **NO goroutines in `domain/` or `port/`.**
- **Markdown**: in-app documentation is authored in Markdown and rendered via `gioui.org/x/markdown` inside `adapter/ui/`. There is NO `MarkdownRenderer` port — only the UI consumes markdown rendering. Domain text fields like `DictionaryEntry.Description` are plain `string` with a markdown content convention.
- **Flyway constraint**: the free Community Edition is the only target; there is NO `undo` command. The tool emits ONLY `V_` (versioned) and `R_` (repeatable) migration files. "Undoing" a change means emitting a forward "compensation" migration. `MigrationFile.Down` is reserved for our own compensation-migration emission and is never executed by Flyway.
- **Notation**: crow's foot ONLY. Each Relationship endpoint carries an independent `Cardinality` (One / Many) and `Optionality` (Required / Optional), encoded as two separate enums so renderers and forward-engineering can read them independently (forward-eng cares about NOT NULL independently from UNIQUE).
- **Provenance over Inferred bool**: `Relationship.Provenance` records `Source` (Declared / Inferred / Manual), `Confidence` (0..255), and a human-readable `Reason`. Reverse-engineering surfaces the reason in the review UI so users know why a heuristic fired.

## UI rendering approach: 2.5D in Gio

ElectroRangerD renders with a 3D-styled look — entities at angles, angled connectors, drawer-style buttons — using **2.5D techniques in Gio's existing 2D pipeline**. No OpenGL bolt-on. No custom 3D engine.

Mechanism (Figma / Notion / Linear pattern):

- **Axonometric / cabinet projection** for entity boxes via `f32.Affine2D` shear. Parallel projection only — no perspective, no z-buffer.
- **Layered shadows** under entity boxes for depth cueing.
- **Sheared Bezier paths** for angled connectors between entities.
- **Clipped translate-Y animations** for drawer-style buttons. No 3D involved in drawers.

### Hard rule: keep title text upright

Gio rasterizes glyphs upright; text rendered through a sheared transform looks like cal up close. **Un-shear before drawing entity titles** — only the box chrome carries the shear. Figma uses exactly this pattern.

### Coordinate-space discipline

- `internal/diagram/` holds **logical 2D coordinates** of ERD entities and edges. NEVER apply axonometric shear here.
- `internal/adapter/ui/` applies the shear at render time. The shear matrix and its inverse (for hit-testing) live with the canvas code in `adapter/ui/canvas/`.
- Mouse coordinates flow: pointer event → inverse-shear → logical 2D space → `diagram.Hit(...)` returns the entity name.

### Rejected alternatives (do not reopen without an architect review)

- **Gio + OpenGL ES bolt-on** for a true 3D canvas: 3–6 months solo work, ANGLE deployment headache on macOS/Windows, Wayland gap on Linux. Not justified for an ERD tool.
- **Full custom build (g3n / raylib-go / raw OpenGL)**: no Go 3D foundation gives a desktop-grade ERD app faster than Gio gives a desktop-grade 2D one. UI widgets get re-invented from scratch.

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

- **Per-mode UI** — Plan C: the four-mode state machine, peek panel, mode-local History stacks, master-password prompt UI, markdown rendering, drawer animations, the 2.5D canvas itself.
- **Telemetry beyond `slog`**
- **Backwards-compat layers / migration shims**
- **CI workflows**
- **Packaging** (.app, .exe, signing)
- **Second SQL dialect**
- **`adapter/dictionaryfile`** — separate on-disk dictionary persistence lands when real save/load logic does.
- **Implicit FK heuristic engine** — reverse-engineering logic, not the type model (the Provenance type that records inferred FKs IS in Plan A).
- **Crow's foot rendering glyphs** — the cardinality/optionality types are in Plan A; the visual rendering lands with the canvas in Plan C.

## Current state

Plans A (foundation reshape) and B (vault) are complete. Every service method body still returns `errs.ErrNotImplemented` except the Vault service, which is fully functional (Initialize / Unlock / Lock / ChangePassword / profile CRUD / OS keychain integration). The UI is still a placeholder Gio window. Plan C (per-mode UI shell) is the next plan.
