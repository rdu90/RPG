package engine

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/espionage"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/ports"
	"github.com/rdu90/RPG/internal/rng"
)

const (
	// recruitCost and recruitTurnCost are the price of hiring a new spy.
	recruitCost     = 300
	recruitTurnCost = 2

	// missionTurnCost is the turn price of sending a spy on any mission,
	// regardless of target distance — the whole point of espionage is that
	// it doesn't require the player's ship to travel there.
	missionTurnCost = 4

	// spyBaseSkill and spySkillJitter bound a recruit's fixed Skill: there
	// is no leveling/training mechanic in v1, so the roll at recruitment is
	// permanent.
	spyBaseSkill   = 35
	spySkillJitter = 30 // skill lands in [spyBaseSkill, spyBaseSkill+spySkillJitter]

	// stealBaseCredits, stealPerDevLevel, and stealCreditsJitter derive a
	// successful MissionSteal's credit reward: richer, more developed
	// systems have more worth stealing.
	stealBaseCredits   = 150
	stealPerDevLevel   = 60
	stealCreditsJitter = 50

	// sabotageDiscount is the fraction knocked off every commodity price at
	// the target system on a successful MissionSabotage, floored at
	// sabotagePriceFloorFraction of that commodity's base price so a
	// sabotaged market never becomes worthless.
	sabotageDiscount           = 0.35
	sabotagePriceFloorFraction = 0.4
)

// RecruitSpyCost and RecruitSpyTurnCost are the price of hiring a new spy.
const (
	RecruitSpyCost     = recruitCost
	RecruitSpyTurnCost = recruitTurnCost
)

// SpyMissionTurnCost is the turn price of sending a spy on any mission.
const SpyMissionTurnCost = missionTurnCost

// spyCodenames is the small, fixed pool recruited spies draw their names
// from; spyName appends a numeric suffix once the pool wraps.
var spyCodenames = []string{
	"Nyx", "Vega", "Corvus", "Lyra", "Talon", "Wraith", "Onyx", "Sable", "Rook", "Cipher",
}

// RecruitSpyResult is the result of a RecruitSpy command: the newly hired
// spy, alongside the player's current state.
type RecruitSpyResult struct {
	Player player.Player
	Spy    espionage.Spy
}

// MissionResult is the result of a SendSpyMission command: the mission's
// outcome and the spy who ran it (status/mission count updated), alongside
// the player's current state.
type MissionResult struct {
	Player  player.Player
	Spy     espionage.Spy
	Mission espionage.MissionKind
	Target  galaxy.NodeID
	Outcome espionage.Outcome
	// CreditsStolen is only set for a successful MissionSteal.
	CreditsStolen int
}

// recruitSpy hires a new spy for a flat credit and turn cost, with Skill
// randomized (but permanent) at recruitment.
func (e *Engine) recruitSpy(ctx context.Context, _ RecruitSpy) (RecruitSpyResult, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return RecruitSpyResult{}, err
	}
	if p.Credits < recruitCost {
		return RecruitSpyResult{}, fmt.Errorf("engine: need %d credits to recruit a spy, have %d", recruitCost, p.Credits)
	}
	turns, err := p.Turns.Spend(time.Now().UTC(), recruitTurnCost)
	if err != nil {
		return RecruitSpyResult{}, err
	}

	spies, err := e.repo.GetSpies(ctx)
	if err != nil {
		return RecruitSpyResult{}, err
	}
	game, err := e.repo.GetGame(ctx)
	if err != nil {
		return RecruitSpyResult{}, err
	}

	index := len(spies)
	r := rng.New(spyRecruitSeed(game.ID, index))
	spy := espionage.Spy{
		ID:     fmt.Sprintf("spy-%d", index),
		Name:   spyName(index),
		Skill:  spyBaseSkill + r.IntN(spySkillJitter+1),
		Status: espionage.StatusAvailable,
	}
	if err := e.repo.SaveSpy(ctx, spy); err != nil {
		return RecruitSpyResult{}, err
	}

	p.Turns = turns
	p.Credits -= recruitCost
	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return RecruitSpyResult{}, err
	}

	return RecruitSpyResult{Player: p, Spy: spy}, nil
}

