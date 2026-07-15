package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/rdu90/RPG/internal/engine/colony"
	"github.com/rdu90/RPG/internal/engine/combat"
	"github.com/rdu90/RPG/internal/engine/faction"
	"github.com/rdu90/RPG/internal/engine/fleet"
	"github.com/rdu90/RPG/internal/engine/player"
)

const (
	// bombardTurnCost is the turn price of an orbital bombardment.
	bombardTurnCost = 3

	// invadeTurnCost is the turn price of a ground invasion attempt,
	// matching colonizeTurnCost since both commit the player's fleet to a
	// system for the same rough effort.
	invadeTurnCost = 5

	// bombardDamageMultiplier scales fleet.Damage(ship.Attack,
	// garrison.Defense) up for a bombardment: multiple firing passes from
	// orbit, not a single exchange.
	bombardDamageMultiplier = 3

	// bombardGarrisonFloorFraction is the lowest fraction of a garrison's
	// MaxHull that bombardment alone can bring it to — softening it up
	// still requires a ground invasion to finish the capture.
	bombardGarrisonFloorFraction = 0.2

	// bombardPopulationDamageFraction is the fraction of a colony's
	// population a bombardment costs, floored at 1 so a colony is never
	// bombed out of existence.
	bombardPopulationDamageFraction = 0.1
)

// BombardResult is the result of a Bombard command: the weakened colony,
// alongside the player's current state.
type BombardResult struct {
	Player         player.Player
	Colony         colony.Colony
	PopulationLost int
}

// InvadeResult is the result of an Invade command: the resolved ground
// battle against the colony's garrison, and the colony's state afterward —
// captured if the battle was won. Defender is the faction ID that held the
// colony going into the fight: Colony.Owner itself flips to OwnerPlayer on
// a win, so callers describing the outcome (which faction fell, or held)
// need this to still name the right side.
type InvadeResult struct {
	Player   player.Player
	Colony   colony.Colony
	Defender string
	Battle   combat.Result
	Captured bool
}

// rivalColonyAt loads the colony at the player's current system, erroring
// if there is none or if it already belongs to the player.
func (e *Engine) rivalColonyAt(ctx context.Context, p player.Player) (colony.Colony, error) {
	c, ok, err := e.repo.GetColony(ctx, p.NodeID)
	if err != nil {
		return colony.Colony{}, err
	}
	if !ok {
		return colony.Colony{}, fmt.Errorf("engine: no colony here")
	}
	if c.Owner == colony.OwnerPlayer {
		return colony.Colony{}, fmt.Errorf("engine: the colony at %s is already yours", p.NodeID)
	}
	return c, nil
}

// bombard strikes the rival colony at the player's current system from
// orbit, weakening its garrison and population without risking the
// player's ship.
func (e *Engine) bombard(ctx context.Context, _ Bombard) (BombardResult, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return BombardResult{}, err
	}

	c, err := e.rivalColonyAt(ctx, p)
	if err != nil {
		return BombardResult{}, err
	}

	turns, err := p.Turns.Spend(time.Now().UTC(), bombardTurnCost)
	if err != nil {
		return BombardResult{}, err
	}
	p.Turns = turns

	dmg := fleet.Damage(p.Ship.Attack, c.Garrison.Defense) * bombardDamageMultiplier
	floor := int(float64(c.Garrison.MaxHull) * bombardGarrisonFloorFraction)
	c.Garrison.Hull -= dmg
	if c.Garrison.Hull < floor {
		c.Garrison.Hull = floor
	}

	popLoss := int(float64(c.Population) * bombardPopulationDamageFraction)
	if popLoss < 1 {
		popLoss = 1
	}
	c.Population -= popLoss
	if c.Population < 1 {
		c.Population = 1
	}

	if err := e.repo.SaveColony(ctx, c); err != nil {
		return BombardResult{}, err
	}
	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return BombardResult{}, err
	}
	return BombardResult{Player: p, Colony: c, PopulationLost: popLoss}, nil
}

// invade attempts to capture the rival colony at the player's current
// system, fighting its garrison to a conclusion.
func (e *Engine) invade(ctx context.Context, _ Invade) (InvadeResult, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return InvadeResult{}, err
	}

	c, err := e.rivalColonyAt(ctx, p)
	if err != nil {
		return InvadeResult{}, err
	}

	turns, err := p.Turns.Spend(time.Now().UTC(), invadeTurnCost)
	if err != nil {
		return InvadeResult{}, err
	}
	p.Turns = turns

	name := c.Owner
	if f, ok := faction.Find(c.Owner); ok {
		name = f.Name
	}
	hostile := combat.Hostile{
		Name:    name + " Garrison",
		Attack:  c.Garrison.Attack,
		Defense: c.Garrison.Defense,
		Hull:    c.Garrison.Hull,
		MaxHull: c.Garrison.MaxHull,
	}

	battle := combat.Fight(p.Ship, hostile)
	p.Ship.Hull = battle.PlayerHull

	result := InvadeResult{Player: p, Defender: c.Owner, Battle: battle}
	switch battle.Outcome {
	case combat.Victory:
		c.Owner = colony.OwnerPlayer
		c.Garrison = fleet.Stats{}
		result.Captured = true
	case combat.Defeat:
		lost := int(float64(p.Credits) * defeatCreditsFraction)
		p.Credits -= lost
		for id, qty := range p.Cargo {
			p.Cargo[id] = qty - int(float64(qty)*defeatCargoFraction)
		}
		c.Garrison.Hull = battle.HostileHull
	default:
		c.Garrison.Hull = battle.HostileHull
	}

	if err := e.repo.SaveColony(ctx, c); err != nil {
		return InvadeResult{}, err
	}
	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return InvadeResult{}, err
	}
	result.Player = p
	result.Colony = c
	return result, nil
}
