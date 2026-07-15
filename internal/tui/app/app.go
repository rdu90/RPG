// Package app is the bubbletea root model: a title screen that creates or
// loads a save, then drives the galaxy map and trade screens that make up
// the M1 core loop (fly between systems, buy/sell at static prices). This
// package only ever imports internal/transport, never internal/engine
// directly.
package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/rdu90/RPG/internal/transport/command"
	"github.com/rdu90/RPG/internal/transport/local"
	"github.com/rdu90/RPG/internal/transport/query"
	"github.com/rdu90/RPG/internal/tui/companion"
	"github.com/rdu90/RPG/internal/tui/style"
)

// OpenSave opens (creating if necessary) the save with the given name and
// returns a transport client bound to it. The caller supplies this so the
// TUI never needs to know about file paths or persistence technology.
type OpenSave func(name string) (*local.Client, error)

// ListSaves returns the names of existing saves.
type ListSaves func() ([]string, error)

type state int

const (
	stateTitle state = iota
	stateMenu
	stateNewGameInput
	stateLoadList
	stateWorking
	stateMap
	stateTrade
	stateHaggle
	stateColonize
	stateColonies
	stateTechTree
	stateEspionage
	stateEspionageTarget
	stateEncounter
	stateHelp
	stateError
)

type tradeMode int

const (
	tradeIdle tradeMode = iota
	tradeBuying
	tradeSelling
)

type gameReadyMsg struct {
	client *local.Client
	game   query.Game
	err    error
}

type savesLoadedMsg struct {
	saves []string
	err   error
}

type worldLoadedMsg struct {
	galaxy  query.Galaxy
	player  query.Player
	anomaly query.AnomalyStatus
	colony  query.ColonyStatus
	err     error
}

type playerUpdatedMsg struct {
	player    query.Player
	anomaly   query.AnomalyStatus
	colony    query.ColonyStatus
	encounter *query.Hostile
	err       error
}

type combatResolvedMsg struct {
	result query.CombatResult
	err    error
}

type scoutedMsg struct {
	result query.ScoutResult
	err    error
}

type anomalyClaimedMsg struct {
	result query.ClaimAnomalyResult
	err    error
}

type colonizedMsg struct {
	result query.ColonizeResult
	err    error
}

type coloniesLoadedMsg struct {
	colonies []query.Colony
	err      error
}

type techTreeLoadedMsg struct {
	status query.TechTreeStatus
	err    error
}

type researchStartedMsg struct {
	result query.StartResearchResult
	err    error
}

type spiesLoadedMsg struct {
	spies []query.Spy
	err   error
}

type spyRecruitedMsg struct {
	result query.RecruitSpyResult
	err    error
}

type missionSentMsg struct {
	result query.MissionResult
	err    error
}

type marketLoadedMsg struct {
	prices []query.Price
	err    error
}

type haggleUpdatedMsg struct {
	result query.HaggleResult
	err    error
}

// Model is the root bubbletea model.
type Model struct {
	openSave  OpenSave
	listSaves ListSaves

	state state
	// afterWork is where a playerUpdatedMsg should land: stateMap after a
	// move, stateTrade after a buy/sell.
	afterWork state
	// helpFrom is where "?" was pressed from, so stateHelp knows where to
	// return.
	helpFrom state

	menuCursor int

	nameInput  textinput.Model
	qtyInput   textinput.Model
	priceInput textinput.Model

	saves      []string
	loadCursor int

	client *local.Client
	game   query.Game
	err    error

	galaxy      query.Galaxy
	player      query.Player
	mapCursor   int
	anomaly     query.AnomalyStatus
	scoutReport string

	colony         query.ColonyStatus
	colonizeCursor int
	colonizeErr    error
	colonies       []query.Colony

	research    query.Research
	techCatalog []query.Tech
	techCursor  int

	spies           []query.Spy
	espionageCursor int
	espionageSpy    query.Spy
	targetCursor    int
	missionReport   string

	encounterHostile query.Hostile
	combatResult     query.CombatResult
	combatDone       bool

	market      []query.Price
	tradeCursor int
	tradeMode   tradeMode
	tradeErr    error

	haggleSession  query.HaggleSession
	haggleOffering bool
	haggleErr      error
}

// New builds the root model.
func New(openSave OpenSave, listSaves ListSaves) Model {
	ti := textinput.New()
	ti.Placeholder = "save name"
	ti.CharLimit = 40
	ti.Focus()

	qty := textinput.New()
	qty.Placeholder = "quantity"
	qty.CharLimit = 6

	price := textinput.New()
	price.Placeholder = "price per unit"
	price.CharLimit = 6

	return Model{
		openSave:   openSave,
		listSaves:  listSaves,
		state:      stateTitle,
		nameInput:  ti,
		qtyInput:   qty,
		priceInput: price,
	}
}

