// Command rpg is the single-player terminal entrypoint: it wires the
// SQLite persistence layer, the engine, and the in-process transport
// together and hands them to the bubbletea TUI.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rdu90/RPG/internal/config"
	"github.com/rdu90/RPG/internal/engine"
	"github.com/rdu90/RPG/internal/persistence/sqlite"
	"github.com/rdu90/RPG/internal/transport/local"
	"github.com/rdu90/RPG/internal/tui/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "rpg:", err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := config.SaveDir()
	if err != nil {
		return fmt.Errorf("resolve save directory: %w", err)
	}

	var store *sqlite.Store
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

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

	m := app.New(openSave, listSaves)
	p := tea.NewProgram(m)
	_, err = p.Run()
	return err
}
