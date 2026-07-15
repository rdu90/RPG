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
		INSERT INTO colonies (node_id, focus, population, last_tick_at, owner, garrison_attack, garrison_defense, garrison_hull, garrison_max_hull)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (node_id) DO UPDATE SET
			focus = excluded.focus,
			population = excluded.population,
			last_tick_at = excluded.last_tick_at,
			owner = excluded.owner,
			garrison_attack = excluded.garrison_attack,
			garrison_defense = excluded.garrison_defense,
			garrison_hull = excluded.garrison_hull,
			garrison_max_hull = excluded.garrison_max_hull`,
		c.NodeID, c.Focus, c.Population, c.LastTickAt, c.Owner,
		c.Garrison.Attack, c.Garrison.Defense, c.Garrison.Hull, c.Garrison.MaxHull,
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
		`SELECT focus, population, last_tick_at, owner, garrison_attack, garrison_defense, garrison_hull, garrison_max_hull FROM colonies WHERE node_id = ?`, nodeID)
	if err := row.Scan(&focus, &c.Population, &c.LastTickAt, &c.Owner,
		&c.Garrison.Attack, &c.Garrison.Defense, &c.Garrison.Hull, &c.Garrison.MaxHull); err != nil {
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
	rows, err := s.db.QueryContext(ctx,
		`SELECT node_id, focus, population, last_tick_at, owner, garrison_attack, garrison_defense, garrison_hull, garrison_max_hull FROM colonies`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get colonies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []colony.Colony
	for rows.Next() {
		var c colony.Colony
		var focus string
		if err := rows.Scan(&c.NodeID, &focus, &c.Population, &c.LastTickAt, &c.Owner,
			&c.Garrison.Attack, &c.Garrison.Defense, &c.Garrison.Hull, &c.Garrison.MaxHull); err != nil {
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
