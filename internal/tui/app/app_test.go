package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rdu90/RPG/internal/config"
	"github.com/rdu90/RPG/internal/engine"
	"github.com/rdu90/RPG/internal/persistence/sqlite"
	"github.com/rdu90/RPG/internal/transport/local"
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

func TestFlyAndTradeRoundTrip(t *testing.T) {
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

	// Open the trade screen and buy some food.
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
	if m.state != stateTrade {
		t.Fatalf("expected stateTrade after buying, got %v (err=%v)", m.state, m.err)
	}
	commodity := m.market[0].CommodityID
	if m.player.Cargo[commodity] != 3 {
		t.Fatalf("expected 3 units of %s in cargo, got %d", commodity, m.player.Cargo[commodity])
	}
	if m.player.Credits >= creditsBeforeBuy {
		t.Fatalf("expected credits to decrease after buying, before=%d after=%d", creditsBeforeBuy, m.player.Credits)
	}

	// Sell it back.
	creditsBeforeSell := m.player.Credits
	m = key(t, m, "s")
	m = typeString(t, m, "3")
	m = key(t, m, "enter")
	if m.state != stateTrade {
		t.Fatalf("expected stateTrade after selling, got %v (err=%v)", m.state, m.err)
	}
	if _, has := m.player.Cargo[commodity]; has {
		t.Fatalf("expected cargo to be empty after selling all units, got %v", m.player.Cargo)
	}
	if m.player.Credits <= creditsBeforeSell {
		t.Fatalf("expected credits to increase after selling, before=%d after=%d", creditsBeforeSell, m.player.Credits)
	}

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
