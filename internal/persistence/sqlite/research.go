package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rdu90/RPG/internal/engine/techtree"
)

// GetResearch implements ports.ResearchRepository. A save with no research
// row yet (nothing ever started) returns a zero-value Research with an
// initialized empty Unlocked set.
func (s *Store) GetResearch(ctx context.Context) (techtree.Research, error) {
	var r techtree.Research
	var active string
	row := s.db.QueryRowContext(ctx,
		`SELECT active_tech, progress, last_tick_at, rate_bonus, trade_greed_reduction FROM research WHERE id = 1`)
	if err := row.Scan(&active, &r.Progress, &r.LastTickAt, &r.RateBonus, &r.TradeGreedReduction); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return techtree.Research{Unlocked: map[techtree.TechID]bool{}}, nil
		}
		return techtree.Research{}, fmt.Errorf("sqlite: get research: %w", err)
	}
	r.Active = techtree.TechID(active)

	rows, err := s.db.QueryContext(ctx, `SELECT tech_id FROM research_unlocked`)
	if err != nil {
		return techtree.Research{}, fmt.Errorf("sqlite: get unlocked techs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	r.Unlocked = map[techtree.TechID]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return techtree.Research{}, fmt.Errorf("sqlite: scan unlocked tech: %w", err)
		}
		r.Unlocked[techtree.TechID(id)] = true
	}
	if err := rows.Err(); err != nil {
		return techtree.Research{}, fmt.Errorf("sqlite: get unlocked techs: %w", err)
	}

	return r, nil
}

// SaveResearch implements ports.ResearchRepository, upserting the singleton
// research row and fully replacing the unlocked-tech set (delete then
// reinsert), the same pattern SaveColony and SaveMarket use.
func (s *Store) SaveResearch(ctx context.Context, r techtree.Research) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: save research: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO research (id, active_tech, progress, last_tick_at, rate_bonus, trade_greed_reduction)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			active_tech = excluded.active_tech,
			progress = excluded.progress,
			last_tick_at = excluded.last_tick_at,
			rate_bonus = excluded.rate_bonus,
			trade_greed_reduction = excluded.trade_greed_reduction`,
		string(r.Active), r.Progress, r.LastTickAt, r.RateBonus, r.TradeGreedReduction,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save research: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM research_unlocked`); err != nil {
		return fmt.Errorf("sqlite: clear unlocked techs: %w", err)
	}
	for id := range r.Unlocked {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO research_unlocked (tech_id) VALUES (?)`, string(id),
		); err != nil {
			return fmt.Errorf("sqlite: save unlocked tech %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: save research: %w", err)
	}
	return nil
}
