package app

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rdu90/RPG/internal/config"
	"github.com/rdu90/RPG/internal/engine"
	"github.com/rdu90/RPG/internal/persistence/sqlite"
	"github.com/rdu90/RPG/internal/transport/local"
	"github.com/rdu90/RPG/internal/transport/query"
)

// newTestHooks returns OpenSave/ListSaves backed by real SQLite files in a
// temp dir, exercising the full tui -> transport -> engine -> ports ->
// persistence round trip exactly as cmd/rpg/main.go wires it.
func newTestHooks(t *testing.T) (OpenSave, ListSaves, func()) {
	t.Helper()
	dir := t.TempDir()

	var store *sqlite.Store
	openSave := func(name string) (*local.Client, error) {
		if store != nil {
			_ = store.Close()
			store = nil
		}
		s, err := sqlite.Open(config.SavePath(dir, name))
		if err != nil {
			return nil, err
		}
		store = s
		return local.New(engine.New(store)), nil
	}
	listSaves := func() ([]string, error) {
		return config.ListSaves(dir)
	}
	cleanup := func() {
		if store != nil {
			_ = store.Close()
		}
	}
	return openSave, listSaves, cleanup
}

// key sends a keystroke through Update and returns the resulting model,
// running any produced tea.Cmd synchronously and feeding its message back
// in, repeating until a step produces no further command. Bubbletea's real
// event loop does this asynchronously one hop at a time; tests don't need
// that concurrency, but some flows here chain multiple commands (e.g.
// creating a game triggers a world-load command once the game exists).
func key(t *testing.T, m Model, k string) Model {
	t.Helper()
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}

	next, cmd := m.Update(msg)
	nm := next.(Model)
	for cmd != nil {
		resultMsg := cmd()
		if resultMsg == nil {
			break
		}
		next, cmd = nm.Update(resultMsg)
		nm = next.(Model)
	}
	return nm
}

// typeString sends each rune of s as a keystroke.
func typeString(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m = key(t, m, string(r))
	}
	return m
}

func TestNewGameEntersGalaxyMap(t *testing.T) {
	openSave, listSaves, cleanup := newTestHooks(t)
	defer cleanup()

	m := New(openSave, listSaves)
	if !strings.Contains(m.View(), "New Game") {
		t.Fatalf("expected menu to list New Game, got:\n%s", m.View())
	}

	// Menu cursor starts on "New Game"; enter it, type a name, submit.
	m = key(t, m, "enter")
	if m.state != stateNewGameInput {
		t.Fatalf("expected stateNewGameInput, got %v", m.state)
	}
	m = typeString(t, m, "alpha")
	m = key(t, m, "enter")

	if m.state != stateMap {
		t.Fatalf("expected stateMap after create, got %v (err=%v)", m.state, m.err)
	}
	if m.game.Name != "alpha" {
		t.Fatalf("expected created game name %q, got %q", "alpha", m.game.Name)
	}
	if len(m.galaxy.Nodes) == 0 {
		t.Fatal("expected a generated galaxy")
	}
	if m.player.Credits != 500 {
		t.Fatalf("expected starting credits 500, got %d", m.player.Credits)
	}
	if !strings.Contains(m.View(), "Credits:") {
		t.Fatalf("expected map view to show credits, got:\n%s", m.View())
	}
}