// sendSpyMission resolves c immediately as a single probability check: Spy's
// Skill against the target system's CounterIntel. A failed mission has a
// further chance of costing the spy their freedom.
func (e *Engine) sendSpyMission(ctx context.Context, c SendSpyMission) (MissionResult, error) {
	switch c.Mission {
	case espionage.MissionSteal, espionage.MissionSabotage, espionage.MissionIntel:
	default:
		return MissionResult{}, fmt.Errorf("engine: unknown mission kind %q", c.Mission)
	}

	spies, err := e.repo.GetSpies(ctx)
	if err != nil {
		return MissionResult{}, err
	}
	idx := -1
	for i, s := range spies {
		if s.ID == c.Spy {
			idx = i
			break
		}
	}
	if idx == -1 {
		return MissionResult{}, fmt.Errorf("engine: unknown spy %q", c.Spy)
	}
	spy := spies[idx]
	if spy.Status != espionage.StatusAvailable {
		return MissionResult{}, fmt.Errorf("engine: spy %s is not available", spy.Name)
	}

	gal, err := e.repo.GetGalaxy(ctx)
	if err != nil {
		return MissionResult{}, err
	}
	node, ok := gal.Node(c.Target)
	if !ok {
		return MissionResult{}, fmt.Errorf("engine: unknown system %s", c.Target)
	}

	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return MissionResult{}, err
	}
	turns, err := p.Turns.Spend(time.Now().UTC(), missionTurnCost)
	if err != nil {
		return MissionResult{}, err
	}
	p.Turns = turns

	game, err := e.repo.GetGame(ctx)
	if err != nil {
		return MissionResult{}, err
	}
	r := rng.New(missionSeed(game.ID, spy.ID, c.Target, c.Mission, spy.MissionsRun))

	outcome := espionage.Resolve(r, spy.Skill, espionage.CounterIntel(node.DevelopmentLevel))
	spy.MissionsRun++
	if outcome.Captured {
		spy.Status = espionage.StatusCaptured
	}

	result := MissionResult{Mission: c.Mission, Target: c.Target, Outcome: outcome}
	if outcome.Success {
		switch c.Mission {
		case espionage.MissionIntel:
			if p.Discovered == nil {
				p.Discovered = map[galaxy.NodeID]bool{}
			}
			p.Discovered[c.Target] = true
		case espionage.MissionSteal:
			gain := stealBaseCredits + node.DevelopmentLevel*stealPerDevLevel + r.IntN(stealCreditsJitter+1)
			p.Credits += gain
			result.CreditsStolen = gain
		case espionage.MissionSabotage:
			if err := e.sabotageMarket(ctx, c.Target); err != nil {
				return MissionResult{}, err
			}
		}
	}

	if err := e.repo.SaveSpy(ctx, spy); err != nil {
		return MissionResult{}, err
	}
	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return MissionResult{}, err
	}

	result.Spy = spy
	result.Player = p
	return result, nil
}

// sabotageMarket crashes every commodity price at nodeID by sabotageDiscount,
// floored at sabotagePriceFloorFraction of each commodity's base price.
func (e *Engine) sabotageMarket(ctx context.Context, nodeID galaxy.NodeID) error {
	prices, err := e.repo.GetMarket(ctx, nodeID)
	if err != nil {
		return err
	}
	for i, pr := range prices {
		commodity, ok := economy.Find(pr.CommodityID)
		if !ok {
			continue
		}
		floor := int(float64(commodity.BasePrice) * sabotagePriceFloorFraction)
		if floor < 1 {
			floor = 1
		}
		price := int(float64(pr.Price) * (1 - sabotageDiscount))
		if price < floor {
			price = floor
		}
		prices[i].Price = price
	}
	return e.repo.SaveMarket(ctx, nodeID, prices)
}

// spyName cycles through spyCodenames, appending a numeric suffix once the
// pool wraps so recruits stay distinguishable.
func spyName(index int) string {
	name := spyCodenames[index%len(spyCodenames)]
	if index < len(spyCodenames) {
		return name
	}
	return fmt.Sprintf("%s-%d", name, index/len(spyCodenames)+1)
}

// spyRecruitSeed derives a deterministic RNG seed for the index-th spy
// recruited in a save, so a recruitment always rolls the same Skill if
// replayed.
func spyRecruitSeed(gameID ports.GameID, index int) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(gameID))
	_, _ = h.Write([]byte("spy-recruit"))
	_, _ = h.Write([]byte(strconv.Itoa(index)))
	return int64(h.Sum64())
}

// missionSeed derives a deterministic per-attempt RNG seed from the save's
// GameID, the spy and target involved, and the spy's running mission count
// (the "round" analog, mirroring haggleSeed's round number) — the same spy
// sent against the same target twice resolves differently each time without
// the engine needing to persist any RNG state between commands.
func missionSeed(gameID ports.GameID, spyID string, target galaxy.NodeID, mission espionage.MissionKind, attempt int) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(gameID))
	_, _ = h.Write([]byte(spyID))
	_, _ = h.Write([]byte(target))
	_, _ = h.Write([]byte(mission))
	_, _ = h.Write([]byte(strconv.Itoa(attempt)))
	return int64(h.Sum64())
}
