package sqlite

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
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
}

func TestPlayerRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	p := player.Player{
		Credits:       500,
		NodeID:        "sys-000",
		CargoCapacity: 40,
		Cargo:         map[economy.CommodityID]int{"food": 5, "weapons": 2},
		Turns:         turn.New(100, 20*time.Second, now),
		Reputation:    map[galaxy.NodeID]int{"sys-000": 5, "sys-001": -3},
		Alignment:     player.Alignment{Legality: 0.4, Morality: -0.2},
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

	// SavePlayer must overwrite scalar fields and fully replace cargo and
	// reputation (including removing entries, not just upserting them).
	p.Credits = 250
	p.NodeID = "sys-001"
	p.Cargo = map[economy.CommodityID]int{"food": 1}
	p.Reputation = map[galaxy.NodeID]int{"sys-001": 10}
	p.Alignment = player.Alignment{Legality: -0.6, Morality: 0.1}
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
}
