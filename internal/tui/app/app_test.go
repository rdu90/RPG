package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

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

// dismissTitle presses any key to advance past the opening title screen,
// landing on the main menu.
func dismissTitle(t *testing.T, m Model) Model {
	t.Helper()
	if m.state != stateTitle {
		t.Fatalf("expected stateTitle, got %v", m.state)
	}
	m = key(t, m, "enter")
	if m.state != stateMenu {
		t.Fatalf("expected stateMenu after dismissing title, got %v", m.state)
	}
	return m
}

func TestNewGameEntersGalaxyMap(t *testing.T) {
	openSave, listSaves, cleanup := newTestHooks(t)
	defer cleanup()

	m := New(openSave, listSaves)
	m = dismissTitle(t, m)
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
	m = dismissTitle(t, m)
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
	m2 = dismissTitle(t, m2)
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
	m = dismissTitle(t, m)
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
		m = dismissTitle(t, m)
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

func TestColonizeFoundsColonyFromMap(t *testing.T) {
	dir := t.TempDir()
	store, err := sqlite.Open(config.SavePath(dir, "alpha"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	openSave := func(name string) (*local.Client, error) {
		return local.New(engine.New(store)), nil
	}
	listSaves := func() ([]string, error) { return config.ListSaves(dir) }

	m := New(openSave, listSaves)
	m = dismissTitle(t, m)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "alpha")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}
	if m.colony.Exists {
		t.Fatal("expected no colony at the freshly-created starting system")
	}
	if !strings.Contains(m.View(), "A colony could be founded here") {
		t.Fatalf("expected the map to show no-colony hint, got:\n%s", m.View())
	}

	// Top up credits directly against the store so colonization is
	// affordable without needing to simulate a full trading session.
	ctx := context.Background()
	p, err := store.GetPlayer(ctx)
	if err != nil {
		t.Fatalf("get player: %v", err)
	}
	p.Credits = 10000
	if err := store.SavePlayer(ctx, p); err != nil {
		t.Fatalf("save player: %v", err)
	}
	m.player.Credits = p.Credits

	m = key(t, m, "p") // open the colonize commodity picker
	if m.state != stateColonize {
		t.Fatalf("expected stateColonize, got %v", m.state)
	}
	m = key(t, m, "enter") // confirm the first commodity (food)
	if m.state != stateMap {
		t.Fatalf("expected stateMap after founding a colony, got %v (err=%v)", m.state, m.err)
	}
	if !m.colony.Exists {
		t.Fatal("expected a colony to now exist at the current system")
	}
	if m.colony.Colony.Focus != "food" {
		t.Fatalf("expected focus food, got %s", m.colony.Colony.Focus)
	}
	if m.player.Credits >= 10000 {
		t.Fatal("expected credits to be spent founding the colony")
	}

	// Founding a second colony at an already-colonized system is guarded
	// client-side as a no-op.
	m2 := key(t, m, "p")
	if m2.state != stateMap {
		t.Fatalf("expected founding to be a no-op when a colony already exists, got state %v", m2.state)
	}

	// The colonies overview screen should list what was just founded.
	m = key(t, m, "o")
	if m.state != stateColonies {
		t.Fatalf("expected stateColonies, got %v (err=%v)", m.state, m.err)
	}
	if !strings.Contains(m.View(), "Food Rations") {
		t.Fatalf("expected the colonies screen to list the founded colony, got:\n%s", m.View())
	}

	m = key(t, m, "esc")
	if m.state != stateMap {
		t.Fatalf("expected esc to return to the map, got %v", m.state)
	}
}

func TestResearchTechFromMap(t *testing.T) {
	dir := t.TempDir()
	store, err := sqlite.Open(config.SavePath(dir, "alpha"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	openSave := func(name string) (*local.Client, error) {
		return local.New(engine.New(store)), nil
	}
	listSaves := func() ([]string, error) { return config.ListSaves(dir) }

	m := New(openSave, listSaves)
	m = dismissTitle(t, m)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "alpha")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}
	cargoBefore := m.player.CargoCapacity

	m = key(t, m, "r") // open the research screen
	if m.state != stateTechTree {
		t.Fatalf("expected stateTechTree, got %v (err=%v)", m.state, m.err)
	}
	if len(m.techCatalog) == 0 {
		t.Fatal("expected a non-empty tech catalog")
	}
	if m.techCatalog[0].ID != "cargo-1" {
		t.Fatalf("expected the first catalog entry to be cargo-1, got %s", m.techCatalog[0].ID)
	}
	if !strings.Contains(m.View(), "No active research project") {
		t.Fatalf("expected no active research yet, got:\n%s", m.View())
	}

	m = key(t, m, "enter") // start researching cargo-1 (cursor starts at 0)
	if m.state != stateTechTree {
		t.Fatalf("expected stateTechTree after starting research, got %v (err=%v)", m.state, m.err)
	}
	if m.research.Active != "cargo-1" {
		t.Fatalf("expected active research cargo-1, got %s", m.research.Active)
	}

	// Force the research clock far into the past so ticks have elapsed
	// well past cargo-1's cost, then reload the screen to observe it
	// complete and its effect land on the player.
	ctx := context.Background()
	research, err := store.GetResearch(ctx)
	if err != nil {
		t.Fatalf("get research: %v", err)
	}
	research.LastTickAt = research.LastTickAt.Add(-time.Hour)
	if err := store.SaveResearch(ctx, research); err != nil {
		t.Fatalf("save research: %v", err)
	}

	m = key(t, m, "esc")
	if m.state != stateMap {
		t.Fatalf("expected esc to return to the map, got %v", m.state)
	}
	m = key(t, m, "r") // reload the research screen, ticking the rewound clock
	if m.state != stateTechTree {
		t.Fatalf("expected stateTechTree, got %v (err=%v)", m.state, m.err)
	}
	if !m.research.HasUnlocked("cargo-1") {
		t.Fatal("expected cargo-1 to have completed")
	}
	if m.research.Active != "" {
		t.Fatalf("expected no active project after completion, got %s", m.research.Active)
	}
	wantCapacity := cargoBefore + 5 // cargo-1's magnitude
	if m.player.CargoCapacity != wantCapacity {
		t.Fatalf("expected cargo capacity %d, got %d", wantCapacity, m.player.CargoCapacity)
	}
	if !strings.Contains(m.View(), "unlocked") {
		t.Fatalf("expected the catalog to show cargo-1 as unlocked, got:\n%s", m.View())
	}

	m = key(t, m, "esc")
	if m.state != stateMap {
		t.Fatalf("expected esc to return to the map, got %v", m.state)
	}
}

func TestEspionageRecruitAndSendMissionFromMap(t *testing.T) {
	dir := t.TempDir()
	store, err := sqlite.Open(config.SavePath(dir, "alpha"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	openSave := func(name string) (*local.Client, error) {
		return local.New(engine.New(store)), nil
	}
	listSaves := func() ([]string, error) { return config.ListSaves(dir) }

	m := New(openSave, listSaves)
	m = dismissTitle(t, m)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "alpha")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}
	creditsBefore := m.player.Credits
	turnsBefore := m.player.Turns.Remaining

	m = key(t, m, "e") // open the espionage screen
	if m.state != stateEspionage {
		t.Fatalf("expected stateEspionage, got %v (err=%v)", m.state, m.err)
	}
	if len(m.spies) != 0 {
		t.Fatalf("expected no spies yet, got %d", len(m.spies))
	}
	if !strings.Contains(m.View(), "Recruit New Spy") {
		t.Fatalf("expected a recruit option, got:\n%s", m.View())
	}

	m = key(t, m, "enter") // cursor starts on the (only) recruit row
	if m.state != stateEspionage {
		t.Fatalf("expected stateEspionage after recruiting, got %v (err=%v)", m.state, m.err)
	}
	if len(m.spies) != 1 {
		t.Fatalf("expected 1 recruited spy, got %d", len(m.spies))
	}
	if m.player.Credits != creditsBefore-query.RecruitSpyCost {
		t.Fatalf("expected %d credits spent, got balance %d", query.RecruitSpyCost, m.player.Credits)
	}
	if m.player.Turns.Remaining != turnsBefore-query.RecruitSpyTurnCost {
		t.Fatalf("expected %d turns spent, got %d remaining", query.RecruitSpyTurnCost, m.player.Turns.Remaining)
	}

	// The cursor was left on row 0, which now holds the just-recruited spy.
	if m.espionageCursor != 0 {
		t.Fatalf("expected cursor on row 0, got %d", m.espionageCursor)
	}
	m = key(t, m, "enter") // select the spy, opening the target picker
	if m.state != stateEspionageTarget {
		t.Fatalf("expected stateEspionageTarget, got %v (err=%v)", m.state, m.err)
	}
	if len(m.galaxy.Nodes) == 0 {
		t.Fatal("expected galaxy nodes to be listed as targets")
	}

	turnsBeforeMission := m.player.Turns.Remaining
	m = key(t, m, "s") // send the selected spy on a steal mission against the first listed target
	if m.state != stateEspionageTarget {
		t.Fatalf("expected to remain on stateEspionageTarget after a mission, got %v (err=%v)", m.state, m.err)
	}
	if m.missionReport == "" {
		t.Fatal("expected a mission report after sending a mission")
	}
	if m.player.Turns.Remaining != turnsBeforeMission-query.SpyMissionTurnCost {
		t.Fatalf("expected %d turns spent on the mission, got %d remaining", query.SpyMissionTurnCost, m.player.Turns.Remaining)
	}
	if len(m.spies) != 1 || m.spies[0].MissionsRun != 1 {
		t.Fatalf("expected the spy's mission count to increment, got %+v", m.spies)
	}

	m = key(t, m, "esc")
	if m.state != stateEspionage {
		t.Fatalf("expected esc to return to the roster, got %v", m.state)
	}
	m = key(t, m, "esc")
	if m.state != stateMap {
		t.Fatalf("expected esc to return to the map, got %v", m.state)
	}
}

// flyUntilEncounter repeatedly flies to the first-listed neighbor until a
// hostile encounter is rolled, resetting mapCursor each hop so a shorter
// neighbor list at the new system can't leave it pointing out of range.
func flyUntilEncounter(t *testing.T, m Model) Model {
	t.Helper()
	for i := 0; i < 80; i++ {
		if len(m.galaxy.Neighbors(m.player.NodeID)) == 0 {
			t.Fatal("expected at least one warp lane from the current system")
		}
		m.mapCursor = 0
		m = key(t, m, "enter")
		if m.err != nil {
			t.Fatalf("unexpected error while hunting for an encounter: %v", m.err)
		}
		if m.state == stateEncounter {
			return m
		}
	}
	t.Fatal("expected a hostile encounter within 80 flights")
	return m
}

func TestHostileEncounterFightResolvesAndReturnsToMap(t *testing.T) {
	openSave, listSaves, cleanup := newTestHooks(t)
	defer cleanup()

	m := New(openSave, listSaves)
	m = dismissTitle(t, m)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "encounter-fight")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}

	m = flyUntilEncounter(t, m)
	if !strings.Contains(m.View(), "approaches") {
		t.Fatalf("expected an encounter prompt, got:\n%s", m.View())
	}

	m = key(t, m, "f") // fight
	if m.state != stateEncounter || !m.combatDone {
		t.Fatalf("expected a resolved encounter screen, got state=%v combatDone=%v (err=%v)", m.state, m.combatDone, m.err)
	}
	if !strings.Contains(m.View(), "Combat Report") {
		t.Fatalf("expected a combat report, got:\n%s", m.View())
	}
	if len(m.combatResult.Battle.Log) == 0 {
		t.Fatal("expected a non-empty battle log after fighting")
	}

	m = key(t, m, "enter") // acknowledge the report
	if m.state != stateMap {
		t.Fatalf("expected esc/enter to return to the map, got %v (err=%v)", m.state, m.err)
	}
}