// menuOptions returns the main menu's items, offering "Resume" ahead of
// "New Game"/"Load Game" whenever a save is already loaded — the menu is
// reachable mid-game (e.g. via esc from the map) and shouldn't strand the
// player with no way back to it.
func (m Model) menuOptions() []string {
	if m.client != nil {
		return []string{"Resume", "New Game", "Load Game", "Quit"}
	}
	return []string{"New Game", "Load Game", "Quit"}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		return m.handleKey(msg)
	case savesLoadedMsg:
		m.saves = msg.saves
		m.loadCursor = 0
		m.err = msg.err
		if msg.err != nil {
			m.state = stateError
		}
		return m, nil
	case gameReadyMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.client = msg.client
		m.game = msg.game
		m.state = stateWorking
		return m, loadWorldCmd(msg.client)
	case worldLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.galaxy = msg.galaxy
		m.player = msg.player
		m.anomaly = msg.anomaly
		m.colony = msg.colony
		m.mapCursor = 0
		m.state = stateMap
		return m, nil
	case playerUpdatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.player = msg.player
		m.anomaly = msg.anomaly
		m.colony = msg.colony
		m.scoutReport = ""
		if msg.encounter != nil {
			m.encounterHostile = *msg.encounter
			m.combatDone = false
			m.state = stateEncounter
			return m, nil
		}
		m.state = m.afterWork
		return m, nil
	case combatResolvedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.player = msg.result.Player
		m.combatResult = msg.result
		m.combatDone = true
		m.state = stateEncounter
		return m, nil
	case scoutedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.player = msg.result.Player
		m.scoutReport = describeScoutResult(msg.result)
		m.state = stateMap
		return m, nil
	case anomalyClaimedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.player = msg.result.Player
		m.anomaly = query.AnomalyStatus{Anomaly: msg.result.Anomaly, Claimed: true}
		m.state = stateMap
		return m, nil
	case colonizedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.player = msg.result.Player
		m.colony = query.ColonyStatus{Exists: true, Colony: msg.result.Colony}
		m.state = stateMap
		return m, nil
	case coloniesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.colonies = msg.colonies
		m.state = stateColonies
		return m, nil
	case techTreeLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.techCatalog = msg.status.Catalog
		m.research = msg.status.Research
		m.player = msg.status.Player
		m.techCursor = 0
		m.state = stateTechTree
		return m, nil
	case researchStartedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.research = msg.result.Research
		m.player = msg.result.Player
		m.state = stateTechTree
		return m, nil
	case spiesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.spies = msg.spies
		m.espionageCursor = 0
		m.state = stateEspionage
		return m, nil
	case spyRecruitedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.spies = append(m.spies, msg.result.Spy)
		m.player = msg.result.Player
		m.state = stateEspionage
		return m, nil
	case missionSentMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		for i, s := range m.spies {
			if s.ID == msg.result.Spy.ID {
				m.spies[i] = msg.result.Spy
				break
			}
		}
		m.player = msg.result.Player
		m.missionReport = describeMissionResult(msg.result)
		m.state = stateEspionageTarget
		return m, nil
	case marketLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.market = msg.prices
		m.tradeCursor = 0
		m.state = stateTrade
		return m, nil
	case haggleUpdatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateError
			return m, nil
		}
		m.haggleSession = msg.result.Session
		m.player = msg.result.Player
		m.state = stateHaggle
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateTitle:
		m.state = stateMenu
		return m, nil
	case stateMenu:
		return m.handleMenuKey(msg)
	case stateNewGameInput:
		return m.handleNewGameKey(msg)
	case stateLoadList:
		return m.handleLoadListKey(msg)
	case stateMap:
		return m.handleMapKey(msg)
	case stateTrade:
		return m.handleTradeKey(msg)
	case stateHaggle:
		return m.handleHaggleKey(msg)
	case stateColonize:
		return m.handleColonizeKey(msg)
	case stateColonies:
		return m.handleColoniesKey(msg)
	case stateTechTree:
		return m.handleTechTreeKey(msg)
	case stateEspionage:
		return m.handleEspionageKey(msg)
	case stateEspionageTarget:
		return m.handleEspionageTargetKey(msg)
	case stateEncounter:
		return m.handleEncounterKey(msg)
	case stateHelp:
		m.state = m.helpFrom
		return m, nil
	case stateError:
		switch msg.String() {
		case "esc":
			m.state = stateMenu
			m.err = nil
			return m, nil
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	options := m.menuOptions()
	switch msg.String() {
	case "?":
		m.helpFrom = stateMenu
		m.state = stateHelp
		return m, nil
	case "esc":
		if m.client != nil {
			m.state = stateMap
		}
		return m, nil
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(options)-1 {
			m.menuCursor++
		}
	case "enter":
		switch options[m.menuCursor] {
		case "Resume":
			m.state = stateMap
		case "New Game":
			m.state = stateNewGameInput
			m.nameInput.SetValue("")
			m.nameInput.Focus()
		case "Load Game":
			m.state = stateLoadList
			return m, m.loadSavesCmd()
		case "Quit":
			return m, tea.Quit
		}
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleNewGameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.state = stateMenu
		return m, nil
	case tea.KeyEnter:
		name := m.nameInput.Value()
		if name == "" {
			return m, nil
		}
		m.state = stateWorking
		return m, m.createGameCmd(name)
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m Model) handleLoadListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.helpFrom = stateLoadList
		m.state = stateHelp
		return m, nil
	case "esc":
		m.state = stateMenu
		return m, nil
	case "up", "k":
		if m.loadCursor > 0 {
			m.loadCursor--
		}
	case "down", "j":
		if m.loadCursor < len(m.saves)-1 {
			m.loadCursor++
		}
	case "enter":
		if len(m.saves) == 0 {
			return m, nil
		}
		name := m.saves[m.loadCursor]
		m.state = stateWorking
		return m, m.loadGameCmd(name)
	}
	return m, nil
}