func TestFlyAndHaggleRoundTrip(t *testing.T) {
	openSave, listSaves, cleanup := newTestHooks(t)
	defer cleanup()

	m := New(openSave, listSaves)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "alpha")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}

	startNode := m.player.NodeID
	startTurns := m.player.Turns.Remaining
	neighbors := m.galaxy.Neighbors(startNode)
	if len(neighbors) == 0 {
		t.Fatal("expected at least one warp lane from the starting system")
	}
	dest := neighbors[0].To
	cost := neighbors[0].TurnCost

	m = key(t, m, "enter") // fly to the first (default-selected) neighbor
	if m.state != stateMap {
		t.Fatalf("expected stateMap after flying, got %v (err=%v)", m.state, m.err)
	}
	if m.player.NodeID != dest {
		t.Fatalf("expected to arrive at %s, got %s", dest, m.player.NodeID)
	}
	if m.player.Turns.Remaining != startTurns-cost {
		t.Fatalf("expected %d turns remaining, got %d", startTurns-cost, m.player.Turns.Remaining)
	}

	// Open the trade screen and negotiate buying some food.
	m = key(t, m, "t")
	if m.state != stateTrade {
		t.Fatalf("expected stateTrade, got %v (err=%v)", m.state, m.err)
	}
	if len(m.market) == 0 {
		t.Fatal("expected a non-empty market")
	}
	creditsBeforeBuy := m.player.Credits

	m = key(t, m, "b")
	m = typeString(t, m, "3")
	m = key(t, m, "enter")
	if m.state != stateHaggle {
		t.Fatalf("expected stateHaggle after starting to buy, got %v (err=%v)", m.state, m.err)
	}
	if m.haggleSession.Outcome != query.HaggleInProgress {
		t.Fatalf("expected a fresh negotiation to be in progress, got %v", m.haggleSession.Outcome)
	}
	commodity := m.haggleSession.Commodity

	m = key(t, m, "a") // accept the NPC's opening offer
	if m.state != stateHaggle {
		t.Fatalf("expected stateHaggle after accepting, got %v (err=%v)", m.state, m.err)
	}
	if m.haggleSession.Outcome != query.HaggleAccepted {
		t.Fatalf("expected the negotiation to be Accepted, got %v", m.haggleSession.Outcome)
	}
	if m.player.Cargo[commodity] != 3 {
		t.Fatalf("expected 3 units of %s in cargo, got %d", commodity, m.player.Cargo[commodity])
	}
	if m.player.Credits >= creditsBeforeBuy {
		t.Fatalf("expected credits to decrease after buying, before=%d after=%d", creditsBeforeBuy, m.player.Credits)
	}

	m = key(t, m, "enter") // acknowledge the concluded negotiation
	if m.state != stateTrade {
		t.Fatalf("expected stateTrade after acknowledging the deal, got %v (err=%v)", m.state, m.err)
	}

	// Sell it back.
	creditsBeforeSell := m.player.Credits
	m = key(t, m, "s")
	m = typeString(t, m, "3")
	m = key(t, m, "enter")
	if m.state != stateHaggle {
		t.Fatalf("expected stateHaggle after starting to sell, got %v (err=%v)", m.state, m.err)
	}
	m = key(t, m, "a")
	if m.haggleSession.Outcome != query.HaggleAccepted {
		t.Fatalf("expected the negotiation to be Accepted, got %v", m.haggleSession.Outcome)
	}
	if _, has := m.player.Cargo[commodity]; has {
		t.Fatalf("expected cargo to be empty after selling all units, got %v", m.player.Cargo)
	}
	if m.player.Credits <= creditsBeforeSell {
		t.Fatalf("expected credits to increase after selling, before=%d after=%d", creditsBeforeSell, m.player.Credits)
	}
	m = key(t, m, "enter") // back to the trade screen

	// Simulate a fresh process: new Model over the same save directory,
	// confirming position, turns, and credits all persisted.
	m2 := New(openSave, listSaves)
	m2 = key(t, m2, "down") // move cursor to "Load Game"
	m2 = key(t, m2, "enter")
	if m2.state != stateLoadList {
		t.Fatalf("expected stateLoadList, got %v (err=%v)", m2.state, m2.err)
	}
	if len(m2.saves) != 1 || m2.saves[0] != "alpha" {
		t.Fatalf("expected saves=[alpha], got %v", m2.saves)
	}

	m2 = key(t, m2, "enter")
	if m2.state != stateMap {
		t.Fatalf("expected stateMap after load, got %v (err=%v)", m2.state, m2.err)
	}
	if m2.player.NodeID != dest {
		t.Fatalf("expected reloaded player at %s, got %s", dest, m2.player.NodeID)
	}
	if m2.player.Credits != m.player.Credits {
		t.Fatalf("expected reloaded credits %d, got %d", m.player.Credits, m2.player.Credits)
	}
	if m2.player.Turns.Remaining != m.player.Turns.Remaining {
		t.Fatalf("expected reloaded turns %d, got %d", m.player.Turns.Remaining, m2.player.Turns.Remaining)
	}
}

