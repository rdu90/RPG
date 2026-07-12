package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rdu90/RPG/internal/engine/colony"
	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
)

// SaveColony implements ports.ColonyRepository.
func (s *Store) SaveColony(ctx context.Context, c colony.Colony) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO colonies (node_id, focus, population, last_tick_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (node_id) DO UPDATE SET
			focus = excluded.focus,
			population = excluded.population,
			last_tick_at = excluded.last_tick_at`,
		c.NodeID, c.Focus, c.Population, c.LastTickAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save colony at %s: %w", c.NodeID, err)
	}
	return nil
}

// GetColony implements ports.ColonyRepository.
func (s *Store) GetColony(ctx context.Context, nodeID galaxy.NodeID) (colony.Colony, bool, error) {
	c := colony.Colony{NodeID: nodeID}
	var focus string
	row := s.db.QueryRowContext(ctx,
		`SELECT focus, population, last_tick_at FROM colonies WHERE node_id = ?`, nodeID)
	if err := row.Scan(&focus, &c.Population, &c.LastTickAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return colony.Colony{}, false, nil
		}
		return colony.Colony{}, false, fmt.Errorf("sqlite: get colony at %s: %w", nodeID, err)
	}
	c.Focus = economy.CommodityID(focus)
	return c, true, nil
}

// GetColonies implements ports.ColonyRepository.
func (s *Store) GetColonies(ctx context.Context) ([]colony.Colony, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_id, focus, population, last_tick_at FROM colonies`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get colonies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []colony.Colony
	for rows.Next() {
		var c colony.Colony
		var focus string
		if err := rows.Scan(&c.NodeID, &focus, &c.Population, &c.LastTickAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan colony: %w", err)
		}
		c.Focus = economy.CommodityID(focus)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: get colonies: %w", err)
	}
	return out, nil
}