func (m Model) handleMapKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	neighbors := m.galaxy.Neighbors(m.player.NodeID)
	switch msg.String() {
	case "?":
		m.helpFrom = stateMap
		m.state = stateHelp
		return m, nil
	case "esc":
		m.state = stateMenu
		m.menuCursor = 0
		return m, nil
	case "up", "k":
		if m.mapCursor > 0 {
			m.mapCursor--
		}
	case "down", "j":
		if m.mapCursor < len(neighbors)-1 {
			m.mapCursor++
		}
	case "enter":
		if len(neighbors) == 0 {
			return m, nil
		}
		to := neighbors[m.mapCursor].To
		m.afterWork = stateMap
		m.state = stateWorking
		return m, m.moveCmd(to)
	case "x":
		if len(neighbors) == 0 {
			return m, nil
		}
		to := neighbors[m.mapCursor].To
		if m.player.HasDiscovered(to) {
			return m, nil
		}
		m.state = stateWorking
		return m, m.scoutCmd(to)
	case "c":
		if m.anomaly.Anomaly.Empty() || m.anomaly.Claimed {
			return m, nil
		}
		m.state = stateWorking
		return m, m.claimAnomalyCmd()
	case "t":
		m.state = stateWorking
		return m, m.loadMarketCmd()
	case "p":
		if m.colony.Exists {
			return m, nil
		}
		m.colonizeCursor = 0
		m.colonizeErr = nil
		m.state = stateColonize
		return m, nil
	case "o":
		m.state = stateWorking
		return m, m.loadColoniesCmd()
	case "r":
		m.state = stateWorking
		return m, m.loadTechTreeCmd()
	case "e":
		m.state = stateWorking
		return m, m.loadSpiesCmd()
	case "h":
		if !m.player.Ship.Damaged() {
			return m, nil
		}
		m.afterWork = stateMap
		m.state = stateWorking
		return m, m.repairShipCmd()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleColonizeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.helpFrom = stateColonize
		m.state = stateHelp
		return m, nil
	case "esc":
		m.state = stateMap
		return m, nil
	case "up", "k":
		if m.colonizeCursor > 0 {
			m.colonizeCursor--
		}
	case "down", "j":
		if m.colonizeCursor < len(query.Commodities)-1 {
			m.colonizeCursor++
		}
	case "enter":
		focus := query.Commodities[m.colonizeCursor].ID
		m.state = stateWorking
		return m, m.colonizeCmd(focus)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleColoniesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.helpFrom = stateColonies
		m.state = stateHelp
		return m, nil
	case "esc", "enter":
		m.state = stateMap
		return m, nil
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleTechTreeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.helpFrom = stateTechTree
		m.state = stateHelp
		return m, nil
	case "esc":
		m.state = stateMap
		return m, nil
	case "up", "k":
		if m.techCursor > 0 {
			m.techCursor--
		}
	case "down", "j":
		if m.techCursor < len(m.techCatalog)-1 {
			m.techCursor++
		}
	case "enter":
		if len(m.techCatalog) == 0 {
			return m, nil
		}
		tech := m.techCatalog[m.techCursor]
		if tech.ID == m.research.Active || !m.research.Available(tech.ID) {
			return m, nil
		}
		m.state = stateWorking
		return m, m.startResearchCmd(tech.ID)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleEspionageKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := len(m.spies) + 1 // +1 for the "Recruit New Spy" row
	switch msg.String() {
	case "?":
		m.helpFrom = stateEspionage
		m.state = stateHelp
		return m, nil
	case "esc":
		m.state = stateMap
		return m, nil
	case "up", "k":
		if m.espionageCursor > 0 {
			m.espionageCursor--
		}
	case "down", "j":
		if m.espionageCursor < rows-1 {
			m.espionageCursor++
		}
	case "enter":
		if m.espionageCursor == len(m.spies) {
			m.state = stateWorking
			return m, m.recruitSpyCmd()
		}
		spy := m.spies[m.espionageCursor]
		if spy.Status != query.SpyAvailable {
			return m, nil
		}
		m.espionageSpy = spy
		m.targetCursor = 0
		m.missionReport = ""
		m.state = stateEspionageTarget
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleEspionageTargetKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?":
		m.helpFrom = stateEspionageTarget
		m.state = stateHelp
		return m, nil
	case "esc":
		m.state = stateEspionage
		return m, nil
	case "up", "k":
		if m.targetCursor > 0 {
			m.targetCursor--
		}
	case "down", "j":
		if m.targetCursor < len(m.galaxy.Nodes)-1 {
			m.targetCursor++
		}
	case "s", "a", "i":
		if len(m.galaxy.Nodes) == 0 {
			return m, nil
		}
		mission := query.MissionSteal
		switch msg.String() {
		case "a":
			mission = query.MissionSabotage
		case "i":
			mission = query.MissionIntel
		}
		target := m.galaxy.Nodes[m.targetCursor].ID
		m.state = stateWorking
		return m, m.sendSpyMissionCmd(m.espionageSpy.ID, target, mission)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleEncounterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.combatDone {
		switch msg.String() {
		case "enter", "esc":
			m.combatDone = false
			m.state = stateMap
			return m, nil
		case "q":
			return m, tea.Quit
		}
		return m, nil
	}
	switch msg.String() {
	case "?":
		m.helpFrom = stateEncounter
		m.state = stateHelp
		return m, nil
	case "f":
		m.state = stateWorking
		return m, m.resolveEncounterCmd(m.encounterHostile, false)
	case "r":
		m.state = stateWorking
		return m, m.resolveEncounterCmd(m.encounterHostile, true)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleTradeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.tradeMode != tradeIdle {
		switch msg.Type {
		case tea.KeyEsc:
			m.tradeMode = tradeIdle
			m.tradeErr = nil
			m.qtyInput.SetValue("")
			m.qtyInput.Blur()
			return m, nil
		case tea.KeyEnter:
			qty, err := strconv.Atoi(strings.TrimSpace(m.qtyInput.Value()))
			if err != nil || qty <= 0 {
				m.tradeErr = fmt.Errorf("enter a positive whole number")
				return m, nil
			}
			commodity := m.market[m.tradeCursor].CommodityID
			buy := m.tradeMode == tradeBuying
			m.tradeMode = tradeIdle
			m.tradeErr = nil
			m.qtyInput.SetValue("")
			m.qtyInput.Blur()
			m.state = stateWorking
			return m, m.startHaggleCmd(commodity, qty, buy)
		}
		var cmd tea.Cmd
		m.qtyInput, cmd = m.qtyInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "?":
		m.helpFrom = stateTrade
		m.state = stateHelp
		return m, nil
	case "esc":
		m.state = stateMap
		return m, nil
	case "up", "k":
		if m.tradeCursor > 0 {
			m.tradeCursor--
		}
	case "down", "j":
		if m.tradeCursor < len(m.market)-1 {
			m.tradeCursor++
		}
	case "b":
		if len(m.market) == 0 {
			return m, nil
		}
		m.tradeMode = tradeBuying
		m.tradeErr = nil
		m.qtyInput.SetValue("")
		m.qtyInput.Focus()
	case "s":
		if len(m.market) == 0 {
			return m, nil
		}
		m.tradeMode = tradeSelling
		m.tradeErr = nil
		m.qtyInput.SetValue("")
		m.qtyInput.Focus()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleHaggleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.haggleSession.Outcome != query.HaggleInProgress {
		switch msg.String() {
		case "esc", "enter":
			m.state = stateTrade
			return m, nil
		}
		return m, nil
	}

	if m.haggleOffering {
		switch msg.Type {
		case tea.KeyEsc:
			m.haggleOffering = false
			m.haggleErr = nil
			m.priceInput.SetValue("")
			m.priceInput.Blur()
			return m, nil
		case tea.KeyEnter:
			price, err := strconv.Atoi(strings.TrimSpace(m.priceInput.Value()))
			if err != nil || price <= 0 {
				m.haggleErr = fmt.Errorf("enter a positive whole number")
				return m, nil
			}
			m.haggleOffering = false
			m.haggleErr = nil
			m.priceInput.SetValue("")
			m.priceInput.Blur()
			m.state = stateWorking
			return m, m.haggleOfferCmd(price)
		}
		var cmd tea.Cmd
		m.priceInput, cmd = m.priceInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "?":
		m.helpFrom = stateHaggle
		m.state = stateHelp
		return m, nil
	case "o":
		m.haggleOffering = true
		m.haggleErr = nil
		m.priceInput.SetValue("")
		m.priceInput.Focus()
	case "w":
		m.state = stateWorking
		return m, m.haggleWalkAwayCmd()
	case "a":
		m.state = stateWorking
		return m, m.haggleAcceptCmd()
	case "esc":
		m.state = stateTrade
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) loadSavesCmd() tea.Cmd {
	listSaves := m.listSaves
	return func() tea.Msg {
		saves, err := listSaves()
		return savesLoadedMsg{saves: saves, err: err}
	}
}

func (m Model) createGameCmd(name string) tea.Cmd {
	openSave := m.openSave
	return func() tea.Msg {
		client, err := openSave(name)
		if err != nil {
			return gameReadyMsg{err: err}
		}
		res, err := client.Execute(context.Background(), command.CreateGame{Name: name})
		if err != nil {
			return gameReadyMsg{err: err}
		}
		return gameReadyMsg{client: client, game: res.(query.Game)}
	}
}

func (m Model) loadGameCmd(name string) tea.Cmd {
	openSave := m.openSave
	return func() tea.Msg {
		client, err := openSave(name)
		if err != nil {
			return gameReadyMsg{err: err}
		}
		res, err := client.Query(context.Background(), query.GetGame{})
		if err != nil {
			return gameReadyMsg{err: err}
		}
		return gameReadyMsg{client: client, game: res.(query.Game)}
	}
}

func loadWorldCmd(client *local.Client) tea.Cmd {
	return func() tea.Msg {
		gRes, err := client.Query(context.Background(), query.GetGalaxy{})
		if err != nil {
			return worldLoadedMsg{err: err}
		}
		pRes, err := client.Query(context.Background(), query.GetPlayer{})
		if err != nil {
			return worldLoadedMsg{err: err}
		}
		aRes, err := client.Query(context.Background(), query.GetAnomaly{})
		if err != nil {
			return worldLoadedMsg{err: err}
		}
		cRes, err := client.Query(context.Background(), query.GetColony{})
		if err != nil {
			return worldLoadedMsg{err: err}
		}
		return worldLoadedMsg{
			galaxy:  gRes.(query.Galaxy),
			player:  pRes.(query.Player),
			anomaly: aRes.(query.AnomalyStatus),
			colony:  cRes.(query.ColonyStatus),
		}
	}
}

func (m Model) moveCmd(to query.NodeID) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.Move{To: to})
		if err != nil {
			return playerUpdatedMsg{err: err}
		}
		result := res.(query.MoveResult)
		aRes, err := client.Query(context.Background(), query.GetAnomaly{})
		if err != nil {
			return playerUpdatedMsg{err: err}
		}
		cRes, err := client.Query(context.Background(), query.GetColony{})
		if err != nil {
			return playerUpdatedMsg{err: err}
		}
		return playerUpdatedMsg{
			player:    result.Player,
			anomaly:   aRes.(query.AnomalyStatus),
			colony:    cRes.(query.ColonyStatus),
			encounter: result.Encounter,
		}
	}
}

func (m Model) resolveEncounterCmd(hostile query.Hostile, flee bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.ResolveEncounter{Hostile: hostile, Flee: flee})
		if err != nil {
			return combatResolvedMsg{err: err}
		}
		return combatResolvedMsg{result: res.(query.CombatResult)}
	}
}

