package engine

import (
	"context"
	"testing"
	"time"

	"github.com/rdu90/RPG/internal/engine/colony"
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/explore"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/haggle"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/ports"
	"github.com/rdu90/RPG/internal/engine/techtree"
	"github.com/rdu90/RPG/internal/rng"
)

// fakeRepo is an in-memory ports.Repository, standing in for a real
// persistence backend so engine command/query logic can be tested without
// touching SQL. It deliberately does not import any persistence package,
// preserving the "engine never depends on persistence" rule even at the
// test-file level.
type fakeRepo struct {
	game     ports.Game
	galaxy   galaxy.Galaxy
	market   map[galaxy.NodeID][]economy.Price
	player   player.Player
	colonies map[galaxy.NodeID]colony.Colony
	research techtree.Research
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		market:   map[galaxy.NodeID][]economy.Price{},
		colonies: map[galaxy.NodeID]colony.Colony{},
		research: techtree.Research{Unlocked: map[techtree.TechID]bool{}},
	}
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

func (r *fakeRepo) SaveColony(_ context.Context, c colony.Colony) error {
	r.colonies[c.NodeID] = c
	return nil
}

func (r *fakeRepo) GetColony(_ context.Context, nodeID galaxy.NodeID) (colony.Colony, bool, error) {
	c, ok := r.colonies[nodeID]
	return c, ok, nil
}

func (r *fakeRepo) GetColonies(_ context.Context) ([]colony.Colony, error) {
	out := make([]colony.Colony, 0, len(r.colonies))
	for _, c := range r.colonies {
		out = append(out, c)
	}
	return out, nil
}

func (r *fakeRepo) GetResearch(_ context.Context) (techtree.Research, error) { return r.research, nil }

