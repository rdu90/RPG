// Package app is the bubbletea root model: a title screen that can create
// or load a save, proving the full tui -> transport -> engine -> ports ->
// persistence round trip end to end. Gameplay screens land in later
// milestones; this package only ever imports internal/transport, never
// internal/engine directly.
package app

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/rdu90/RPG/internal/transport/command"
	"github.com/rdu90/RPG/internal/transport/local"
	"github.com/rdu90/RPG/internal/transport/query"
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
	stateMenu state = iota
	stateNewGameInput
	stateLoadList
	stateWorking
	stateGameReady
	stateError
)

type gameReadyMsg struct {
	game query.Game
	err  error
}

type savesLoadedMsg struct {
	saves []string
	err   error
}

// Model is the root bubbletea model.
type Model struct {
	openSave  OpenSave
	listSaves ListSaves

	state state

	menuItems  []string
	menuCursor int

	nameInput textinput.Model

	saves      []string
	loadCursor int

	game query.Game
	err  error
}

// New builds the root model.
func New(openSave OpenSave, listSaves ListSaves) Model {
	ti := textinput.New()
	ti.Placeholder = "save name"
	ti.CharLimit = 40
	ti.Focus()

	return Model{
		openSave:  openSave,
		listSaves: listSaves,
		state:     stateMenu,
		menuItems: []string{"New Game", "Load Game", "Quit"},
		nameInput: ti,
	}
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
		m.game = msg.game
		m.state = stateGameReady
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateMenu:
		return m.handleMenuKey(msg)
	case stateNewGameInput:
		return m.handleNewGameKey(msg)
	case stateLoadList:
		return m.handleLoadListKey(msg)
	case stateGameReady, stateError:
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
	switch msg.String() {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(m.menuItems)-1 {
			m.menuCursor++
		}
	case "enter":
		switch m.menuItems[m.menuCursor] {
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
		return gameReadyMsg{game: res.(query.Game)}
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
		return gameReadyMsg{game: res.(query.Game)}
	}
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.state {
	case stateMenu:
		return m.viewMenu()
	case stateNewGameInput:
		return m.viewNewGameInput()
	case stateLoadList:
		return m.viewLoadList()
	case stateWorking:
		return style.Title.Render("RPG") + "\n\nWorking...\n"
	case stateGameReady:
		return m.viewGameReady()
	case stateError:
		return m.viewError()
	}
	return ""
}

func (m Model) viewMenu() string {
	s := style.Title.Render("RPG") + "\n\n"
	for i, item := range m.menuItems {
		if i == m.menuCursor {
			s += style.Selected.Render("> "+item) + "\n"
		} else {
			s += "  " + item + "\n"
		}
	}
	s += "\n" + style.Faint.Render("up/down to move, enter to select, q to quit")
	return s
}

func (m Model) viewNewGameInput() string {
	s := style.Title.Render("New Game") + "\n\n"
	s += "Save name: " + m.nameInput.View() + "\n\n"
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
	s += "\n" + style.Faint.Render("up/down to move, enter to load, esc to cancel")
	return s
}

func (m Model) viewGameReady() string {
	s := style.Title.Render("RPG") + "\n\n"
	s += fmt.Sprintf("Save %q ready.\n", m.game.Name)
	s += fmt.Sprintf("Created:  %s\n", m.game.CreatedAt.Format("2006-01-02 15:04:05"))
	s += fmt.Sprintf("Updated:  %s\n\n", m.game.UpdatedAt.Format("2006-01-02 15:04:05"))
	s += style.Faint.Render("gameplay lands in a later milestone") + "\n\n"
	s += style.Faint.Render("esc to return to menu, q to quit")
	return s
}

func (m Model) viewError() string {
	s := style.Title.Render("RPG") + "\n\n"
	s += style.ErrorText.Render(fmt.Sprintf("error: %v", m.err)) + "\n\n"
	s += style.Faint.Render("esc to return to menu, q to quit")
	return s
}