func (m Model) repairShipCmd() tea.Cmd {
	client := m.client
	anomaly := m.anomaly
	colony := m.colony
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.RepairShip{})
		if err != nil {
			return playerUpdatedMsg{err: err}
		}
		return playerUpdatedMsg{player: res.(query.Player), anomaly: anomaly, colony: colony}
	}
}

func (m Model) scoutCmd(to query.NodeID) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.ScoutNode{To: to})
		if err != nil {
			return scoutedMsg{err: err}
		}
		return scoutedMsg{result: res.(query.ScoutResult)}
	}
}

func (m Model) claimAnomalyCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.ClaimAnomaly{})
		if err != nil {
			return anomalyClaimedMsg{err: err}
		}
		return anomalyClaimedMsg{result: res.(query.ClaimAnomalyResult)}
	}
}

func (m Model) colonizeCmd(focus query.CommodityID) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.Colonize{Focus: focus})
		if err != nil {
			return colonizedMsg{err: err}
		}
		return colonizedMsg{result: res.(query.ColonizeResult)}
	}
}

func (m Model) loadColoniesCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Query(context.Background(), query.GetColonies{})
		if err != nil {
			return coloniesLoadedMsg{err: err}
		}
		return coloniesLoadedMsg{colonies: res.([]query.Colony)}
	}
}

func (m Model) loadTechTreeCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Query(context.Background(), query.GetTechTree{})
		if err != nil {
			return techTreeLoadedMsg{err: err}
		}
		return techTreeLoadedMsg{status: res.(query.TechTreeStatus)}
	}
}

func (m Model) startResearchCmd(tech query.TechID) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.StartResearch{Tech: tech})
		if err != nil {
			return researchStartedMsg{err: err}
		}
		return researchStartedMsg{result: res.(query.StartResearchResult)}
	}
}

func (m Model) loadSpiesCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Query(context.Background(), query.GetSpies{})
		if err != nil {
			return spiesLoadedMsg{err: err}
		}
		return spiesLoadedMsg{spies: res.([]query.Spy)}
	}
}

func (m Model) recruitSpyCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.RecruitSpy{})
		if err != nil {
			return spyRecruitedMsg{err: err}
		}
		return spyRecruitedMsg{result: res.(query.RecruitSpyResult)}
	}
}

