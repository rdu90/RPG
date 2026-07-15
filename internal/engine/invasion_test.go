package engine

import (
	"context"
	"testing"

	"github.com/rdu90/RPG/internal/engine/colony"
	"github.com/rdu90/RPG/internal/engine/combat"
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/fleet"
)

func TestCreateGameSeedsRivalColoniesDeterministically(t *testing.T) {
	e1, repo1 := newTestEngine(t)
	if _, err := e1.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e2, repo2 := newTestEngine(t)
	if _, err := e2.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo1.colonies) == 0 {
		t.Fatal("expected at least one rival colony to be seeded on a 16-node galaxy")
	}
	if len(repo1.colonies) != len(repo2.colonies) {
		t.Fatalf("expected the same number of seeded colonies across identical GameIDs, got %d vs %d", len(repo1.colonies), len(repo2.colonies))
	}
	for nodeID, c1 := range repo1.colonies {
		c2, ok := repo2.colonies[nodeID]
		if !ok {
			t.Fatalf("expected colony at %s in both saves", nodeID)
		}
		if c1.Owner != c2.Owner || c1.Garrison != c2.Garrison || c1.Focus != c2.Focus {
			t.Fatalf("expected identical seeded colony at %s, got %+v vs %+v", nodeID, c1, c2)
		}
		if c1.NodeID == repo1.player.NodeID {
			t.Fatalf("expected no rival colony seeded at the player's starting system %s", c1.NodeID)
		}
		if c1.Owner == colony.OwnerPlayer {
			t.Fatalf("expected seeded colony at %s to belong to a rival faction, got player", nodeID)
		}
	}
}

// seedRivalColonyAtPlayer places a rival colony directly at the player's
// current system, bypassing galaxy-generation seeding, so bombard/invade
// tests don't depend on where CreateGame happened to seed one.
func seedRivalColonyAtPlayer(repo *fakeRepo, garrison fleet.Stats) {
	repo.colonies[repo.player.NodeID] = colony.Colony{
		NodeID:     repo.player.NodeID,
		Focus:      "food",
		Population: 500,
		LastTickAt: repo.player.Turns.LastRefillAt,
		Owner:      "crimson-hand",
		Garrison:   garrison,
	}
}

func TestBombardRejectsWhenNoColony(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := e.Execute(context.Background(), Bombard{}); err == nil {
		t.Fatal("expected error bombarding a system with no colony")
	}
}