func TestHostileEncounterFleeResolvesAndReturnsToMap(t *testing.T) {
	openSave, listSaves, cleanup := newTestHooks(t)
	defer cleanup()

	m := New(openSave, listSaves)
	m = dismissTitle(t, m)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "encounter-flee")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}

	m = flyUntilEncounter(t, m)

	m = key(t, m, "r") // attempt to flee
	if m.state != stateEncounter || !m.combatDone {
		t.Fatalf("expected a resolved encounter screen, got state=%v combatDone=%v (err=%v)", m.state, m.combatDone, m.err)
	}
	if !strings.Contains(m.View(), "enter to continue") {
		t.Fatalf("expected a combat report, got:\n%s", m.View())
	}

	m = key(t, m, "enter") // acknowledge the report
	if m.state != stateMap {
		t.Fatalf("expected esc/enter to return to the map, got %v (err=%v)", m.state, m.err)
	}
}

func TestTitleScreenShowsLoreAndSaveHintThenDismisses(t *testing.T) {
	openSave, listSaves, _ := newTestHooks(t)

	m := New(openSave, listSaves)
	if !strings.Contains(m.View(), "saves automatically") {
		t.Fatalf("expected the title screen to explain the save model, got:\n%s", m.View())
	}
	m = dismissTitle(t, m)
	if !strings.Contains(m.View(), "New Game") {
		t.Fatalf("expected the menu after dismissing the title, got:\n%s", m.View())
	}
}