func (m Model) sendSpyMissionCmd(spy string, target query.NodeID, mission query.MissionKind) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.SendSpyMission{Spy: spy, Target: target, Mission: mission})
		if err != nil {
			return missionSentMsg{err: err}
		}
		return missionSentMsg{result: res.(query.MissionResult)}
	}
}

// describeMissionResult renders a one-line report of a resolved spy mission.
func describeMissionResult(res query.MissionResult) string {
	if res.Outcome.Captured {
		return fmt.Sprintf("Mission failed — %s was captured!", res.Spy.Name)
	}
	if !res.Outcome.Success {
		return fmt.Sprintf("Mission failed — %s made it back safely.", res.Spy.Name)
	}
	switch res.Mission {
	case query.MissionSteal:
		return fmt.Sprintf("Success! %s stole %d credits.", res.Spy.Name, res.CreditsStolen)
	case query.MissionSabotage:
		return fmt.Sprintf("Success! %s sabotaged the local market.", res.Spy.Name)
	case query.MissionIntel:
		return fmt.Sprintf("Success! %s beamed back intel on the target system.", res.Spy.Name)
	default:
		return "Mission complete."
	}
}

// describeCombatResult renders a one-line report of a resolved encounter.
func describeCombatResult(res query.CombatResult) string {
	if res.Fled {
		return fmt.Sprintf("You evaded the %s and slipped away.", res.Hostile.Name)
	}
	switch res.Battle.Outcome {
	case query.CombatVictory:
		return fmt.Sprintf("Victory! You destroyed the %s and salvaged %d credits.", res.Hostile.Name, res.CreditsGained)
	case query.CombatDefeat:
		return fmt.Sprintf("Defeat! Your ship was disabled — the %s made off with %d credits and some cargo.", res.Hostile.Name, res.CreditsLost)
	case query.CombatDisengaged:
		return fmt.Sprintf("The %s broke off the attack.", res.Hostile.Name)
	default:
		return "The encounter is over."
	}
}

// describeScoutResult renders a one-line report of what a scout found (or
// didn't) at the surveyed system.
func describeScoutResult(res query.ScoutResult) string {
	if res.Anomaly.Empty() {
		return "Scout report: nothing of interest detected."
	}
	return fmt.Sprintf("Scout report: sensors detect a %s!", res.Anomaly.Kind)
}

func (m Model) loadMarketCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Query(context.Background(), query.GetMarket{})
		if err != nil {
			return marketLoadedMsg{err: err}
		}
		return marketLoadedMsg{prices: res.([]query.Price)}
	}
}

func (m Model) startHaggleCmd(commodity query.CommodityID, qty int, buy bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.StartHaggle{Commodity: commodity, Quantity: qty, Buying: buy})
		if err != nil {
			return haggleUpdatedMsg{err: err}
		}
		return haggleUpdatedMsg{result: res.(query.HaggleResult)}
	}
}

func (m Model) haggleOfferCmd(price int) tea.Cmd {
	client, session := m.client, m.haggleSession
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.HaggleOffer{Session: session, Price: price})
		if err != nil {
			return haggleUpdatedMsg{err: err}
		}
		return haggleUpdatedMsg{result: res.(query.HaggleResult)}
	}
}

func (m Model) haggleWalkAwayCmd() tea.Cmd {
	client, session := m.client, m.haggleSession
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.HaggleWalkAway{Session: session})
		if err != nil {
			return haggleUpdatedMsg{err: err}
		}
		return haggleUpdatedMsg{result: res.(query.HaggleResult)}
	}
}

func (m Model) haggleAcceptCmd() tea.Cmd {
	client, session := m.client, m.haggleSession
	return func() tea.Msg {
		res, err := client.Execute(context.Background(), command.HaggleAccept{Session: session})
		if err != nil {
			return haggleUpdatedMsg{err: err}
		}
		return haggleUpdatedMsg{result: res.(query.HaggleResult)}
	}
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.state {
	case stateTitle:
		return m.viewTitle()
	case stateMenu:
		return m.viewMenu()
	case stateNewGameInput:
		return m.viewNewGameInput()
	case stateLoadList:
		return m.viewLoadList()
	case stateWorking:
		return style.Title.Render("RPG") + "\n\nWorking...\n"
	case stateMap:
		return m.viewMap()
	case stateTrade:
		return m.viewTrade()
	case stateHaggle:
		return m.viewHaggle()
	case stateColonize:
		return m.viewColonize()
	case stateColonies:
		return m.viewColonies()
	case stateTechTree:
		return m.viewTechTree()
	case stateEspionage:
		return m.viewEspionage()
	case stateEspionageTarget:
		return m.viewEspionageTarget()
	case stateEncounter:
		return m.viewEncounter()
	case stateHelp:
		return m.viewHelp()
	case stateError:
		return m.viewError()
	}
	return ""
}

// saveHint tells the player how saving works, since there's no explicit
// save command to discover on their own.
const saveHint = "Progress saves automatically after every action — there's no separate save command."

func (m Model) viewTitle() string {
	s := style.Title.Render("RPG") + "\n\n"
	s += "A ragtag band of survivors, scattered across the stars.\n\n"
	s += "Explore uncharted systems, expand your reach with colonies, exploit the\n"
	s += "galaxy's economy and technology, and — if it comes to it — exterminate\n"
	s += "whatever stands between you and a galaxy-spanning empire.\n\n"
	s += style.Faint.Render(saveHint) + "\n\n"
	s += style.Faint.Render("press any key to begin")
	return s
}

func (m Model) viewMenu() string {
	s := style.Title.Render("RPG") + "\n\n"
	for i, item := range m.menuOptions() {
		if i == m.menuCursor {
			s += style.Selected.Render("> "+item) + "\n"
		} else {
			s += "  " + item + "\n"
		}
	}
	if m.client != nil {
		s += "\n" + style.Faint.Render("up/down to move, enter to select, esc to resume, ? for help, q to quit")
	} else {
		s += "\n" + style.Faint.Render("up/down to move, enter to select, ? for help, q to quit")
	}
	return s
}

func (m Model) viewNewGameInput() string {
	s := style.Title.Render("New Game") + "\n\n"
	s += "Save name: " + m.nameInput.View() + "\n\n"
	s += style.Faint.Render(saveHint) + "\n\n"
	s += style.Faint.Render("enter to confirm, esc to cancel")
	return s
}