func (r *fakeRepo) SaveResearch(_ context.Context, res techtree.Research) error {
	r.research = res
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
	if !repo.player.HasDiscovered(repo.player.NodeID) {
		t.Fatal("expected the starting system to already be discovered")
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
	if !p.HasDiscovered(edge.To) {
		t.Fatal("expected the destination to be discovered after arriving")
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

func TestStartHaggleOpensAwayFromFairPrice(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	price, ok := findPrice(repo.market[repo.player.NodeID], "food")
	if !ok {
		t.Fatal("expected food to be traded at the starting system")
	}

	res, err := e.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: 5, Buying: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(HaggleResult)
	if result.Session.Outcome != haggle.InProgress {
		t.Fatalf("expected a fresh session to be in progress, got %v", result.Session.Outcome)
	}
	if result.Session.NPCOffer <= price {
		t.Fatalf("expected the NPC's opening buy offer to be above fair price %d, got %d", price, result.Session.NPCOffer)
	}
}

func TestHaggleAcceptRoundTrip(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	startingCredits := repo.player.Credits
	node := repo.player.NodeID

	res, err := e.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: 5, Buying: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	session := res.(HaggleResult).Session

	res, err = e.Execute(context.Background(), HaggleAccept{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(HaggleResult)
	if result.Session.Outcome != haggle.Accepted {
		t.Fatalf("expected Accepted, got %v", result.Session.Outcome)
	}
	if result.Player.Cargo["food"] != 5 {
		t.Fatalf("expected 5 food in cargo, got %d", result.Player.Cargo["food"])
	}
	wantCredits := startingCredits - result.Session.NPCOffer*5
	if result.Player.Credits != wantCredits {
		t.Fatalf("expected credits %d, got %d", wantCredits, result.Player.Credits)
	}
	if result.Player.ReputationAt(node) <= 0 {
		t.Fatalf("expected a reputation gain at %s after a closed deal, got %d", node, result.Player.ReputationAt(node))
	}

	// Sell it back through a second negotiation.
	res, err = e.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: 5, Buying: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	session = res.(HaggleResult).Session

	res, err = e.Execute(context.Background(), HaggleAccept{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result = res.(HaggleResult)
	if _, has := result.Player.Cargo["food"]; has {
		t.Fatalf("expected food removed from cargo after selling all, got %v", result.Player.Cargo)
	}
}

func TestHaggleAcceptNudgesAlignment(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := e.Execute(context.Background(), StartHaggle{Commodity: "narcotics", Quantity: 1, Buying: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	session := res.(HaggleResult).Session

	res, err = e.Execute(context.Background(), HaggleAccept{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(HaggleResult)
	if result.Player.Alignment.Legality >= 0 {
		t.Fatalf("expected buying narcotics to pull legality negative, got %v", result.Player.Alignment.Legality)
	}
}

func TestHaggleOfferAndWalkAwaySpendATurn(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	turnsBefore := repo.player.Turns.Remaining

	res, err := e.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: 5, Buying: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	session := res.(HaggleResult).Session

	res, err = e.Execute(context.Background(), HaggleOffer{Session: session, Price: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(HaggleResult)
	if result.Player.Turns.Remaining != turnsBefore-1 {
		t.Fatalf("expected offering a round to spend a turn: before=%d after=%d", turnsBefore, result.Player.Turns.Remaining)
	}
}

func TestHaggleAcceptDoesNotSpendATurn(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := e.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: 5, Buying: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	session := res.(HaggleResult).Session

	pRes, err := e.Query(context.Background(), GetPlayer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	turnsBefore := pRes.(player.Player).Turns.Remaining

	res, err = e.Execute(context.Background(), HaggleAccept{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(HaggleResult)
	if result.Player.Turns.Remaining != turnsBefore {
		t.Fatalf("expected accepting to spend no turns: before=%d after=%d", turnsBefore, result.Player.Turns.Remaining)
	}
}

func TestHaggleWalkAwayEitherImprovesOrEndsWithPenalty(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	node := repo.player.NodeID

	res, err := e.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: 5, Buying: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	session := res.(HaggleResult).Session
	opening := session.NPCOffer

	res, err = e.Execute(context.Background(), HaggleWalkAway{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(HaggleResult)
	switch result.Session.Outcome {
	case haggle.InProgress:
		if result.Session.NPCOffer >= opening {
			t.Fatalf("expected a successful bluff to improve the offer below %d, got %d", opening, result.Session.NPCOffer)
		}
	case haggle.Rejected:
		if result.Player.ReputationAt(node) >= 0 {
			t.Fatalf("expected a reputation penalty for a failed bluff, got %d", result.Player.ReputationAt(node))
		}
	default:
		t.Fatalf("unexpected outcome %v", result.Session.Outcome)
	}
}

func TestHaggleActionOnConcludedSessionErrors(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := e.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: 5, Buying: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	session := res.(HaggleResult).Session

	res, err = e.Execute(context.Background(), HaggleAccept{Session: session})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	concluded := res.(HaggleResult).Session

	if _, err := e.Execute(context.Background(), HaggleOffer{Session: concluded, Price: 1}); err == nil {
		t.Fatal("expected error acting on a concluded session")
	}
}

func TestStartHaggleRejectsInsufficientCargoSpace(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := e.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: cargoCapacity + 1, Buying: true}); err == nil {
		t.Fatal("expected error negotiating to buy more than cargo capacity")
	}
}

func TestStartHaggleRejectsInsufficientCargoToSell(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := e.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: 1, Buying: false}); err == nil {
		t.Fatal("expected error negotiating to sell cargo the player doesn't have")
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

func TestScoutNodeDiscoversAdjacentSystemAtHalfTurnCost(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	start := repo.player.NodeID
	edge := repo.galaxy.Neighbors(start)[0]
	before := repo.player.Turns.Remaining

	res, err := e.Execute(context.Background(), ScoutNode{To: edge.To})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(ScoutResult)
	if result.NodeID != edge.To {
		t.Fatalf("expected scout result for %s, got %s", edge.To, result.NodeID)
	}
	if !result.Player.HasDiscovered(edge.To) {
		t.Fatal("expected the scouted node to be discovered")
	}
	if result.Player.NodeID != start {
		t.Fatalf("expected scouting to not move the player, still at %s, got %s", start, result.Player.NodeID)
	}

	wantCost := scoutCost(edge.TurnCost)
	if got := before - result.Player.Turns.Remaining; got != wantCost {
		t.Fatalf("expected scouting to cost %d turns, spent %d", wantCost, got)
	}
	if wantCost > edge.TurnCost {
		t.Fatalf("expected scouting (%d) to never cost more than flying (%d)", wantCost, edge.TurnCost)
	}
}

func TestScoutNodeRejectsNonAdjacentNode(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	if _, err := e.Execute(context.Background(), ScoutNode{To: farNode}); err == nil {
		t.Fatal("expected error scouting a non-adjacent node")
	}
	if repo.player.HasDiscovered(farNode) {
		t.Fatal("expected a rejected scout to not discover anything")
	}
}

func TestScoutNodeRejectsAlreadyDiscoveredNode(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	edge := repo.galaxy.Neighbors(repo.player.NodeID)[0]
	if _, err := e.Execute(context.Background(), ScoutNode{To: edge.To}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := e.Execute(context.Background(), ScoutNode{To: edge.To}); err == nil {
		t.Fatal("expected error re-scouting an already-discovered system")
	}
}

// anomalyAtSeed replays the same deterministic roll the engine uses, so
// tests can locate a node that does (or doesn't) hide something without
// depending on random luck.
func anomalyAtSeed(gameID ports.GameID, n galaxy.Node) explore.Anomaly {
	return explore.At(rng.New(anomalySeed(gameID, n.ID)), n.DevelopmentLevel)
}

func TestClaimAnomalyCollectsRewardOnce(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var target galaxy.NodeID
	var want explore.Anomaly
	for _, n := range repo.galaxy.Nodes {
		if a := anomalyAtSeed(repo.game.ID, n); !a.Empty() {
			target, want = n.ID, a
			break
		}
	}
	if target == "" {
		t.Skip("no anomaly present anywhere in this galaxy at this seed")
	}

	repo.player.NodeID = target
	creditsBefore := repo.player.Credits
	repBefore := repo.player.ReputationAt(target)

	res, err := e.Execute(context.Background(), ClaimAnomaly{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(ClaimAnomalyResult)
	if result.Anomaly != want {
		t.Fatalf("expected anomaly %+v, got %+v", want, result.Anomaly)
	}
	if result.Player.Credits != creditsBefore+want.CreditsReward {
		t.Fatalf("expected credits %d, got %d", creditsBefore+want.CreditsReward, result.Player.Credits)
	}
	if got := result.Player.ReputationAt(target); got != repBefore+want.ReputationReward {
		t.Fatalf("expected reputation %d, got %d", repBefore+want.ReputationReward, got)
	}
	if !result.Player.HasClaimedAnomaly(target) {
		t.Fatal("expected the anomaly to be marked claimed")
	}

	if _, err := e.Execute(context.Background(), ClaimAnomaly{}); err == nil {
		t.Fatal("expected error claiming an already-claimed anomaly")
	}
}

func TestClaimAnomalyErrorsWhenNothingHidden(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var target galaxy.NodeID
	for _, n := range repo.galaxy.Nodes {
		if anomalyAtSeed(repo.game.ID, n).Empty() {
			target = n.ID
			break
		}
	}
	if target == "" {
		t.Skip("every system in this galaxy hides an anomaly at this seed")
	}

	repo.player.NodeID = target
	if _, err := e.Execute(context.Background(), ClaimAnomaly{}); err == nil {
		t.Fatal("expected error claiming when nothing is hidden")
	}
}

func TestGetAnomalyReflectsClaimedState(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var target galaxy.NodeID
	var want explore.Anomaly
	for _, n := range repo.galaxy.Nodes {
		if a := anomalyAtSeed(repo.game.ID, n); !a.Empty() {
			target, want = n.ID, a
			break
		}
	}
	if target == "" {
		t.Skip("no anomaly present anywhere in this galaxy at this seed")
	}
	repo.player.NodeID = target

	res, err := e.Query(context.Background(), GetAnomaly{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := res.(AnomalyStatus)
	if status.Anomaly != want {
		t.Fatalf("expected anomaly %+v, got %+v", want, status.Anomaly)
	}
	if status.Claimed {
		t.Fatal("expected the anomaly to be unclaimed before collecting it")
	}

	if _, err := e.Execute(context.Background(), ClaimAnomaly{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res2, err := e.Query(context.Background(), GetAnomaly{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res2.(AnomalyStatus).Claimed {
		t.Fatal("expected the anomaly to be claimed after collecting it")
	}
}

func TestColonizeFoundsColonyAndSpendsCreditsAndTurns(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repo.player.Credits = 10000
	node, _ := repo.galaxy.Node(repo.player.NodeID)
	wantCost := colonizeBaseCost + colonizeCostPerDevelopmentLevel*node.DevelopmentLevel
	creditsBefore := repo.player.Credits
	turnsBefore := repo.player.Turns.Remaining

	res, err := e.Execute(context.Background(), Colonize{Focus: "food"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(ColonizeResult)
	if result.Colony.NodeID != repo.player.NodeID {
		t.Fatalf("expected colony at %s, got %s", repo.player.NodeID, result.Colony.NodeID)
	}
	if result.Colony.Focus != "food" {
		t.Fatalf("expected focus food, got %s", result.Colony.Focus)
	}
	if result.Player.Credits != creditsBefore-wantCost {
		t.Fatalf("expected credits %d, got %d", creditsBefore-wantCost, result.Player.Credits)
	}
	if result.Player.Turns.Remaining != turnsBefore-colonizeTurnCost {
		t.Fatalf("expected turns %d, got %d", turnsBefore-colonizeTurnCost, result.Player.Turns.Remaining)
	}
	if _, ok := repo.colonies[repo.player.NodeID]; !ok {
		t.Fatal("expected colony to be persisted")
	}
}

func TestColonizeRejectsWhenColonyAlreadyExists(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.player.Credits = 10000
	if _, err := e.Execute(context.Background(), Colonize{Focus: "food"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := e.Execute(context.Background(), Colonize{Focus: "weapons"}); err == nil {
		t.Fatal("expected error founding a second colony at the same system")
	}
}

func TestColonizeRejectsUnknownCommodity(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := e.Execute(context.Background(), Colonize{Focus: "unobtainium"}); err == nil {
		t.Fatal("expected error founding a colony with an unknown commodity focus")
	}
}

func TestColonizeRejectsInsufficientCredits(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.player.Credits = 0

	if _, err := e.Execute(context.Background(), Colonize{Focus: "food"}); err == nil {
		t.Fatal("expected error founding a colony without enough credits")
	}
	if _, ok := repo.colonies[repo.player.NodeID]; ok {
		t.Fatal("expected no colony to be persisted on a rejected founding")
	}
}

func TestGetColonyReflectsNoColony(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := e.Query(context.Background(), GetColony{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.(ColonyStatus).Exists {
		t.Fatal("expected no colony to exist at a freshly-created save's starting system")
	}
}

func TestGetColonyTicksPopulationGrowthAndDecaysFocusPrice(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.player.Credits = 10000
	if _, err := e.Execute(context.Background(), Colonize{Focus: "food"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	node := repo.player.NodeID
	priceBefore, ok := findPrice(repo.market[node], "food")
	if !ok {
		t.Fatal("expected food to be traded at the colony's system")
	}
	populationBefore := repo.colonies[node].Population

	// Force the colony's clock far into the past so ticks have elapsed.
	col := repo.colonies[node]
	col.LastTickAt = col.LastTickAt.Add(-10 * time.Hour)
	repo.colonies[node] = col

	res, err := e.Query(context.Background(), GetColony{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := res.(ColonyStatus)
	if !status.Exists {
		t.Fatal("expected colony to exist")
	}
	if status.Colony.Population <= populationBefore {
		t.Fatalf("expected population to grow, got %d <= %d", status.Colony.Population, populationBefore)
	}

	priceAfter, ok := findPrice(repo.market[node], "food")
	if !ok {
		t.Fatal("expected food still traded at the colony's system")
	}
	if priceAfter >= priceBefore {
		t.Fatalf("expected the focus commodity's price to decay from production, got %d >= %d", priceAfter, priceBefore)
	}

	if repo.colonies[node].Population != status.Colony.Population {
		t.Fatal("expected the ticked colony to be persisted")
	}
}

func TestGetColoniesReturnsAllColoniesTicked(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.player.Credits = 10000
	if _, err := e.Execute(context.Background(), Colonize{Focus: "food"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	neighbor := repo.galaxy.Neighbors(repo.player.NodeID)[0].To
	repo.player.NodeID = neighbor
	repo.player.Credits = 10000
	if _, err := e.Execute(context.Background(), Colonize{Focus: "weapons"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := e.Query(context.Background(), GetColonies{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cols := res.([]colony.Colony)
	if len(cols) != 2 {
		t.Fatalf("expected 2 colonies, got %d", len(cols))
	}
}

func TestStartResearchBeginsProject(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := e.Execute(context.Background(), StartResearch{Tech: "cargo-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := res.(StartResearchResult)
	if result.Research.Active != "cargo-1" {
		t.Fatalf("expected active tech cargo-1, got %s", result.Research.Active)
	}
	if result.Research.Progress != 0 {
		t.Fatalf("expected fresh progress 0, got %d", result.Research.Progress)
	}
	if repo.research.Active != "cargo-1" {
		t.Fatal("expected research to be persisted")
	}
}

func TestStartResearchRejectsUnknownTech(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := e.Execute(context.Background(), StartResearch{Tech: "warp-drive"}); err == nil {
		t.Fatal("expected error starting an unknown tech")
	}
}

func TestStartResearchRejectsUnavailableTech(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// cargo-2 requires cargo-1, which hasn't been researched yet.
	if _, err := e.Execute(context.Background(), StartResearch{Tech: "cargo-2"}); err == nil {
		t.Fatal("expected error starting a tech whose prerequisite isn't unlocked")
	}
}

func TestGetTechTreeReturnsCatalogAndProgress(t *testing.T) {
	e, _ := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, err := e.Query(context.Background(), GetTechTree{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := res.(TechTreeStatus)
	if len(status.Catalog) != len(techtree.Catalog) {
		t.Fatalf("expected full catalog of %d techs, got %d", len(techtree.Catalog), len(status.Catalog))
	}
	if status.Research.Active != "" {
		t.Fatalf("expected no active research yet, got %s", status.Research.Active)
	}
}

func TestGetTechTreeTicksProgressAndAppliesCargoCapacityEffect(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cargoBefore := repo.player.CargoCapacity

	if _, err := e.Execute(context.Background(), StartResearch{Tech: "cargo-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Force the research clock far into the past so ticks have elapsed
	// well past cargo-1's cost (40 points at 4/tick = 10 ticks).
	repo.research.LastTickAt = repo.research.LastTickAt.Add(-time.Hour)

	res, err := e.Query(context.Background(), GetTechTree{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := res.(TechTreeStatus)
	if !status.Research.HasUnlocked("cargo-1") {
		t.Fatal("expected cargo-1 to have completed")
	}
	if status.Research.Active != "" {
		t.Fatalf("expected no active project after completion, got %s", status.Research.Active)
	}

	wantCapacity := cargoBefore + 5 // cargo-1's magnitude
	if status.Player.CargoCapacity != wantCapacity {
		t.Fatalf("expected cargo capacity %d, got %d", wantCapacity, status.Player.CargoCapacity)
	}
	if repo.player.CargoCapacity != wantCapacity {
		t.Fatal("expected the cargo capacity effect to be persisted on the player")
	}
	if !repo.research.HasUnlocked("cargo-1") {
		t.Fatal("expected the completed research to be persisted")
	}
}

func TestGetTechTreeAppliesTurnMaxEffect(t *testing.T) {
	e, repo := newTestEngine(t)
	if _, err := e.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	turnsMaxBefore := repo.player.Turns.Max

	if _, err := e.Execute(context.Background(), StartResearch{Tech: "logistics-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.research.LastTickAt = repo.research.LastTickAt.Add(-time.Hour)

	res, err := e.Query(context.Background(), GetTechTree{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := res.(TechTreeStatus)
	wantMax := turnsMaxBefore + 3 // logistics-1's magnitude
	if status.Player.Turns.Max != wantMax {
		t.Fatalf("expected turns max %d, got %d", wantMax, status.Player.Turns.Max)
	}
}

func TestStartHaggleAppliesTradeGreedReduction(t *testing.T) {
	baseline, _ := newTestEngine(t)
	if _, err := baseline.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	res, err := baseline.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: 5, Buying: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	baselineOffer := res.(HaggleResult).Session.NPCOffer

	discounted, discountedRepo := newTestEngine(t)
	if _, err := discounted.Execute(context.Background(), CreateGame{Name: "alpha"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	discountedRepo.research.TradeGreedReduction = 50
	res, err = discounted.Execute(context.Background(), StartHaggle{Commodity: "food", Quantity: 5, Buying: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	discountedOffer := res.(HaggleResult).Session.NPCOffer

	if discountedOffer >= baselineOffer {
		t.Fatalf("expected trade greed reduction to improve the buying offer: baseline=%d discounted=%d",
			baselineOffer, discountedOffer)
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