func TestEscFromMapOffersResumeAndReturnsUnchanged(t *testing.T) {
	openSave, listSaves, cleanup := newTestHooks(t)
	defer cleanup()

	m := New(openSave, listSaves)
	m = dismissTitle(t, m)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "alpha")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}
	wantNode := m.player.NodeID

	m = key(t, m, "esc")
	if m.state != stateMenu {
		t.Fatalf("expected stateMenu after esc, got %v", m.state)
	}
	if !strings.Contains(m.View(), "Resume") {
		t.Fatalf("expected Resume to be offered once a game is loaded, got:\n%s", m.View())
	}

	m = key(t, m, "enter") // Resume is the default (first) selection
	if m.state != stateMap {
		t.Fatalf("expected Resume to return to stateMap, got %v", m.state)
	}
	if m.player.NodeID != wantNode {
		t.Fatalf("expected resuming to leave the player where they were, got %s want %s", m.player.NodeID, wantNode)
	}
}

func TestHelpScreenFromMapReturnsToMap(t *testing.T) {
	openSave, listSaves, cleanup := newTestHooks(t)
	defer cleanup()

	m := New(openSave, listSaves)
	m = dismissTitle(t, m)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "alpha")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}

	m = key(t, m, "?")
	if m.state != stateHelp {
		t.Fatalf("expected stateHelp, got %v", m.state)
	}
	if !strings.Contains(m.View(), "warp lanes") {
		t.Fatalf("expected map help text, got:\n%s", m.View())
	}

	m = key(t, m, "x") // any key dismisses help
	if m.state != stateMap {
		t.Fatalf("expected help to return to stateMap, got %v", m.state)
	}
}

