package sqlite

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rdu90/RPG/internal/engine/colony"
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/espionage"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/techtree"
	"github.com/rdu90/RPG/internal/engine/turn"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestGalaxyRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	g := galaxy.Galaxy{
		Nodes: []galaxy.Node{
			{ID: "sys-000", Name: "Aldrin", X: 1, Y: 2, DevelopmentLevel: 3},
			{ID: "sys-001", Name: "Vega", X: 4, Y: 5, DevelopmentLevel: 1},
		},
		Edges: []galaxy.Edge{
			{From: "sys-000", To: "sys-001", TurnCost: 2},
		},
	}

	if err := s.SaveGalaxy(ctx, g); err != nil {
		t.Fatalf("save galaxy: %v", err)
	}

	got, err := s.GetGalaxy(ctx)
	if err != nil {
		t.Fatalf("get galaxy: %v", err)
	}
	if !reflect.DeepEqual(got.Nodes, g.Nodes) {
		t.Fatalf("nodes mismatch: got %+v, want %+v", got.Nodes, g.Nodes)
	}
	if !reflect.DeepEqual(got.Edges, g.Edges) {
		t.Fatalf("edges mismatch: got %+v, want %+v", got.Edges, g.Edges)
	}
}

func TestMarketRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	prices := []economy.Price{
		{CommodityID: "food", Price: 12},
		{CommodityID: "weapons", Price: 120},
	}
	if err := s.SaveMarket(ctx, "sys-000", prices); err != nil {
		t.Fatalf("save market: %v", err)
	}

	got, err := s.GetMarket(ctx, "sys-000")
	if err != nil {
		t.Fatalf("get market: %v", err)
	}
	if !reflect.DeepEqual(got, prices) {
		t.Fatalf("market mismatch: got %+v, want %+v", got, prices)
	}

	empty, err := s.GetMarket(ctx, "sys-999")
	if err != nil {
		t.Fatalf("get market for unknown node: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected no prices for unknown node, got %+v", empty)
	}

	// SaveMarket must fully replace an existing market (delete then
	// reinsert), not error out on the primary key it already holds — this
	// is the path a colony's production-driven price decay repeatedly
	// writes through on later galaxy ticks.
	updated := []economy.Price{
		{CommodityID: "food", Price: 9},
		{CommodityID: "weapons", Price: 200},
	}
	if err := s.SaveMarket(ctx, "sys-000", updated); err != nil {
		t.Fatalf("re-save market: %v", err)
	}
	got, err = s.GetMarket(ctx, "sys-000")
	if err != nil {
		t.Fatalf("get market after re-save: %v", err)
	}
	if !reflect.DeepEqual(got, updated) {
		t.Fatalf("market mismatch after re-save: got %+v, want %+v", got, updated)
	}
}

func TestPlayerRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	p := player.Player{
		Credits:          500,
		NodeID:           "sys-000",
		CargoCapacity:    40,
		Cargo:            map[economy.CommodityID]int{"food": 5, "weapons": 2},
		Turns:            turn.New(100, 20*time.Second, now),
		Reputation:       map[galaxy.NodeID]int{"sys-000": 5, "sys-001": -3},
		Alignment:        player.Alignment{Legality: 0.4, Morality: -0.2},
		Discovered:       map[galaxy.NodeID]bool{"sys-000": true, "sys-001": true},
		ClaimedAnomalies: map[galaxy.NodeID]bool{"sys-000": true},
	}

	if err := s.InitPlayer(ctx, p); err != nil {
		t.Fatalf("init player: %v", err)
	}

	got, err := s.GetPlayer(ctx)
	if err != nil {
		t.Fatalf("get player: %v", err)
	}
	if got.Credits != p.Credits || got.NodeID != p.NodeID || got.CargoCapacity != p.CargoCapacity {
		t.Fatalf("player scalar fields mismatch: got %+v, want %+v", got, p)
	}
	if !reflect.DeepEqual(got.Cargo, p.Cargo) {
		t.Fatalf("cargo mismatch: got %+v, want %+v", got.Cargo, p.Cargo)
	}
	if got.Turns.Max != p.Turns.Max || got.Turns.Remaining != p.Turns.Remaining ||
		got.Turns.RefillEvery != p.Turns.RefillEvery || !got.Turns.LastRefillAt.Equal(p.Turns.LastRefillAt) {
		t.Fatalf("turns mismatch: got %+v, want %+v", got.Turns, p.Turns)
	}
	if !reflect.DeepEqual(got.Reputation, p.Reputation) {
		t.Fatalf("reputation mismatch: got %+v, want %+v", got.Reputation, p.Reputation)
	}
	if got.Alignment != p.Alignment {
		t.Fatalf("alignment mismatch: got %+v, want %+v", got.Alignment, p.Alignment)
	}
	if !reflect.DeepEqual(got.Discovered, p.Discovered) {
		t.Fatalf("discovered mismatch: got %+v, want %+v", got.Discovered, p.Discovered)
	}
	if !reflect.DeepEqual(got.ClaimedAnomalies, p.ClaimedAnomalies) {
		t.Fatalf("claimed anomalies mismatch: got %+v, want %+v", got.ClaimedAnomalies, p.ClaimedAnomalies)
	}

	// SavePlayer must overwrite scalar fields and fully replace cargo,
	// reputation, discovered, and claimed anomalies (including removing
	// entries, not just upserting them).
	p.Credits = 250
	p.NodeID = "sys-001"
	p.Cargo = map[economy.CommodityID]int{"food": 1}
	p.Reputation = map[galaxy.NodeID]int{"sys-001": 10}
	p.Alignment = player.Alignment{Legality: -0.6, Morality: 0.1}
	p.Discovered = map[galaxy.NodeID]bool{"sys-001": true}
	p.ClaimedAnomalies = map[galaxy.NodeID]bool{}
	if err := s.SavePlayer(ctx, p); err != nil {
		t.Fatalf("save player: %v", err)
	}

	got, err = s.GetPlayer(ctx)
	if err != nil {
		t.Fatalf("get player after save: %v", err)
	}
	if got.Credits != 250 || got.NodeID != "sys-001" {
		t.Fatalf("expected updated player state, got %+v", got)
	}
	if !reflect.DeepEqual(got.Cargo, map[economy.CommodityID]int{"food": 1}) {
		t.Fatalf("expected cargo fully replaced, got %+v", got.Cargo)
	}
	if !reflect.DeepEqual(got.Reputation, map[galaxy.NodeID]int{"sys-001": 10}) {
		t.Fatalf("expected reputation fully replaced, got %+v", got.Reputation)
	}
	if got.Alignment != p.Alignment {
		t.Fatalf("expected updated alignment, got %+v", got.Alignment)
	}
	if !reflect.DeepEqual(got.Discovered, map[galaxy.NodeID]bool{"sys-001": true}) {
		t.Fatalf("expected discovered fully replaced, got %+v", got.Discovered)
	}
	if len(got.ClaimedAnomalies) != 0 {
		t.Fatalf("expected claimed anomalies fully cleared, got %+v", got.ClaimedAnomalies)
	}
}

func TestColonyRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	c := colony.Colony{NodeID: "sys-000", Focus: "food", Population: 120, LastTickAt: now}

	if err := s.SaveColony(ctx, c); err != nil {
		t.Fatalf("save colony: %v", err)
	}

	got, ok, err := s.GetColony(ctx, "sys-000")
	if err != nil {
		t.Fatalf("get colony: %v", err)
	}
	if !ok {
		t.Fatalf("expected colony to exist at sys-000")
	}
	if got != c {
		t.Fatalf("colony mismatch: got %+v, want %+v", got, c)
	}

	_, ok, err = s.GetColony(ctx, "sys-999")
	if err != nil {
		t.Fatalf("get colony for unknown node: %v", err)
	}
	if ok {
		t.Fatalf("expected no colony at sys-999")
	}

	c2 := colony.Colony{NodeID: "sys-001", Focus: "weapons", Population: 80, LastTickAt: now}
	if err := s.SaveColony(ctx, c2); err != nil {
		t.Fatalf("save second colony: %v", err)
	}

	all, err := s.GetColonies(ctx)
	if err != nil {
		t.Fatalf("get colonies: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 colonies, got %d: %+v", len(all), all)
	}

	// SaveColony must update in place, not duplicate the row.
	c.Population = 500
	if err := s.SaveColony(ctx, c); err != nil {
		t.Fatalf("update colony: %v", err)
	}
	all, err = s.GetColonies(ctx)
	if err != nil {
		t.Fatalf("get colonies after update: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected still 2 colonies after update, got %d: %+v", len(all), all)
	}
	got, ok, err = s.GetColony(ctx, "sys-000")
	if err != nil || !ok {
		t.Fatalf("get updated colony: ok=%v err=%v", ok, err)
	}
	if got.Population != 500 {
		t.Fatalf("expected updated population 500, got %d", got.Population)
	}
}

func TestResearchRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// A save with nothing researched yet has no row: GetResearch must
	// still return a usable zero value rather than erroring.
	empty, err := s.GetResearch(ctx)
	if err != nil {
		t.Fatalf("get research before any save: %v", err)
	}
	if empty.Active != "" || len(empty.Unlocked) != 0 {
		t.Fatalf("expected empty research state, got %+v", empty)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	r := techtree.Research{
		Active:              "cargo-2",
		Progress:            15,
		LastTickAt:          now,
		Unlocked:            map[techtree.TechID]bool{"cargo-1": true, "trade-1": true},
		RateBonus:           1,
		TradeGreedReduction: 2,
	}
	if err := s.SaveResearch(ctx, r); err != nil {
		t.Fatalf("save research: %v", err)
	}

	got, err := s.GetResearch(ctx)
	if err != nil {
		t.Fatalf("get research: %v", err)
	}
	if got.Active != r.Active || got.Progress != r.Progress || !got.LastTickAt.Equal(r.LastTickAt) ||
		got.RateBonus != r.RateBonus || got.TradeGreedReduction != r.TradeGreedReduction {
		t.Fatalf("research scalar fields mismatch: got %+v, want %+v", got, r)
	}
	if !reflect.DeepEqual(got.Unlocked, r.Unlocked) {
		t.Fatalf("unlocked mismatch: got %+v, want %+v", got.Unlocked, r.Unlocked)
	}

	// SaveResearch must update in place (singleton row) and fully replace
	// the unlocked set, including removing entries — mirroring
	// SavePlayer's cargo/reputation/discovered semantics.
	r.Active = "logistics-1"
	r.Progress = 3
	r.Unlocked = map[techtree.TechID]bool{"cargo-1": true}
	if err := s.SaveResearch(ctx, r); err != nil {
		t.Fatalf("re-save research: %v", err)
	}
	got, err = s.GetResearch(ctx)
	if err != nil {
		t.Fatalf("get research after re-save: %v", err)
	}
	if got.Active != "logistics-1" || got.Progress != 3 {
		t.Fatalf("expected updated research state, got %+v", got)
	}
	if !reflect.DeepEqual(got.Unlocked, map[techtree.TechID]bool{"cargo-1": true}) {
		t.Fatalf("expected unlocked fully replaced, got %+v", got.Unlocked)
	}
}

func TestSpyRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	none, err := s.GetSpies(ctx)
	if err != nil {
		t.Fatalf("get spies before any save: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no spies, got %+v", none)
	}

	spy := espionage.Spy{ID: "spy-1", Name: "Nyx", Skill: 42, Status: espionage.StatusAvailable, MissionsRun: 0}
	if err := s.SaveSpy(ctx, spy); err != nil {
		t.Fatalf("save spy: %v", err)
	}

	got, err := s.GetSpies(ctx)
	if err != nil {
		t.Fatalf("get spies: %v", err)
	}
	if len(got) != 1 || got[0] != spy {
		t.Fatalf("expected round-tripped spy %+v, got %+v", spy, got)
	}

	// SaveSpy must update the row in place (by ID) rather than inserting a
	// duplicate.
	spy.Status = espionage.StatusCaptured
	spy.MissionsRun = 3
	if err := s.SaveSpy(ctx, spy); err != nil {
		t.Fatalf("re-save spy: %v", err)
	}
	got, err = s.GetSpies(ctx)
	if err != nil {
		t.Fatalf("get spies after re-save: %v", err)
	}
	if len(got) != 1 || got[0] != spy {
		t.Fatalf("expected updated spy %+v, got %+v", spy, got)
	}

	other := espionage.Spy{ID: "spy-2", Name: "Vega", Skill: 55, Status: espionage.StatusAvailable}
	if err := s.SaveSpy(ctx, other); err != nil {
		t.Fatalf("save second spy: %v", err)
	}
	got, err = s.GetSpies(ctx)
	if err != nil {
		t.Fatalf("get spies with two rows: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected two spies, got %+v", got)
	}
}