func (m Model) viewLoadList() string {
	s := style.Title.Render("Load Game") + "\n\n"
	if len(m.saves) == 0 {
		s += style.Faint.Render("no saves found") + "\n"
	}
	for i, name := range m.saves {
		if i == m.loadCursor {
			s += style.Selected.Render("> "+name) + "\n"
		} else {
			s += "  " + name + "\n"
		}
	}
	s += "\n" + style.Faint.Render(saveHint) + "\n"
	s += style.Faint.Render("up/down to move, enter to load, esc to cancel, ? for help")
	return s
}

func (m Model) viewMap() string {
	s := style.Title.Render("RPG — "+m.game.Name) + "\n\n"

	cur, _ := m.galaxy.Node(m.player.NodeID)
	s += fmt.Sprintf("System: %s (dev %d)   Credits: %d cr   Turns: %d/%d   Cargo: %d/%d   Hull: %d/%d\n\n",
		cur.Name, cur.DevelopmentLevel, m.player.Credits,
		m.player.Turns.Remaining, m.player.Turns.Max,
		m.player.CargoUsed(), m.player.CargoCapacity,
		m.player.Ship.Hull, m.player.Ship.MaxHull)
	if m.player.Ship.Damaged() {
		s += style.Faint.Render(fmt.Sprintf("Hull damaged. Press h to repair (%d cr/point).", query.RepairCostPerHull)) + "\n\n"
	}

	if !m.anomaly.Anomaly.Empty() {
		if m.anomaly.Claimed {
			s += style.Faint.Render(companion.AlreadyInvestigated(m.anomaly.Anomaly.Kind)) + "\n\n"
		} else {
			s += style.Selected.Render(fmt.Sprintf("Sensors detect a %s here! Press c to investigate.", m.anomaly.Anomaly.Kind)) + "\n\n"
		}
	}

	if m.colony.Exists {
		c, _ := query.FindCommodity(m.colony.Colony.Focus)
		cap := query.ColonyPopulationCap(cur.DevelopmentLevel)
		s += style.Faint.Render(fmt.Sprintf("Colony here: population %d/%d, producing %s.", m.colony.Colony.Population, cap, c.Name)) + "\n\n"
	} else {
		cost := query.ColonizeCost(cur.DevelopmentLevel)
		shortfall := cost - m.player.Credits
		if shortfall < 0 {
			shortfall = 0
		}
		s += style.Faint.Render(companion.ColonyHint(m.player.Credits >= cost, cost, query.ColonizeTurnCost, shortfall)) + "\n\n"
	}

	if m.scoutReport != "" {
		s += style.Faint.Render(m.scoutReport) + "\n\n"
	}

	neighbors := m.galaxy.Neighbors(m.player.NodeID)
	var target query.NodeID
	if len(neighbors) > 0 {
		target = neighbors[m.mapCursor].To
	}
	visible := visibleNodes(m.galaxy, m.player.Discovered)
	s += renderStarfield(m.galaxy, m.player.NodeID, target, m.player.Discovered, visible) + "\n"

	s += style.Faint.Render("Warp lanes from here:") + "\n"
	if len(neighbors) == 0 {
		s += style.Faint.Render("  (no warp lanes connect this system!)") + "\n"
	}
	for i, e := range neighbors {
		var line string
		if m.player.HasDiscovered(e.To) {
			n, _ := m.galaxy.Node(e.To)
			line = fmt.Sprintf("%s (%d turn%s, dev %d)", n.Name, e.TurnCost, plural(e.TurnCost), n.DevelopmentLevel)
		} else {
			line = fmt.Sprintf("(unexplored, %d turn%s — x to scout)", e.TurnCost, plural(e.TurnCost))
		}
		if i == m.mapCursor {
			s += style.Selected.Render("> "+line) + "\n"
		} else {
			s += "  " + line + "\n"
		}
	}

	s += "\n" + style.Faint.Render("up/down select, enter to fly, x to scout, t to trade, p to found colony, o for colonies, r for research, e for espionage, h to repair, esc to menu, ? for help, q to quit")
	return s
}

func (m Model) viewColonize() string {
	cur, _ := m.galaxy.Node(m.player.NodeID)
	s := style.Title.Render("Found Colony — "+cur.Name) + "\n\n"
	s += fmt.Sprintf("Cost: %d cr, %d turns   Credits: %d cr   Turns: %d/%d\n\n",
		query.ColonizeCost(cur.DevelopmentLevel), query.ColonizeTurnCost,
		m.player.Credits, m.player.Turns.Remaining, m.player.Turns.Max)
	s += style.Faint.Render("Choose the commodity this colony will produce:") + "\n\n"

	for i, c := range query.Commodities {
		line := fmt.Sprintf("%-20s [%s]", c.Name, c.Category)
		if i == m.colonizeCursor {
			s += style.Selected.Render("> "+line) + "\n"
		} else {
			s += "  " + line + "\n"
		}
	}

	if m.colonizeErr != nil {
		s += "\n" + style.ErrorText.Render(m.colonizeErr.Error()) + "\n"
	}

	s += "\n" + style.Faint.Render("up/down select, enter to found, esc to cancel, ? for help")
	return s
}

func (m Model) viewColonies() string {
	s := style.Title.Render("Colonies") + "\n\n"
	if len(m.colonies) == 0 {
		s += style.Faint.Render("no colonies founded yet") + "\n"
	}
	for _, col := range m.colonies {
		n, _ := m.galaxy.Node(col.NodeID)
		c, _ := query.FindCommodity(col.Focus)
		cap := query.ColonyPopulationCap(n.DevelopmentLevel)
		s += fmt.Sprintf("  %-15s population %5d/%-5d producing %s\n", n.Name, col.Population, cap, c.Name)
	}
	s += "\n" + style.Faint.Render("esc to return to the map, ? for help")
	return s
}

