package sqlite

import (
	"context"
	"fmt"

	"github.com/rdu90/RPG/internal/engine/galaxy"
)

// SaveGalaxy implements ports.GalaxyRepository.
func (s *Store) SaveGalaxy(ctx context.Context, g galaxy.Galaxy) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: save galaxy: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, n := range g.Nodes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO galaxy_nodes (id, name, x, y, development_level) VALUES (?, ?, ?, ?, ?)`,
			n.ID, n.Name, n.X, n.Y, n.DevelopmentLevel,
		); err != nil {
			return fmt.Errorf("sqlite: save galaxy node %s: %w", n.ID, err)
		}
	}
	for _, e := range g.Edges {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO galaxy_edges (from_id, to_id, turn_cost) VALUES (?, ?, ?)`,
			e.From, e.To, e.TurnCost,
		); err != nil {
			return fmt.Errorf("sqlite: save galaxy edge %s->%s: %w", e.From, e.To, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: save galaxy: %w", err)
	}
	return nil
}

// GetGalaxy implements ports.GalaxyRepository.
func (s *Store) GetGalaxy(ctx context.Context) (galaxy.Galaxy, error) {
	var g galaxy.Galaxy

	nodeRows, err := s.db.QueryContext(ctx, `SELECT id, name, x, y, development_level FROM galaxy_nodes`)
	if err != nil {
		return galaxy.Galaxy{}, fmt.Errorf("sqlite: get galaxy nodes: %w", err)
	}
	defer func() { _ = nodeRows.Close() }()
	for nodeRows.Next() {
		var n galaxy.Node
		if err := nodeRows.Scan(&n.ID, &n.Name, &n.X, &n.Y, &n.DevelopmentLevel); err != nil {
			return galaxy.Galaxy{}, fmt.Errorf("sqlite: scan galaxy node: %w", err)
		}
		g.Nodes = append(g.Nodes, n)
	}
	if err := nodeRows.Err(); err != nil {
		return galaxy.Galaxy{}, fmt.Errorf("sqlite: get galaxy nodes: %w", err)
	}

	edgeRows, err := s.db.QueryContext(ctx, `SELECT from_id, to_id, turn_cost FROM galaxy_edges`)
	if err != nil {
		return galaxy.Galaxy{}, fmt.Errorf("sqlite: get galaxy edges: %w", err)
	}
	defer func() { _ = edgeRows.Close() }()
	for edgeRows.Next() {
		var e galaxy.Edge
		if err := edgeRows.Scan(&e.From, &e.To, &e.TurnCost); err != nil {
			return galaxy.Galaxy{}, fmt.Errorf("sqlite: scan galaxy edge: %w", err)
		}
		g.Edges = append(g.Edges, e)
	}
	if err := edgeRows.Err(); err != nil {
		return galaxy.Galaxy{}, fmt.Errorf("sqlite: get galaxy edges: %w", err)
	}

	return g, nil
}
