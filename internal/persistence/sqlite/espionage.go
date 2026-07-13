package sqlite

import (
	"context"
	"fmt"

	"github.com/rdu90/RPG/internal/engine/espionage"
)

// SaveSpy implements ports.EspionageRepository, upserting by ID.
func (s *Store) SaveSpy(ctx context.Context, spy espionage.Spy) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO spies (id, name, skill, status, missions_run)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = excluded.name,
			skill = excluded.skill,
			status = excluded.status,
			missions_run = excluded.missions_run`,
		spy.ID, spy.Name, spy.Skill, int(spy.Status), spy.MissionsRun,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save spy %s: %w", spy.ID, err)
	}
	return nil
}

// GetSpies implements ports.EspionageRepository.
func (s *Store) GetSpies(ctx context.Context) ([]espionage.Spy, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, skill, status, missions_run FROM spies`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get spies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []espionage.Spy
	for rows.Next() {
		var spy espionage.Spy
		var status int
		if err := rows.Scan(&spy.ID, &spy.Name, &spy.Skill, &status, &spy.MissionsRun); err != nil {
			return nil, fmt.Errorf("sqlite: scan spy: %w", err)
		}
		spy.Status = espionage.Status(status)
		out = append(out, spy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: get spies: %w", err)
	}
	return out, nil
}
