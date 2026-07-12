// Package techtree models the player's research tree: a small, data-driven
// catalog of tech nodes gated by linear per-branch prerequisites, and a
// Research state that accrues points toward the player's actively-researched
// tech on a coarse tick — computed lazily from elapsed real time, the same
// pattern as turn.Allowance and colony.Colony, rather than a running
// scheduler. Each tech's effect is applied once, as a permanent delta, at
// the moment it completes: cargo capacity and turn allowance effects land
// directly on player.Player (the engine layer's job, since Research has no
// cargo hold or turn allowance of its own), while trade and research-rate
// effects accumulate on Research itself.
package techtree

import (
	"errors"
	"fmt"
	"time"
)

// TechID identifies a single tech node in Catalog.
type TechID string

// EffectKind identifies what a completed tech permanently changes.
type EffectKind string

const (
	// EffectCargoCapacity increases player.Player.CargoCapacity.
	EffectCargoCapacity EffectKind = "cargo_capacity"
	// EffectTurnMax increases player.Player.Turns.Max.
	EffectTurnMax EffectKind = "turn_max"
	// EffectTradeGreedReduction lowers NPC Disposition.Greed in every
	// future negotiation, accumulated on Research.TradeGreedReduction.
	EffectTradeGreedReduction EffectKind = "trade_greed_reduction"
	// EffectResearchRate increases points accrued per tick toward future
	// research, accumulated on Research.RateBonus.
	EffectResearchRate EffectKind = "research_rate"
)

// Effect is the single permanent change a completed Tech grants.
type Effect struct {
	Kind      EffectKind
	Magnitude int
}

// Tech is one node in the tree: a cost in research points and a
// Prerequisite (empty for a branch root) that must be unlocked first.
type Tech struct {
	ID           TechID
	Name         string
	Description  string
	Tier         int
	Cost         int
	Prerequisite TechID
	Effect       Effect
}

// branch is the data-driven definition of one linear chain of tiers within
// Catalog; adding a branch or changing its tier count only touches this
// table, never the tick/persistence machinery.
type branch struct {
	prefix           string
	name             string
	description      string
	kind             EffectKind
	magnitudePerTier int
}

var branches = []branch{
	{"cargo", "Cargo Expansion", "Reinforced holds carry more cargo per trip.", EffectCargoCapacity, 5},
	{"logistics", "Turn Efficiency", "Streamlined operations stretch every turn allowance further.", EffectTurnMax, 3},
	{"trade", "Trade Contacts", "Better market intelligence narrows what NPCs can get away with.", EffectTradeGreedReduction, 2},
	{"research", "Research Methods", "Better labs accrue research points faster.", EffectResearchRate, 1},
}

const (
	tiersPerBranch = 6
	costPerTier    = 40
)

// Catalog is the fixed, data-driven tech content: four branches of six
// tiers each (24 techs total), gated linearly — tier N requires tier N-1 of
// the same branch, and a branch's tier 1 has no prerequisite.
var Catalog = buildCatalog()

func buildCatalog() []Tech {
	var out []Tech
	for _, b := range branches {
		var prev TechID
		for tier := 1; tier <= tiersPerBranch; tier++ {
			id := TechID(fmt.Sprintf("%s-%d", b.prefix, tier))
			out = append(out, Tech{
				ID:           id,
				Name:         fmt.Sprintf("%s %s", b.name, roman(tier)),
				Description:  b.description,
				Tier:         tier,
				Cost:         costPerTier * tier,
				Prerequisite: prev,
				Effect:       Effect{Kind: b.kind, Magnitude: b.magnitudePerTier * tier},
			})
			prev = id
		}
	}
	return out
}

var romanNumerals = [...]string{"", "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X"}

func roman(tier int) string {
	if tier < len(romanNumerals) {
		return romanNumerals[tier]
	}
	return fmt.Sprintf("%d", tier)
}

// Find looks up a tech by ID.
func Find(id TechID) (Tech, bool) {
	for _, t := range Catalog {
		if t.ID == id {
			return t, true
		}
	}
	return Tech{}, false
}

// ErrNotResearchable is returned by Start when the requested tech doesn't
// exist, is already unlocked, or its prerequisite isn't unlocked yet.
var ErrNotResearchable = errors.New("techtree: tech is not researchable")

const (
	tickInterval    = 30 * time.Second
	baseRatePerTick = 4
)

// Research tracks the player's active research project, its accumulated
// progress, every tech unlocked so far, and the cumulative bonuses granted
// by completed techs whose effect isn't a direct player.Player field.
type Research struct {
	Active              TechID
	Progress            int
	LastTickAt          time.Time
	Unlocked            map[TechID]bool
	RateBonus           int
	TradeGreedReduction int
}

// HasUnlocked reports whether id has already been researched.
func (r Research) HasUnlocked(id TechID) bool {
	return r.Unlocked[id]
}

// Available reports whether id can be started: it exists, isn't already
// unlocked, and its prerequisite (if any) is unlocked.
func (r Research) Available(id TechID) bool {
	t, ok := Find(id)
	if !ok || r.Unlocked[id] {
		return false
	}
	return t.Prerequisite == "" || r.Unlocked[t.Prerequisite]
}

// RatePerTick is the research points accrued per elapsed tick toward the
// active project, boosted by every completed Research Methods tech.
func (r Research) RatePerTick() int {
	return baseRatePerTick + r.RateBonus
}

// Start begins researching id, replacing any in-progress project — its
// accumulated Progress is lost, since only one project can be active at a
// time. Returns ErrNotResearchable if id isn't currently available.
func (r Research) Start(id TechID, now time.Time) (Research, error) {
	if !r.Available(id) {
		return r, ErrNotResearchable
	}
	r.Active = id
	r.Progress = 0
	r.LastTickAt = now
	return r, nil
}

// Ticked advances r to now, accruing RatePerTick points per elapsed tick
// toward the active project. It returns the updated Research, how many
// ticks elapsed (0 if none, in which case the caller should treat r as
// unchanged), and the TechID of any tech that completed as a result (empty
// if none). At most one tech completes per call: once a project completes,
// Active is cleared and any leftover elapsed ticks are simply not spent —
// the player must explicitly choose what to research next.
func (r Research) Ticked(now time.Time) (Research, int, TechID) {
	if r.Active == "" || !now.After(r.LastTickAt) {
		return r, 0, ""
	}
	ticks := int(now.Sub(r.LastTickAt) / tickInterval)
	if ticks <= 0 {
		return r, 0, ""
	}
	r.LastTickAt = r.LastTickAt.Add(time.Duration(ticks) * tickInterval)

	tech, ok := Find(r.Active)
	if !ok {
		r.Active = ""
		r.Progress = 0
		return r, ticks, ""
	}

	r.Progress += ticks * r.RatePerTick()
	if r.Progress < tech.Cost {
		return r, ticks, ""
	}

	if r.Unlocked == nil {
		r.Unlocked = map[TechID]bool{}
	}
	r.Unlocked[tech.ID] = true
	switch tech.Effect.Kind {
	case EffectResearchRate:
		r.RateBonus += tech.Effect.Magnitude
	case EffectTradeGreedReduction:
		r.TradeGreedReduction += tech.Effect.Magnitude
	}
	r.Active = ""
	r.Progress = 0
	return r, ticks, tech.ID
}
