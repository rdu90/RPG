// Package sqlite is the SQLite-backed implementation of the engine's
// repository interfaces (internal/engine/ports). It imports the engine's
// ports package; the engine never imports this package, so storage can be
// swapped or scaled later (WAL-mode server, Postgres) without touching game
// logic.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/rdu90/RPG/internal/engine/ports"
	"github.com/rdu90/RPG/internal/persistence/migrations"
)

// Store is a single save's SQLite-backed persistence handle.
type Store struct {
	db *sql.DB
}

// Open opens (creating if necessary) the SQLite file at path and applies
// any pending migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: ping %s: %w", path, err)
	}

	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(migrations.FS)
	defer goose.SetBaseFS(nil)

	if err := goose.SetDialect("sqlite3"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: set dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: migrate %s: %w", path, err)
	}

	return &Store{db: db}, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateGame implements ports.GameRepository.
func (s *Store) CreateGame(ctx context.Context, name string) (ports.Game, error) {
	now := time.Now().UTC()
	game := ports.Game{
		ID:        ports.GameID(uuid.NewString()),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO games (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		game.ID, game.Name, game.CreatedAt, game.UpdatedAt,
	)
	if err != nil {
		return ports.Game{}, fmt.Errorf("sqlite: create game: %w", err)
	}
	return game, nil
}

// GetGame implements ports.GameRepository.
func (s *Store) GetGame(ctx context.Context) (ports.Game, error) {
	var g ports.Game
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, created_at, updated_at FROM games LIMIT 1`)
	if err := row.Scan(&g.ID, &g.Name, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return ports.Game{}, fmt.Errorf("sqlite: get game: %w", err)
	}
	return g, nil
}