func (m Model) viewTechTree() string {
	s := style.Title.Render("Research") + "\n\n"

	if m.research.Active != "" {
		if tech, ok := query.FindTech(m.research.Active); ok {
			s += fmt.Sprintf("Researching: %s (%d/%d points, %d pts/tick)\n\n",
				tech.Name, m.research.Progress, tech.Cost, m.research.RatePerTick())
		}
	} else {
		s += style.Faint.Render("No active research project.") + "\n\n"
	}
	s += fmt.Sprintf("Research rate: %d pts/tick   Trade savvy: -%d NPC greed\n\n",
		m.research.RatePerTick(), m.research.TradeGreedReduction)

	for i, t := range m.techCatalog {
		status := "locked"
		switch {
		case m.research.HasUnlocked(t.ID):
			status = "unlocked"
		case t.ID == m.research.Active:
			status = "researching"
		case m.research.Available(t.ID):
			status = "available"
		}
		line := fmt.Sprintf("%-24s tier %d   cost %-4d   [%s]", t.Name, t.Tier, t.Cost, status)
		if i == m.techCursor {
			s += style.Selected.Render("> "+line) + "\n"
		} else {
			s += "  " + line + "\n"
		}
	}

	s += "\n" + style.Faint.Render("up/down select, enter to research (switching projects resets progress), esc back, ? for help")
	return s
}

func (m Model) viewEspionage() string {
	s := style.Title.Render("Espionage") + "\n\n"
	s += fmt.Sprintf("Credits: %d cr   Turns: %d/%d\n\n", m.player.Credits, m.player.Turns.Remaining, m.player.Turns.Max)

	if len(m.spies) == 0 {
		s += style.Faint.Render("no spies recruited yet") + "\n"
	}
	for i, spy := range m.spies {
		line := fmt.Sprintf("%-10s skill %3d   missions %3d   [%s]", spy.Name, spy.Skill, spy.MissionsRun, spy.Status)
		if i == m.espionageCursor {
			s += style.Selected.Render("> "+line) + "\n"
		} else {
			s += "  " + line + "\n"
		}
	}

	recruitLine := fmt.Sprintf("Recruit New Spy (%d cr, %d turns)", query.RecruitSpyCost, query.RecruitSpyTurnCost)
	if m.espionageCursor == len(m.spies) {
		s += style.Selected.Render("> "+recruitLine) + "\n"
	} else {
		s += "  " + recruitLine + "\n"
	}

	s += "\n" + style.Faint.Render("up/down select, enter to recruit or choose a spy's target, esc back, ? for help")
	return s
}

func (m Model) viewEspionageTarget() string {
	s := style.Title.Render(fmt.Sprintf("Espionage — %s (skill %d)", m.espionageSpy.Name, m.espionageSpy.Skill)) + "\n\n"
	s += fmt.Sprintf("Mission cost: %d turns   Turns: %d/%d\n\n", query.SpyMissionTurnCost, m.player.Turns.Remaining, m.player.Turns.Max)

	for i, n := range m.galaxy.Nodes {
		status := "undiscovered"
		if m.player.HasDiscovered(n.ID) {
			status = "discovered"
		}
		line := fmt.Sprintf("%-15s dev %d   [%s]", n.Name, n.DevelopmentLevel, status)
		if i == m.targetCursor {
			s += style.Selected.Render("> "+line) + "\n"
		} else {
			s += "  " + line + "\n"
		}
	}

	if m.missionReport != "" {
		s += "\n" + style.Faint.Render(m.missionReport) + "\n"
	}

	s += "\n" + style.Faint.Render("up/down select target, s to steal, a to sabotage, i for intel, esc back, ? for help")
	return s
}

func (m Model) viewEncounter() string {
	if m.combatDone {
		s := style.Title.Render("Combat Report") + "\n\n"
		s += describeCombatResult(m.combatResult) + "\n\n"
		if len(m.combatResult.Battle.Log) > 0 {
			s += strings.Join(m.combatResult.Battle.Log, "\n") + "\n\n"
		}
		s += fmt.Sprintf("Ship: Attack %d, Defense %d, Hull %d/%d   Credits: %d cr\n\n",
			m.player.Ship.Attack, m.player.Ship.Defense, m.player.Ship.Hull, m.player.Ship.MaxHull, m.player.Credits)
		s += style.Faint.Render("enter to continue")
		return s
	}

	h := m.encounterHostile
	s := style.Title.Render("Hostile Encounter") + "\n\n"
	s += fmt.Sprintf("A %s approaches! Attack %d, Defense %d, Hull %d/%d\n\n", h.Name, h.Attack, h.Defense, h.Hull, h.MaxHull)
	s += fmt.Sprintf("Your ship: Attack %d, Defense %d, Hull %d/%d\n\n",
		m.player.Ship.Attack, m.player.Ship.Defense, m.player.Ship.Hull, m.player.Ship.MaxHull)
	s += style.Faint.Render("f to fight, r to attempt to flee, ? for help, q to quit")
	return s
}

func (m Model) viewTrade() string {
	cur, _ := m.galaxy.Node(m.player.NodeID)
	s := style.Title.Render("Trade — "+cur.Name) + "\n\n"
	s += fmt.Sprintf("Credits: %d cr   Cargo: %d/%d\n\n", m.player.Credits, m.player.CargoUsed(), m.player.CargoCapacity)

	for i, price := range m.market {
		c, _ := query.FindCommodity(price.CommodityID)
		owned := m.player.Cargo[price.CommodityID]
		line := fmt.Sprintf("%-20s [%-8s] ~%5d cr   owned: %d", c.Name, c.Category, price.Price, owned)
		if i == m.tradeCursor {
			s += style.Selected.Render("> "+line) + "\n"
		} else {
			s += "  " + line + "\n"
		}
	}

	if m.tradeMode != tradeIdle {
		verb := "Buy"
		if m.tradeMode == tradeSelling {
			verb = "Sell"
		}
		s += fmt.Sprintf("\n%s quantity: %s\n", verb, m.qtyInput.View())
	}
	if m.tradeErr != nil {
		s += "\n" + style.ErrorText.Render(m.tradeErr.Error()) + "\n"
	}

	s += "\n" + style.Faint.Render("prices shown are reference only, ~ marks the market rate\n")
	s += style.Faint.Render("up/down select, b negotiate buying, s negotiate selling, esc back, ? for help")
	return s
}

