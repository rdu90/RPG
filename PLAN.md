# RPG — Build Plan (v1 game: a TradeWars-inspired galaxy trader)

## Context

The repo is named **RPG** (`github.com/rdu90/RPG`) because it's meant to grow into a platform for multiple game genres, not just this one game — "RPG" is the project/platform's name, not a genre label. Today the repo is a blank slate: one commit, a `README.md` containing the design pitch for the first game (Explore/Expand/Exploit/Exterminate, node-graph galaxy, low/no-graphics interface, haggling-driven trade economy). The goal is to build that first game — a real, playable TradeWars-style trader — **and** lay groundwork so later, unrelated game genres can reuse the underlying engine as a platform, without paying the cost of building that generic platform today. All naming below (module path, binaries, save paths) uses `rpg` as the project identity; the TradeWars-style game itself doesn't need its own product name yet since it's the first citizen of the RPG platform, built incrementally.

Decisions locked in with the user before design:

- **Language:** Go.
- **Interface:** terminal TUI first; engine decoupled from UI so a web client can be added later without touching game logic.
- **Multiplayer:** single-player only for v1 gameplay, but the turn/persistence model must be async-multiplayer-ready from day one (classic TradeWars BBS door-game shape — shared persistent galaxy, turn-limited actions, no real-time netcode needed) so multiplayer is additive later, not a rewrite.
- **Platform strategy:** build the actual game first as a complete vertical slice; extract a generic multi-genre platform layer only after this game is done and we know what's genuinely reusable. Keep clean internal package boundaries now so that extraction is a refactor, not a rewrite — do not build a generic ECS/plugin framework up front.

## Architecture

**Turn/time model:** Each player has a `TurnAllowance` (remaining/max/refill-rate) computed lazily from timestamps on read — no background scheduler needed. Most actions (move a ship, haggle a round, launch a scout, start combat) cost turns. A separate, coarser **galaxy tick** (background market drift, NPC movement, colony growth, research accrual) is also computed lazily as "advance N ticks since last seen," fully idempotent. This is the load-bearing decision for async-multiplayer-readiness: because nothing depends on a live running tick-loop, the exact same resolution code works whether it's backing one local SQLite file or a shared server — multiplayer becomes a transport + auth addition later, not a concurrency-model rewrite.

**Persistence:** Embedded SQLite via `modernc.org/sqlite` (pure Go, no CGO — simpler cross-compilation/CI), accessed through `database/sql`, with versioned migrations (`pressly/goose` or `golang-migrate`) from the very first schema. Single-player save = one SQLite file (e.g. `~/.local/share/rpg/<save>.db`). The engine defines repository interfaces (`internal/engine/ports`); the persistence package implements them — the engine never imports persistence. This dependency-inversion seam is what makes swapping/scaling storage later (WAL-mode concurrent server, or a Postgres swap) a persistence-layer change invisible to game logic. Use `sqlc` for typed SQL codegen rather than hand-rolled boilerplate or an ORM.

**Package layout & dependency direction** (strict: `tui → transport → engine → ports ← persistence`; `cmd/*` is the only place that wires concrete implementations together):

```
cmd/
  rpg/                   # TUI single-player entrypoint (composition root)
  rpg-server/            # added in M9: async-multiplayer server entrypoint
internal/
  engine/                # pure domain logic — zero I/O, zero UI, zero SQL
    galaxy/               # sectors, star systems, node graph, coords, warp lanes, fog of war
    economy/              # commodities, markets, pricing/supply-demand
    haggle/               # negotiation state machine
    fleet/                # ships, movement, cargo
    colony/               # planets, colonization, population, buildings
    techtree/             # tech nodes, prerequisites, research (data-driven content)
    espionage/            # spies, missions, detection
    combat/               # fleet battles, bombardment, invasion resolution
    player/               # player/faction state, credits, reputation, alignment
    turn/                 # turn allowance + galaxy tick math
    ports/                # repository interfaces persistence implements
    engine.go             # facade: Execute(Command) / Query(Query) — the transport boundary
  transport/
    command/ query/       # serializable Command/Query DTOs
    local/                 # in-process adapter: TUI -> local.Client -> engine.Engine
  persistence/
    sqlite/                # ports.* implementations
    migrations/             # embedded .sql migration files
  rng/                     # seeded deterministic RNG (galaxy gen, combat, haggle)
  tui/
    app/ screens/ components/ style/   # bubbletea root, per-feature screens, shared widgets, theme
  config/                  # config/save-path resolution
docs/adr/                  # architecture decision records (start with turn model + persistence choice)
```

The critical rule: `tui` only ever talks to `transport` (never `internal/engine` directly). The TUI turns keypresses into `command.X{}`/`query.Y{}` values and calls `transport/local.Client`. When multiplayer arrives, a sibling `transport/grpc` implements the same `Client` interface and the TUI code doesn't change.

**TUI stack:** `bubbletea` (Elm-architecture: `Init/Update/View`) + `lipgloss` (styling/layout, adaptive light/dark colors) + `bubbles` (`table`, `textinput`, `viewport`, `list`, `progress`). Verify current module paths/versions at implementation time (Charm ecosystem has moved to v2 modules). The galaxy map itself is a custom component (sparse coordinate graph, not a grid) rendered inside a `bubbles/viewport` for pan/scroll, glyphs at scaled `(x,y)` positions, simple line/dot warp-lane connectors, cursor-based node selection.