func TestBombardRejectsPlayerOwnedColony(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.player.Credits = 10000
	if _, err := e.Execute(context.Background(), Colonize{Focus: "food"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := e.Execute(context.Background(), Bombard{}); err == nil {
		t.Fatal("expected error bombarding the player's own colony")
	}
}

func TestBombardWeakensGarrisonAndFloorsWithoutFlippingOwnership(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.player.Ship = fleet.Stats{Attack: 100, Defense: 10, Hull: 200, MaxHull: 200}
	repo.player.Turns.Max = 1000
	repo.player.Turns.Remaining = 1000
	seedRivalColonyAtPlayer(repo, fleet.Stats{Attack: 10, Defense: 5, Hull: 100, MaxHull: 100})
	floor := int(float64(100) * bombardGarrisonFloorFraction)

	var last colony.Colony
	for i := 0; i < 50; i++ {
		res, err := e.Execute(context.Background(), Bombard{})
		if err != nil {
			t.Fatalf("unexpected error on bombardment %d: %v", i, err)
		}
		result := res.(BombardResult)
		if result.Colony.Owner == colony.OwnerPlayer {
			t.Fatal("expected bombardment alone to never flip ownership")
		}
		if result.Colony.Garrison.Hull < floor {
			t.Fatalf("expected garrison hull to never drop below the floor %d, got %d", floor, result.Colony.Garrison.Hull)
		}
		last = result.Colony
	}
	if last.Garrison.Hull != floor {
		t.Fatalf("expected repeated bombardment to settle at the floor %d, got %d", floor, last.Garrison.Hull)
	}
	if last.Population >= 500 {
		t.Fatalf("expected population to have dropped from bombardment, got %d", last.Population)
	}
}

func TestInvadeRejectsWhenNoColonyOrPlayerOwned(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := e.Execute(context.Background(), Invade{}); err == nil {
		t.Fatal("expected error invading a system with no colony")
	}

	repo.player.Credits = 10000
	if _, err := e.Execute(context.Background(), Colonize{Focus: "food"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := e.Execute(context.Background(), Invade{}); err == nil {
		t.Fatal("expected error invading the player's own colony")
	}
}

func TestInvadeVictoryCapturesColony(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.player.Ship = fleet.Stats{Attack: 100, Defense: 100, Hull: 200, MaxHull: 200}
	seedRivalColonyAtPlayer(repo, fleet.Stats{Attack: 1, Defense: 1, Hull: 5, MaxHull: 5})

	res, err := e.Execute(context.Background(), Invade{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(InvadeResult)
	if !result.Captured {
		t.Fatalf("expected victory to capture the colony, got outcome %s", result.Battle.Outcome)
	}
	if result.Colony.Owner != colony.OwnerPlayer {
		t.Fatalf("expected captured colony to belong to the player, got %q", result.Colony.Owner)
	}
	if result.Defender != "crimson-hand" {
		t.Fatalf("expected Defender to preserve the pre-capture owner even though Colony.Owner flips, got %q", result.Defender)
	}
	if result.Colony.Garrison != (fleet.Stats{}) {
		t.Fatalf("expected garrison to be stood down after capture, got %+v", result.Colony.Garrison)
	}
	if repo.colonies[repo.player.NodeID].Owner != colony.OwnerPlayer {
		t.Fatal("expected the ownership change to be persisted")
	}
}

func TestInvadeDefeatCostsPlayerAndWearsGarrison(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.player.Ship = fleet.Stats{Attack: 1, Defense: 1, Hull: 10, MaxHull: 10}
	repo.player.Cargo[economy.Commodities[0].ID] = 10
	creditsBefore := repo.player.Credits
	seedRivalColonyAtPlayer(repo, fleet.Stats{Attack: 100, Defense: 100, Hull: 500, MaxHull: 500})

	res, err := e.Execute(context.Background(), Invade{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(InvadeResult)
	if result.Battle.Outcome != combat.Defeat {
		t.Fatalf("expected Defeat, got %s", result.Battle.Outcome)
	}
	if result.Captured {
		t.Fatal("expected defeat to not capture the colony")
	}
	if result.Colony.Owner == colony.OwnerPlayer {
		t.Fatal("expected ownership to remain with the rival faction after defeat")
	}
	if result.Colony.Garrison.Hull >= 500 {
		t.Fatalf("expected the garrison to take some damage even in defeat, got %d/500", result.Colony.Garrison.Hull)
	}
	if result.Player.Credits >= creditsBefore {
		t.Fatal("expected defeat to cost the player credits")
	}
	if result.Player.Cargo[economy.Commodities[0].ID] >= 10 {
		t.Fatal("expected defeat to cost the player cargo")
	}
}

func TestGetColoniesExcludesRivalOwnedColonies(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.player.Credits = 10000
	if _, err := e.Execute(context.Background(), Colonize{Focus: "food"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	neighbor := repo.galaxy.Neighbors(repo.player.NodeID)[0].To
	repo.colonies[neighbor] = colony.Colony{
		NodeID: neighbor, Focus: "weapons", Population: 200, LastTickAt: repo.player.Turns.LastRefillAt,
		Owner: "crimson-hand", Garrison: fleet.Stats{Attack: 10, Defense: 5, Hull: 50, MaxHull: 50},
	}

	res, err := e.Query(context.Background(), GetColonies{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cols := res.([]colony.Colony)
	for _, c := range cols {
		if c.Owner != colony.OwnerPlayer {
			t.Fatalf("expected only player-owned colonies, got one owned by %q", c.Owner)
		}
	}
	if len(cols) != 1 {
		t.Fatalf("expected exactly 1 player-owned colony, got %d", len(cols))
	}
}
