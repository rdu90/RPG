package engine

import (
	"context"
	"fmt"

	"github.com/rdu90/RPG/internal/engine/combat"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/rng"
)

const (
	// repairCostPerHull is the credit price of restoring one point of
	// hull damage.
	repairCostPerHull = 4

	// lootBaseCredits, lootCreditsPerMaxHull, and lootCreditsJitter derive
	// a defeated hostile's credit reward: tougher raiders were carrying
	// more worth plundering.
	lootBaseCredits       = 50
	lootCreditsPerMaxHull = 2
	lootCreditsJitter     = 40

	// defeatCreditsFraction and defeatCargoFraction are the share of
	// credits and of each cargo stack a losing fight costs the player —
	// raided, not destroyed: there is no permadeath in v1.
	defeatCreditsFraction = 0.25
	defeatCargoFraction   = 0.25
)

// RepairCostPerHull is the credit price of restoring one point of hull
// damage.
const RepairCostPerHull = repairCostPerHull

// CombatResult is the result of a ResolveEncounter command: the resolved
// battle (or flee attempt), alongside the player's current state.
type CombatResult struct {
	Player  player.Player
	Hostile combat.Hostile
	// Fled is true if the player attempted to flee and succeeded, in
	// which case Battle is zero-valued — no fight took place.
	Fled          bool
	Battle        combat.Result
	CreditsGained int
	CreditsLost   int
}

// resolveEncounter resolves a hostile encountered on arrival: either an
// attempted flee, or a fight to a conclusion. c.Hostile must be the exact
// value the triggering Move returned, so the fight is reproducible from
// its Seed.
func (e *Engine) resolveEncounter(ctx context.Context, c ResolveEncounter) (CombatResult, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return CombatResult{}, err
	}

	r := rng.New(c.Hostile.Seed)

	if c.Flee && combat.AttemptFlee(r, p.Ship, c.Hostile) {
		return CombatResult{Player: p, Hostile: c.Hostile, Fled: true}, nil
	}

	battle := combat.Fight(p.Ship, c.Hostile)
	p.Ship.Hull = battle.PlayerHull

	result := CombatResult{Player: p, Hostile: c.Hostile, Battle: battle}
	switch battle.Outcome {
	case combat.Victory:
		gain := lootBaseCredits + c.Hostile.MaxHull*lootCreditsPerMaxHull + r.IntN(lootCreditsJitter+1)
		p.Credits += gain
		result.CreditsGained = gain
	case combat.Defeat:
		lost := int(float64(p.Credits) * defeatCreditsFraction)
		p.Credits -= lost
		result.CreditsLost = lost
		for id, qty := range p.Cargo {
			p.Cargo[id] = qty - int(float64(qty)*defeatCargoFraction)
		}
	}

	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return CombatResult{}, err
	}
	result.Player = p
	return result, nil
}

// repairShip restores the player's ship to full hull for a credit cost
// proportional to the damage repaired.
func (e *Engine) repairShip(ctx context.Context, _ RepairShip) (player.Player, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return player.Player{}, err
	}

	missing := p.Ship.MaxHull - p.Ship.Hull
	if missing <= 0 {
		return player.Player{}, fmt.Errorf("engine: ship is already at full hull")
	}
	cost := missing * repairCostPerHull
	if p.Credits < cost {
		return player.Player{}, fmt.Errorf("engine: need %d credits to repair, have %d", cost, p.Credits)
	}

	p.Credits -= cost
	p.Ship = p.Ship.Repaired()

	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return player.Player{}, err
	}
	return p, nil
}
