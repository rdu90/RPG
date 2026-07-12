package engine

import (
	"context"
	"testing"
	"time"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/ports"
)

// fakeRepo is an in-memory ports.Repository, standing in for a real
// persistence backend so engine command/query logic can be tested without
// touching SQL. It deliberately does not import any persistence package,
// preserving the "engine never depends on persistence" rule even at the
// test-file level.
type fakeRepo struct {
	game   ports.Game
	galaxy galaxy.Galaxy
	market map[galaxy.NodeID][]economy.Price
	player player.Player
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{market: map[galaxy.NodeID][]economy.Price{}}
}

func (r *fakeRepo) CreateGame(_ context.Context, name string) (ports.Game, error) {
	r.game = ports.Game{ID: "game-1", Name: name, CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC()}
	return r.game, nil
}

func (r *fakeRepo) GetGame(_ context.Context) (ports.Game, error) { return r.game, nil }

func (r *fakeRepo) SaveGalaxy(_ context.Context, g galaxy.Galaxy) error {
	r.galaxy = g
	return nil
}

func (r *fakeRepo) GetGalaxy(_ context.Context) (galaxy.Galaxy, error) { return r.galaxy, nil }

func (r *fakeRepo) SaveMarket(_ context.Context, nodeID galaxy.NodeID, prices []economy.Price) error {
	r.market[nodeID] = prices
	return nil
}

func (r *fakeRepo) GetMarket(_ context.Context, nodeID galaxy.NodeID) ([]economy.Price, error) {
	return r.market[nodeID], nil
}

func (r *fakeRepo) InitPlayer(_ context.Context, p player.Player) error {
	r.player = p
	return nil
}

func (r *fakeRepo) GetPlayer(_ context.Context) (player.Player, error) { return r.player, nil }

func (r *fakeRepo) SavePlayer(_ context.Context, p player.Player) error {
	r.player = p
	return nil
}

func newTestEngine(t *testing.T) (*Engine, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	return New(repo), repo
}

func TestCreateGameWiresUpNewSave(t *testing.T) {
	e, repo := newTestEngine(t)

	res, err := e.Execute(context.Background(), CreateGame{Name: "alpha"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	game := res.(ports.Game)
	if game.Name != "alpha" {
		t.Fatalf("expected game name alpha, got %q", game.Name)
	}

	if len(repo.galaxy.Nodes) != galaxySize {
		t.Fatalf("expected %d galaxy nodes, got %d", galaxySize, len(repo.galaxy.Nodes))
	}
	for _, n := range repo.galaxy.Nodes {
		prices, ok := repo.market[n.ID]
		if !ok || len(prices) != len(economy.Commodities) {
			t.Fatalf("expected a full market for node %s, got %v", n.ID, prices)
		}
	}

	if repo.player.Credits != startingCredits {
		t.Fatalf("expected starting credits %d, got %d", startingCredits, repo.player.Credits)
	}
	if repo.player.NodeID != repo.galaxy.Nodes[0].ID {
		t.Fatalf("expected player to start at %s, got %s", repo.galaxy.Nodes[0].ID, repo.player.NodeID)
	}
	if repo.player.CargoCapacity != cargoCapacity {
		t.Fatalf("expected cargo capacity %d, got %d", cargoCapacity, repo.player.CargoCapacity)
	}
	if repo.player.Turns.Remaining != turnsMax {
		t.Fatalf("expected full turns %d, got %d", turnsMax, repo.player.Turns.Remaining)
	}
}

func TestMoveAlongWarpLane(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	start := repo.player.NodeID
	edge := repo.galaxy.Neighbors(start)[0]

	res, err := e.Execute(context.Background(), Move{To: edge.To})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := res.(player.Player)
	if p.NodeID != edge.To {
		t.Fatalf("expected player at %s, got %s", edge.To, p.NodeID)
	}
	if p.Turns.Remaining != turnsMax-edge.TurnCost {
		t.Fatalf("expected %d turns remaining, got %d", turnsMax-edge.TurnCost, p.Turns.Remaining)
	}
}

func TestMoveRejectsNonAdjacentNode(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find a node with no direct edge from the start.
	start := repo.player.NodeID
	neighbors := map[galaxy.NodeID]bool{}
	for _, e := range repo.galaxy.Neighbors(start) {
		neighbors[e.To] = true
	}
	var farNode galaxy.NodeID
	for _, n := range repo.galaxy.Nodes {
		if n.ID != start && !neighbors[n.ID] {
			farNode = n.ID
			break
		}
	}
	if farNode == "" {
		t.Skip("galaxy is fully connected at this seed; nothing to test")
	}

	if _, err := e.Execute(context.Background(), Move{To: farNode}); err == nil {
		t.Fatal("expected error moving to a non-adjacent node")
	}
	if repo.player.NodeID != start {
		t.Fatalf("expected player to stay at %s after rejected move, got %s", start, repo.player.NodeID)
	}
}

func TestBuyAndSellRoundTrip(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	startingCreditsBefore := repo.player.Credits
	price, ok := findPrice(repo.market[repo.player.NodeID], "food")
	if !ok {
		t.Fatal("expected food to be traded at the starting system")
	}

	res, err := e.Execute(context.Background(), Buy{Commodity: "food", Quantity: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := res.(player.Player)
	if p.Cargo["food"] != 5 {
		t.Fatalf("expected 5 food in cargo, got %d", p.Cargo["food"])
	}
	if p.Credits != startingCreditsBefore-price*5 {
		t.Fatalf("expected credits %d, got %d", startingCreditsBefore-price*5, p.Credits)
	}

	res, err = e.Execute(context.Background(), Sell{Commodity: "food", Quantity: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p = res.(player.Player)
	if _, has := p.Cargo["food"]; has {
		t.Fatalf("expected food removed from cargo after selling all, got %v", p.Cargo)
	}
	if p.Credits != startingCreditsBefore {
		t.Fatalf("expected credits back to %d after round trip, got %d", startingCreditsBefore, p.Credits)
	}
}

func TestBuyRejectsInsufficientCredits(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := e.Execute(context.Background(), Buy{Commodity: "artifacts", Quantity: 1000}); err == nil {
		t.Fatal("expected error buying more than the player can afford")
	}
}

func TestBuyRejectsInsufficientCargoSpace(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := e.Execute(context.Background(), Buy{Commodity: "food", Quantity: cargoCapacity + 1}); err == nil {
		t.Fatal("expected error buying more than cargo capacity")
	}
}

func TestSellRejectsInsufficientCargo(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := e.Execute(context.Background(), Sell{Commodity: "food", Quantity: 1}); err == nil {
		t.Fatal("expected error selling cargo the player doesn't have")
	}
}

func TestGetMarketReturnsCurrentSystemPrices(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := e.Query(context.Background(), GetMarket{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prices := res.([]economy.Price)
	want := repo.market[repo.player.NodeID]
	if len(prices) != len(want) {
		t.Fatalf("expected %d prices, got %d", len(want), len(prices))
	}
}

func TestUnhandledCommandAndQuery(t *testing.T) {
	e, _ := newTestEngine(t)

	if _, err := e.Execute(context.Background(), unknownCommand{}); err == nil {
		t.Fatal("expected error for unhandled command")
	}
	if _, err := e.Query(context.Background(), unknownQuery{}); err == nil {
		t.Fatal("expected error for unhandled query")
	}
}

type unknownCommand struct{}

func (unknownCommand) isCommand() {}

type unknownQuery struct{}

func (unknownQuery) isQuery() {}
