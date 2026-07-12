package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/rdu90/RPG/internal/engine/player"
	"github.com/rdu90/RPG/internal/engine/techtree"
)

// StartResearchResult is the result of a StartResearch command: the
// player's research state after starting (or switching to) a project,
// alongside the player's current state (which may have changed if switching
// off an in-progress project let it complete first).
type StartResearchResult struct {
	Player   player.Player
	Research techtree.Research
}

// TechTreeStatus is the result of GetTechTree: the fixed tech catalog
// alongside the player's current research progress and state.
type TechTreeStatus struct {
	Catalog  []techtree.Tech
	Research techtree.Research
	Player   player.Player
}

// startResearch begins researching c.Tech, first advancing any currently
// in-progress project to now (so switching away from a project that would
// have just completed still grants its effect) before replacing it.
func (e *Engine) startResearch(ctx context.Context, c StartResearch) (StartResearchResult, error) {
	if _, ok := techtree.Find(c.Tech); !ok {
		return StartResearchResult{}, fmt.Errorf("engine: unknown tech %q", c.Tech)
	}

	research, err := e.repo.GetResearch(ctx)
	if err != nil {
		return StartResearchResult{}, err
	}
	if err := e.tickResearch(ctx, &research); err != nil {
		return StartResearchResult{}, err
	}

	research, err = research.Start(c.Tech, time.Now().UTC())
	if err != nil {
		return StartResearchResult{}, err
	}
	if err := e.repo.SaveResearch(ctx, research); err != nil {
		return StartResearchResult{}, err
	}

	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return StartResearchResult{}, err
	}
	return StartResearchResult{Player: p, Research: research}, nil
}

// getTechTree returns the fixed tech catalog alongside the player's
// research progress, advanced to now first.
func (e *Engine) getTechTree(ctx context.Context) (TechTreeStatus, error) {
	research, err := e.repo.GetResearch(ctx)
	if err != nil {
		return TechTreeStatus{}, err
	}
	if err := e.tickResearch(ctx, &research); err != nil {
		return TechTreeStatus{}, err
	}

	p, err := e.repo.GetPlayer(ctx)
	if err != nil {
		return TechTreeStatus{}, err
	}
	return TechTreeStatus{Catalog: techtree.Catalog, Research: research, Player: p}, nil
}

// tickResearch advances research to now in place. If no ticks elapsed, it
// leaves persistence untouched. If a tech completed as a result, its effect
// is applied and persisted: CargoCapacity and TurnMax effects land on
// player.Player directly (research has no cargo hold or turn allowance of
// its own to store them on), while ResearchRate and TradeGreedReduction
// effects are already folded into research by techtree.Research.Ticked.
// Either way, the advanced research state is persisted.
func (e *Engine) tickResearch(ctx context.Context, research *techtree.Research) error {
	updated, ticks, completed := research.Ticked(time.Now().UTC())
	*research = updated
	if ticks == 0 {
		return nil
	}

	if completed != "" {
		if tech, ok := techtree.Find(completed); ok {
			switch tech.Effect.Kind {
			case techtree.EffectCargoCapacity, techtree.EffectTurnMax:
				p, err := e.repo.GetPlayer(ctx)
				if err != nil {
					return err
				}
				if tech.Effect.Kind == techtree.EffectCargoCapacity {
					p.CargoCapacity += tech.Effect.Magnitude
				} else {
					p.Turns.Max += tech.Effect.Magnitude
				}
				if err := e.repo.SavePlayer(ctx, p); err != nil {
					return err
				}
			}
		}
	}

	return e.repo.SaveResearch(ctx, *research)
}