func TestScoutRevealsSystemAtHalfTurnCost(t *testing.T) {
	openSave, listSaves, cleanup := newTestHooks(t)
	defer cleanup()

	m := New(openSave, listSaves)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "alpha")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}

	neighbors := m.galaxy.Neighbors(m.player.NodeID)
	if len(neighbors) == 0 {
		t.Fatal("expected at least one warp lane from the starting system")
	}
	target := neighbors[0].To // mapCursor starts at 0
	if m.player.HasDiscovered(target) {
		t.Skip("first neighbor already discovered at this seed; nothing to scout")
	}
	edgeCost := neighbors[0].TurnCost
	turnsBefore := m.player.Turns.Remaining

	m = key(t, m, "x")
	if m.state != stateMap {
		t.Fatalf("expected stateMap after scouting, got %v (err=%v)", m.state, m.err)
	}
	if !m.player.HasDiscovered(target) {
		t.Fatal("expected the scouted system to be discovered")
	}
	if m.player.NodeID == target {
		t.Fatal("expected scouting to not move the player")
	}
	wantCost := (edgeCost + 1) / 2
	if got := turnsBefore - m.player.Turns.Remaining; got != wantCost {
		t.Fatalf("expected scouting to cost %d turns, spent %d", wantCost, got)
	}
	if !strings.Contains(m.View(), "Scout report:") {
		t.Fatalf("expected a scout report on the map screen, got:\n%s", m.View())
	}

	// Scouting an already-discovered system is guarded client-side as a
	// no-op, so it costs nothing further.
	m2 := key(t, m, "x")
	if m2.player.Turns.Remaining != m.player.Turns.Remaining {
		t.Fatal("expected re-scouting an already-discovered system to cost nothing")
	}
}

func TestClaimAnomalyAtStartingSystem(t *testing.T) {
	openSave, listSaves, cleanup := newTestHooks(t)
	defer cleanup()

	var m Model
	found := false
	for i := 0; i < 40 && !found; i++ {
		m = New(openSave, listSaves)
		m = key(t, m, "enter") // New Game
		m = typeString(t, m, fmt.Sprintf("save%d", i))
		m = key(t, m, "enter")
		if m.state != stateMap {
			t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
		}
		found = !m.anomaly.Anomaly.Empty()
	}
	if !found {
		t.Skip("could not roll a starting system with an anomaly within the attempt budget")
	}

	creditsBefore := m.player.Credits
	repBefore := m.player.ReputationAt(m.player.NodeID)
	wantAnomaly := m.anomaly.Anomaly

	m = key(t, m, "c")
	if m.state != stateMap {
		t.Fatalf("expected stateMap after claiming, got %v (err=%v)", m.state, m.err)
	}
	if !m.anomaly.Claimed {
		t.Fatal("expected the anomaly to be marked claimed")
	}
	if m.player.Credits != creditsBefore+wantAnomaly.CreditsReward {
		t.Fatalf("expected credits %d, got %d", creditsBefore+wantAnomaly.CreditsReward, m.player.Credits)
	}
	if got := m.player.ReputationAt(m.player.NodeID); got != repBefore+wantAnomaly.ReputationReward {
		t.Fatalf("expected reputation %d, got %d", repBefore+wantAnomaly.ReputationReward, got)
	}

	// Claiming again is guarded client-side as a no-op.
	m2 := key(t, m, "c")
	if m2.player.Credits != m.player.Credits {
		t.Fatal("expected re-claiming an already-claimed anomaly to do nothing")
	}
}