**Domain model highlights:**

- Galaxy is an explicit sparse graph (`Node`, `Warp` edges with turn costs), not a dense grid — matches TradeWars' sector-links model. Star systems have a `DevelopmentLevel` driving what they produce/need.
- Commodities have a fixed `Category` (Normal / Illegal / Exotic / Immoral). **Alignment is a derived 2D vector** (`Legality`, `Morality`), computed as a weighted moving average over the player's trade ledger — not a directly-set field. This is the mechanical answer to "what you trade is your alignment."
- **Haggling** is a multi-round negotiation state machine: NPC opening offer derived from market price + an `NPCDisposition` (patience/greed/trust, itself derived from system development level and per-system player `Reputation`); each round the player offers, walks away (bluff), or accepts; walking away can succeed (better counter) or fail (reputation hit). Deliberately single-axis (price only) — no multi-item baskets or side-favors in v1.
- Tech tree is **data-driven** (YAML/JSON content, not hardcoded Go) specifically to keep "sprawling tree" a content task, not an open-ended engineering one.
- Espionage: a handful of mission types resolved as a single probability check (spy `Skill` vs target `CounterIntel`) — not a multi-step minigame.
- Combat/bombardment/invasion: deterministic, formula + seeded-RNG resolution over discrete rounds, presented as a result log — not a manual tactical-positioning engine.

## Roadmap

| # | Milestone | Playable outcome |
| --- | --- | --- |
| M0 | Foundations | `go.mod`, engine `Execute/Query` facade, `ports` interfaces, SQLite skeleton + first migration, `transport/local`, bubbletea shell that boots to a menu. No gameplay yet. |
| M1 | MVP core loop: fly & trade | Generate a small seeded galaxy, fly between nodes spending turns, buy/sell at static per-system prices, save/resume works. This is the "is it fun" gate. |
| M2 | Haggling | Static prices replaced by the negotiation state machine; per-system reputation and derived alignment come online. |
| M3 | Explore | Scouts, fog of war, discoverable anomaly "secrets." |
| M4 | Expand | Planet colonization, population/production growth per galaxy tick feeding back into markets. |
| M5 | Exploit: tech tree | Data-driven tech content (~20-30 nodes), research accrual, concrete effects on economy/fleet. |
| M6 | Exploit: espionage | Spy recruitment, narrow mission set, probabilistic outcomes. |
| M7 | Exterminate: fleet combat | Deterministic PvE battle resolution against hostiles encountered while traveling. |
| M8 | Exterminate: bombardment & invasion | Extend combat to planetary assault and rival human factions; ownership can change hands. |
| M9 | Async multiplayer enablement | `cmd/rpg-server`, token auth, `transport/grpc` sibling to `transport/local`, SQLite WAL (or Postgres if needed) — proves the "no engine rewrite" bet from the turn-model design. |
| M10 | Platform extraction | Only after the game is complete: pull proven-generic pieces (galaxy graph engine, market-sim primitives, turn/tick model, persistence scaffolding, TUI shell/components) into a separate platform layer for the next game. |

## Scope risks to actively cut against

1. **Tech tree content is unbounded scope** — ship M5 with a small curated set (~20-30 nodes) as data; treat "more nodes" as post-launch backlog, never a reason to delay M6+.
2. **Espionage depth** — resist persistent spy networks or detection minigames; one mission set, one probability check, revisit only after the core loop is proven fun.
3. **Combat tactical depth** — resist building a turn-based tactics-grid engine; keep it formula/log-resolved through M8, treat manual tactical UI as an explicit out-of-scope stretch goal.
4. Don't start networking/auth work before M9, and don't build a generic plugin/ECS framework before M10 — the package boundaries above already buy that optionality without paying for it now.
5. Adopt versioned SQL migrations starting at M0, even though it's single-player-only through M8 — retrofitting migration discipline after 8 milestones of organic schema growth is much more painful than starting with it.

## Critical first files (create in this order)

- `go.mod` — module init (module path `github.com/rdu90/RPG`, matching the `origin` remote)
- `internal/engine/engine.go` — the `Execute(Command)/Query(Query)` facade; get this boundary right first, everything else depends on it
- `internal/engine/ports/ports.go` — repository interfaces persistence must satisfy
- `internal/persistence/sqlite/sqlite.go` + `internal/persistence/migrations/0001_init.sql` — first schema and repo implementation
- `cmd/rpg/main.go` — composition root wiring persistence, engine, `transport/local`, and `tui/app` together

## Verification

- M0: `go build ./...` succeeds; running the binary opens/creates a save DB and shows a menu.
- M1 (the real gate): play through the loop manually — start a new game, pan/select nodes on the galaxy map, fly several hops, observe price differences between systems, execute buy/sell trades, quit and relaunch to confirm the save round-trips correctly via the SQLite file.
- Each subsequent milestone: add engine-level unit tests for its package (e.g. `engine/haggle`, `engine/combat` resolution is pure and deterministic given a seed, so it's straightforward to test), then manually drive the corresponding new TUI screen end-to-end before considering the milestone done.
- Before M9, confirm the "no rewrite" bet directly: write `transport/grpc` against the exact same `Client` interface `transport/local` implements, with no changes required in `internal/engine` or `internal/tui`.
