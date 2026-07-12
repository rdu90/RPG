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
// in (bubbletea's real event loop does this asynchronously; tests don't
// need that concurrency).
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
	if cmd != nil {
		if resultMsg := cmd(); resultMsg != nil {
			next, _ = nm.Update(resultMsg)
			nm = next.(Model)
		}
	}
	return nm
}

func TestNewGameThenReload(t *testing.T) {
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
	for _, r := range "alpha" {
		m = key(t, m, string(r))
	}
	m = key(t, m, "enter")

	if m.state != stateGameReady {
		t.Fatalf("expected stateGameReady after create, got %v (err=%v)", m.state, m.err)
	}
	if m.game.Name != "alpha" {
		t.Fatalf("expected created game name %q, got %q", "alpha", m.game.Name)
	}
	if !strings.Contains(m.View(), "alpha") {
		t.Fatalf("expected view to show save name, got:\n%s", m.View())
	}

	// Simulate a fresh process: new Model over the same save directory.
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
	if m2.state != stateGameReady {
		t.Fatalf("expected stateGameReady after load, got %v (err=%v)", m2.state, m2.err)
	}
	if m2.game.Name != "alpha" || m2.game.CreatedAt != m.game.CreatedAt {
		t.Fatalf("loaded game %+v does not match created game %+v", m2.game, m.game)
	}
}
