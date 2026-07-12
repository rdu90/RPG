# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`RPG` (module `github.com/rdu90/RPG`) is a TradeWars-inspired, terminal-only galaxy trading game, built as the first citizen of a future multi-genre game platform. The repo name is the project/platform identity, not a genre label. See `README.md` for the game pitch (Explore/Expand/Exploit/Exterminate, node-graph galaxy, haggling-driven economy) and `PLAN.md` for the full build plan, architecture rationale, and milestone roadmap (M0–M10) — read `PLAN.md` before making structural changes; it is the source of truth for design intent.

The codebase is currently at milestone **M0** (foundations only): the engine/transport/persistence skeleton and a bubbletea shell exist, but there is no gameplay yet (`internal/engine/galaxy`, `economy`, `haggle`, `fleet`, `colony`, `techtree`, `espionage`, `combat`, `player`, `turn` are all empty packages reserved for later milestones). `cmd/rpg-server` is likewise an empty placeholder for M9 (async multiplayer).

## Commands

```
make build         # build bin/rpg
make run           # build and run
make test          # go test ./...
make test-verbose  # go test -v ./...
make cover         # coverage report (coverage.out)
make fmt           # fail if gofmt would change anything
make fmt-write     # gofmt -w .
make vet           # go vet ./...
make lint          # golangci-lint (pinned via go.mod tool directive)
make tidy          # go mod tidy
make check         # fmt + vet + lint + test — run before considering work done
make release       # cross-compile dist/ binaries for linux/darwin amd64/arm64
```

Run a single test: `go test ./internal/tui/app/... -run TestName -v`

`golangci-lint` is invoked via `go tool` (declared in `go.mod`'s `tool` directive), not a separately installed binary — no `.golangci.yml` exists yet, so it runs with defaults.

## Architecture

**Strict dependency direction:** `tui → transport → engine → ports ← persistence`. `cmd/*` is the only place concrete implementations are wired together (composition root). This is enforced by convention, not tooling — respect it when adding code.

- **`internal/engine`** — pure game-logic core. Zero I/O, zero UI, zero SQL. The only entry points are `Engine.Execute(ctx, Command)` and `Engine.Query(ctx, Query)` (see `internal/engine/engine.go`), a facade that type-switches on Command/Query values and dispatches to the relevant subsystem repository. New gameplay commands/queries are added as small types implementing `isCommand()`/`isQuery()` (see `commands.go`, `queries.go`) and a case in the switch — this boundary is deliberately kept thin so a future network transport can wrap the exact same values unchanged.
- **`internal/engine/ports`** — repository interfaces the engine depends on (e.g. `GameRepository`). Persistence implements these; the engine package never imports persistence. This inversion is what lets storage be swapped/scaled later without touching game logic.
- **`internal/persistence/sqlite`** — the `ports.*` implementation, using `modernc.org/sqlite` (pure Go, no CGO) via `database/sql`. `sqlite.Open` applies pending goose migrations on open.
- **`internal/persistence/migrations`** — versioned `.sql` files (goose format), embedded via `go:embed` so the binary carries its own schema. Every schema change is a new numbered migration file here, never an edit to an applied one.
- **`internal/transport/command`, `internal/transport/query`** — re-export the engine's Command/Query/result vocabulary under a transport-facing import path (currently type aliases). Code above the transport boundary (the TUI) imports these, never `internal/engine` directly, so a remote transport can later replace `transport/local` without caller changes.
- **`internal/transport/local`** — the current in-process transport: `local.Client` wraps an `*engine.Engine` directly, no serialization. A future `transport/grpc` would implement the identical `Execute/Query` shape against a remote engine. Proving this swap requires zero changes to `internal/engine` or `internal/tui` is an explicit milestone gate (M9).
- **`internal/tui`** — bubbletea (Elm-architecture Init/Update/View) + lipgloss + bubbles. `app/` is the root model/program; `screens/` and `components/` (currently empty, reserved for upcoming milestones) will hold per-feature screens and shared widgets; `style/` holds the shared theme. The TUI only ever talks through `internal/transport`, never reaches into `internal/engine` directly.
- **`internal/config`** — resolves the save directory (honors `$XDG_DATA_HOME`, defaults to `~/.local/share/rpg`) and save file paths; each save is one SQLite file, one `Game` row.
- **`internal/rng`** — reserved for seeded deterministic RNG (galaxy gen, combat, haggle) — not yet populated.
- **`cmd/rpg`** — single-player TUI entrypoint; the composition root that wires `config` → `sqlite.Store` → `engine.New` → `local.New` → `tui/app.New` together, including save open/switch and save listing closures passed into the TUI.
- **`cmd/rpg-server`** — placeholder for the M9 async-multiplayer server entrypoint.

**Turn/tick model (load-bearing design decision, see PLAN.md for full rationale):** turn allowances and the galaxy tick are computed lazily from timestamps on read ("advance N ticks since last seen"), not driven by a background scheduler. This makes the exact same resolution code work for both a local single-player SQLite file and a future shared server, so multiplayer becomes a transport + auth addition later rather than a concurrency-model rewrite.

**When adding gameplay to an empty `internal/engine/*` subpackage:** keep it pure domain logic with no I/O; add any new persistence needs as a `ports` interface method, implement it in `internal/persistence/sqlite`, add a migration, add Command/Query types in `engine/commands.go`/`queries.go` plus their transport re-exports, and wire a new TUI screen under `tui/screens` that talks only through `transport`.
