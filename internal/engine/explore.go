package engine

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/rdu90/RPG/internal/engine/explore"
	"github.com/rdu90/RPG/internal/engine/galaxy"
	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/ports"
	"github.com/rdu90/RPG/internal/rng"
)

// ScoutResult carries a newly-surveyed system's anomaly (if any) alongside
// the player's current persisted state.
type ScoutResult struct {
	Player  player.Player
	NodeID  galaxy.NodeID
	Anomaly explore.Anomaly
}

// ClaimAnomalyResult carries the anomaly just collected alongside the
// player's current persisted state.
type ClaimAnomalyResult struct {
	Player  player.Player
	Anomaly explore.Anomaly
}

// AnomalyStatus describes what (if anything) is hidden at a system and
// whether it's already been claimed.
type AnomalyStatus struct {
	Anomaly explore.Anomaly
	Claimed bool
}

// scoutNode surveys c.To, a system adjacent to the player's current one,
// at half the turn cost of flying there, revealing it and any anomaly it
// hides without the player needing to travel there first.
func (e *Engine) scoutNode(ctx context.Context, c ScoutNode) (ScoutResult, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return ScoutResult{}, err
	}

	if p.HasDiscovered(c.To) {
		return ScoutResult{}, fmt.Errorf("engine: %s has already been surveyed", c.To)
	}

	gal, err := e.repo.GetGalaxy(ctx)
	if err != nil {
		return ScoutResult{}, err
	}
	edge, ok := gal.EdgeBetween(p.NodeID, c.To)
	if !ok {
		return ScoutResult{}, fmt.Errorf("engine: no warp lane from %s to %s, out of scouting range", p.NodeID, c.To)
	}
	node, ok := gal.Node(c.To)
	if !ok {
		return ScoutResult{}, fmt.Errorf("engine: unknown system %s", c.To)
	}

	turns, err := p.Turns.Spend(time.Now().UTC(), scoutCost(edge.TurnCost))
	if err != nil {
		return ScoutResult{}, err
	}
	p.Turns = turns

	game, err := e.repo.GetGame(ctx)
	if err != nil {
		return ScoutResult{}, err
	}
	anomaly := explore.At(rng.New(anomalySeed(game.ID, c.To)), node.DevelopmentLevel)

	if p.Discovered == nil {
		p.Discovered = map[galaxy.NodeID]bool{}
	}
	p.Discovered[c.To] = true

	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return ScoutResult{}, err
	}
	return ScoutResult{Player: p, NodeID: c.To, Anomaly: anomaly}, nil
}

// claimAnomaly collects the reward from an unclaimed anomaly at the
// player's current system.
func (e *Engine) claimAnomaly(ctx context.Context, _ ClaimAnomaly) (ClaimAnomalyResult, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return ClaimAnomalyResult{}, err
	}

	anomaly, err := e.anomalyAt(ctx, p.NodeID)
	if err != nil {
		return ClaimAnomalyResult{}, err
	}
	if anomaly.Empty() {
		return ClaimAnomalyResult{}, fmt.Errorf("engine: there is nothing to claim at %s", p.NodeID)
	}
	if p.HasClaimedAnomaly(p.NodeID) {
		return ClaimAnomalyResult{}, fmt.Errorf("engine: the anomaly at %s has already been claimed", p.NodeID)
	}

	p.Credits += anomaly.CreditsReward
	if anomaly.ReputationReward != 0 {
		if p.Reputation == nil {
			p.Reputation = map[galaxy.NodeID]int{}
		}
		rep := p.Reputation[p.NodeID] + anomaly.ReputationReward
		if rep < reputationFloor {
			rep = reputationFloor
		} else if rep > reputationCeil {
			rep = reputationCeil
		}
		p.Reputation[p.NodeID] = rep
	}

	if p.ClaimedAnomalies == nil {
		p.ClaimedAnomalies = map[galaxy.NodeID]bool{}
	}
	p.ClaimedAnomalies[p.NodeID] = true

	if err := e.repo.SavePlayer(ctx, p); err != nil {
		return ClaimAnomalyResult{}, err
	}
	return ClaimAnomalyResult{Player: p, Anomaly: anomaly}, nil
}

// getAnomaly reports what (if anything) is hidden at the player's current
// system, and whether it's already been claimed.
func (e *Engine) getAnomaly(ctx context.Context) (AnomalyStatus, error) {
	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return AnomalyStatus{}, err
	}
	anomaly, err := e.anomalyAt(ctx, p.NodeID)
	if err != nil {
		return AnomalyStatus{}, err
	}
	return AnomalyStatus{Anomaly: anomaly, Claimed: p.HasClaimedAnomaly(p.NodeID)}, nil
}

// anomalyAt deterministically rolls the anomaly (if any) hidden at nodeID.
func (e *Engine) anomalyAt(ctx context.Context, nodeID galaxy.NodeID) (explore.Anomaly, error) {
	gal, err := e.repo.GetGalaxy(ctx)
	if err != nil {
		return explore.Anomaly{}, err
	}
	node, ok := gal.Node(nodeID)
	if !ok {
		return explore.Anomaly{}, fmt.Errorf("engine: unknown system %s", nodeID)
	}
	game, err := e.repo.GetGame(ctx)
	if err != nil {
		return explore.Anomaly{}, err
	}
	return explore.At(rng.New(anomalySeed(game.ID, nodeID)), node.DevelopmentLevel), nil
}

// scoutCost is half a warp lane's flight cost, rounded up (and always at
// least 1, since edgeCost itself is always at least 1): scouting ahead is
// cheaper than actually flying there.
func scoutCost(edgeCost int) int {
	return (edgeCost + 1) / 2
}

// anomalySeed derives a deterministic per-system RNG seed from the save's
// GameID, so the same system always rolls the same anomaly across repeated
// calls (scout, claim, status query) without the engine persisting the roll.
func anomalySeed(gameID ports.GameID, nodeID galaxy.NodeID) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(gameID))
	_, _ = h.Write([]byte("anomaly"))
	_, _ = h.Write([]byte(nodeID))
	return int64(h.Sum64())
}
