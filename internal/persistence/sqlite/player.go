package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/turn"
)

// InitPlayer implements ports.PlayerRepository.
func (s *Store) InitPlayer(ctx context.Context, p player.Player) error {
	return s.savePlayer(ctx, p)
}

// SavePlayer implements ports.PlayerRepository.
func (s *Store) SavePlayer(ctx context.Context, p player.Player) error {
	return s.savePlayer(ctx, p)
}

func (s *Store) savePlayer(ctx context.Context, p player.Player) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: save player: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO player (id, credits, node_id, cargo_capacity, turns_max, turns_remaining, turns_refill_every_ms, turns_last_refill_at, alignment_legality, alignment_morality)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			credits = excluded.credits,
			node_id = excluded.node_id,
			cargo_capacity = excluded.cargo_capacity,
			turns_max = excluded.turns_max,
			turns_remaining = excluded.turns_remaining,
			turns_refill_every_ms = excluded.turns_refill_every_ms,
			turns_last_refill_at = excluded.turns_last_refill_at,
			alignment_legality = excluded.alignment_legality,
			alignment_morality = excluded.alignment_morality`,
		p.Credits, p.NodeID, p.CargoCapacity,
		p.Turns.Max, p.Turns.Remaining, p.Turns.RefillEvery.Milliseconds(), p.Turns.LastRefillAt,
		p.Alignment.Legality, p.Alignment.Morality,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save player: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM player_cargo`); err != nil {
		return fmt.Errorf("sqlite: clear player cargo: %w", err)
	}
	for id, qty := range p.Cargo {
		if qty == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO player_cargo (commodity_id, quantity) VALUES (?, ?)`, id, qty,
		); err != nil {
			return fmt.Errorf("sqlite: save player cargo %s: %w", id, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM player_reputation`); err != nil {
		return fmt.Errorf("sqlite: clear player reputation: %w", err)
	}
	for node, rep := range p.Reputation {
		if rep == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO player_reputation (node_id, reputation) VALUES (?, ?)`, node, rep,
		); err != nil {
			return fmt.Errorf("sqlite: save player reputation at %s: %w", node, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: save player: %w", err)
	}
	return nil
}

// GetPlayer implements ports.PlayerRepository.
func (s *Store) GetPlayer(ctx context.Context) (player.Player, error) {
	var (
		p               player.Player
		refillEveryMs   int64
		lastRefillAt    time.Time
		turnsMax, turns int
	)

	row := s.db.QueryRowContext(ctx, `
		SELECT credits, node_id, cargo_capacity, turns_max, turns_remaining, turns_refill_every_ms, turns_last_refill_at, alignment_legality, alignment_morality
		FROM player LIMIT 1`)
	if err := row.Scan(&p.Credits, &p.NodeID, &p.CargoCapacity, &turnsMax, &turns, &refillEveryMs, &lastRefillAt,
		&p.Alignment.Legality, &p.Alignment.Morality); err != nil {
		return player.Player{}, fmt.Errorf("sqlite: get player: %w", err)
	}
	p.Turns = turn.Allowance{
		Max:          turnsMax,
		Remaining:    turns,
		RefillEvery:  time.Duration(refillEveryMs) * time.Millisecond,
		LastRefillAt: lastRefillAt,
	}

	rows, err := s.db.QueryContext(ctx, `SELECT commodity_id, quantity FROM player_cargo`)
	if err != nil {
		return player.Player{}, fmt.Errorf("sqlite: get player cargo: %w", err)
	}
	defer func() { _ = rows.Close() }()

	p.Cargo = map[economy.CommodityID]int{}
	for rows.Next() {
		var id economy.CommodityID
		var qty int
		if err := rows.Scan(&id, &qty); err != nil {
			return player.Player{}, fmt.Errorf("sqlite: scan player cargo: %w", err)
		}
		p.Cargo[id] = qty
	}
	if err := rows.Err(); err != nil {
		return player.Player{}, fmt.Errorf("sqlite: get player cargo: %w", err)
	}

	repRows, err := s.db.QueryContext(ctx, `SELECT node_id, reputation FROM player_reputation`)
	if err != nil {
		return player.Player{}, fmt.Errorf("sqlite: get player reputation: %w", err)
	}
	defer func() { _ = repRows.Close() }()

	p.Reputation = map[galaxy.NodeID]int{}
	for repRows.Next() {
		var node galaxy.NodeID
		var rep int
		if err := repRows.Scan(&node, &rep); err != nil {
			return player.Player{}, fmt.Errorf("sqlite: scan player reputation: %w", err)
		}
		p.Reputation[node] = rep
	}
	if err := repRows.Err(); err != nil {
		return player.Player{}, fmt.Errorf("sqlite: get player reputation: %w", err)
	}

	return p, nil
}