func (m Model) viewHaggle() string {
	s := m.haggleSession
	c, _ := query.FindCommodity(s.Commodity)
	verb := "Buying"
	if !s.Buying {
		verb = "Selling"
	}

	body := style.Title.Render(fmt.Sprintf("Haggling — %s %d x %s", verb, s.Quantity, c.Name)) + "\n\n"
	body += fmt.Sprintf("Round %d   Credits: %d cr   Cargo: %d/%d\n\n",
		s.Round, m.player.Credits, m.player.CargoUsed(), m.player.CargoCapacity)
	body += fmt.Sprintf("Their offer: %d cr/unit   (%d cr total)\n\n", s.NPCOffer, s.NPCOffer*s.Quantity)

	switch s.Outcome {
	case query.HaggleAccepted:
		body += style.Selected.Render(fmt.Sprintf("Deal! %d %s at %d cr/unit.", s.Quantity, c.Name, s.NPCOffer)) + "\n\n"
		body += style.Faint.Render("enter/esc to continue trading")
		return body
	case query.HaggleRejected:
		body += style.ErrorText.Render("They won't deal with you any further. The negotiation collapses.") + "\n\n"
		body += style.Faint.Render("enter/esc to continue trading")
		return body
	}

	if m.haggleOffering {
		body += fmt.Sprintf("Your offer (cr/unit): %s\n", m.priceInput.View())
		if m.haggleErr != nil {
			body += style.ErrorText.Render(m.haggleErr.Error()) + "\n"
		}
		body += "\n" + style.Faint.Render("enter to submit, esc to cancel")
		return body
	}

	if m.haggleErr != nil {
		body += style.ErrorText.Render(m.haggleErr.Error()) + "\n\n"
	}
	body += style.Faint.Render("o to counter-offer, w to walk away (bluff), a to accept, esc to abandon, ? for help, q to quit")
	return body
}

// helpText returns a plain-language explanation of the given screen and its
// keys, shown by stateHelp.
func helpText(s state) string {
	switch s {
	case stateMenu:
		return "The main menu. Resume returns to your loaded game (only shown once one is loaded), New Game creates a fresh save, Load Game opens an existing one."
	case stateLoadList:
		return "Your saved games, most recently played first. Pick one and press enter to load it."
	case stateMap:
		return "Your current system and the warp lanes leading out of it. Fly to a neighboring system, scout an undiscovered one without committing to the flight, trade, found or check on colonies, research tech, run espionage, or repair a damaged hull."
	case stateTrade:
		return "The local market. Buy or sell a commodity — you'll then haggle over the price before the deal is done."
	case stateHaggle:
		return "A price negotiation. Counter-offer, walk away and hope for a better follow-up offer, or accept the current price."
	case stateColonize:
		return "Found a colony at your current system, choosing which commodity it focuses on producing."
	case stateColonies:
		return "A list of the colonies you've founded across the galaxy and their status."
	case stateTechTree:
		return "The technology tree. Each branch is a linear chain of tiers — start research on the next available tech in a branch to begin accruing progress toward it over time."
	case stateEspionage:
		return "Your spies. Recruit a new one for a flat cost, or select an available spy to send on a mission."
	case stateEspionageTarget:
		return "Choose a mission target and mission type: steal credits, sabotage, or gather intel."
	case stateEncounter:
		return "A hostile has intercepted you. Fight it out, or attempt to flee (not guaranteed to succeed)."
	default:
		return "No help is available for this screen."
	}
}

func (m Model) viewHelp() string {
	s := style.Title.Render("Help") + "\n\n"
	s += helpText(m.helpFrom) + "\n\n"
	s += style.Faint.Render("press any key to return")
	return s
}

func (m Model) viewError() string {
	s := style.Title.Render("RPG") + "\n\n"
	s += style.ErrorText.Render(fmt.Sprintf("error: %v", m.err)) + "\n\n"
	s += style.Faint.Render("esc to return to menu, q to quit")
	return s
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

const (
	mapGridW = 50
	mapGridH = 14
)

// visibleNodes returns every node known to exist because it's either
// discovered outright or lies at the fog boundary: adjacent to a
// discovered system via a known warp lane. Anything outside this set is
// true fog — not rendered on the starfield at all.
func visibleNodes(g query.Galaxy, discovered map[query.NodeID]bool) map[query.NodeID]bool {
	visible := make(map[query.NodeID]bool, len(discovered))
	for id, ok := range discovered {
		if !ok {
			continue
		}
		visible[id] = true
		for _, e := range g.Neighbors(id) {
			visible[e.To] = true
		}
	}
	return visible
}

// renderStarfield draws the galaxy as a scaled ASCII scatter of coordinate
// nodes: '@' marks the player's current system, '*' the currently
// highlighted destination, '.' every other discovered system, ',' a system
// visible at the fog boundary but not yet surveyed. Anything not in visible
// is left blank — true fog of war.
func renderStarfield(g query.Galaxy, current, target query.NodeID, discovered, visible map[query.NodeID]bool) string {
	if len(g.Nodes) == 0 {
		return ""
	}

	minX, maxX := g.Nodes[0].X, g.Nodes[0].X
	minY, maxY := g.Nodes[0].Y, g.Nodes[0].Y
	for _, n := range g.Nodes {
		minX, maxX = min(minX, n.X), max(maxX, n.X)
		minY, maxY = min(minY, n.Y), max(maxY, n.Y)
	}

	grid := make([][]rune, mapGridH)
	for i := range grid {
		grid[i] = make([]rune, mapGridW)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	for _, n := range g.Nodes {
		if !visible[n.ID] {
			continue
		}
		gx := scaleCoord(n.X, minX, maxX, mapGridW)
		gy := scaleCoord(n.Y, minY, maxY, mapGridH)
		ch := ','
		switch {
		case n.ID == current:
			ch = '@'
		case n.ID == target:
			ch = '*'
		case discovered[n.ID]:
			ch = '.'
		}
		grid[gy][gx] = ch
	}

	var b strings.Builder
	for _, row := range grid {
		b.WriteString(string(row))
		b.WriteByte('\n')
	}
	return b.String()
}

func scaleCoord(v, lo, hi, size int) int {
	if hi == lo {
		return size / 2
	}
	return (v - lo) * (size - 1) / (hi - lo)
}