func TestUnexploredLaneLabelDoesNotUseQuestionMarks(t *testing.T) {
	openSave, listSaves, cleanup := newTestHooks(t)
	defer cleanup()

	m := New(openSave, listSaves)
	m = dismissTitle(t, m)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "alpha")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}

	view := m.View()
	if strings.Contains(view, "???") {
		t.Fatalf("expected no ??? placeholder for unexplored lanes, got:\n%s", view)
	}
	if !strings.Contains(view, "unexplored") {
		t.Fatalf("expected an unexplored-lane label, got:\n%s", view)
	}
}

func TestColonyHintReflectsAffordability(t *testing.T) {
	dir := t.TempDir()
	store, err := sqlite.Open(config.SavePath(dir, "alpha"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	openSave := func(name string) (*local.Client, error) {
		return local.New(engine.New(store)), nil
	}
	listSaves := func() ([]string, error) { return config.ListSaves(dir) }

	m := New(openSave, listSaves)
	m = dismissTitle(t, m)
	m = key(t, m, "enter") // New Game
	m = typeString(t, m, "alpha")
	m = key(t, m, "enter")
	if m.state != stateMap {
		t.Fatalf("expected stateMap, got %v (err=%v)", m.state, m.err)
	}
	if !strings.Contains(m.View(), "short by") {
		t.Fatalf("expected the unaffordable colony hint to note the shortfall, got:\n%s", m.View())
	}

	ctx := context.Background()
	p, err := store.GetPlayer(ctx)
	if err != nil {
		t.Fatalf("get player: %v", err)
	}
	p.Credits = 10000
	if err := store.SavePlayer(ctx, p); err != nil {
		t.Fatalf("save player: %v", err)
	}
	m.player.Credits = p.Credits

	if !strings.Contains(m.View(), "whenever you're ready") {
		t.Fatalf("expected the affordable colony hint once credits suffice, got:\n%s", m.View())
	}
}
