// Package haggle implements the multi-round price-negotiation state machine
// that replaces static per-system prices: an NPC opens away from the fair
// market price by an amount driven by its Disposition, and each round the
// player counters, bluffs a walk-away, or accepts. It is pure domain logic
// (no I/O) — Session is a plain value the caller carries from round to
// round, the same way it carries a Command.
package haggle

import (
	"math"
	"math/rand/v2"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
)

// Disposition captures how an NPC trader behaves in a negotiation, derived
// from the system's development level and the player's standing there.
type Disposition struct {
	Patience int // 0-100: higher tolerates more rounds before giving up
	Greed    int // 0-100: higher opens further from fair price and concedes less
	Trust    int // 0-100: higher (built by reputation) narrows the gap needed to close a deal and makes bluffs more likely to work
}

// NewDisposition derives an NPC's disposition from a system's development
// level (1-5) and the player's reputation at that system.
func NewDisposition(developmentLevel, reputation int) Disposition {
	trust := clamp(50+reputation, 0, 100)
	greed := clamp(70-developmentLevel*8, 10, 90)
	patience := clamp(30+trust/2, 20, 90)
	return Disposition{Patience: patience, Greed: greed, Trust: trust}
}

// maxRounds is how many rounds of back-and-forth the NPC tolerates before
// giving up on the negotiation entirely.
func (d Disposition) maxRounds() int {
	return 2 + d.Patience/20 // 2..6
}

// Outcome is the terminal (or in-progress) state of a Session.
type Outcome int

const (
	InProgress Outcome = iota
	Accepted
	Rejected
)

// String renders the outcome for display/logging.
func (o Outcome) String() string {
	switch o {
	case InProgress:
		return "in progress"
	case Accepted:
		return "accepted"
	case Rejected:
		return "rejected"
	default:
		return "unknown"
	}
}

// Reputation deltas applied once a session leaves InProgress.
const (
	acceptedReputationGain           = 1
	patienceExhaustedReputationDelta = -1
	failedBluffReputationDelta       = -3
)

// Session is one in-progress (or concluded) negotiation over Quantity units
// of a single Commodity at a single system.
type Session struct {
	NodeID      galaxy.NodeID
	Commodity   economy.CommodityID
	Buying      bool // true: player is buying from the NPC; false: player is selling to the NPC
	Quantity    int
	FairPrice   int // the underlying market price per unit
	NPCOffer    int // the NPC's current price per unit
	Round       int
	Disposition Disposition
	Outcome     Outcome

	// ReputationDelta accumulates the reputation change to apply once the
	// session leaves InProgress; the caller applies it exactly once.
	ReputationDelta int
}

// Start opens a new negotiation: the NPC's opening offer is skewed away from
// the fair market price by its greed (worse for the player buying, better
// for the NPC), jittered by r.
func Start(nodeID galaxy.NodeID, commodity economy.CommodityID, buying bool, fairPrice, qty int, disp Disposition, r *rand.Rand) Session {
	skew := 0.05 + float64(disp.Greed)/100*0.35
	jitter := 0.95 + r.Float64()*0.1
	open := float64(fairPrice) * (1 + skewSign(buying)*skew) * jitter

	return Session{
		NodeID:      nodeID,
		Commodity:   commodity,
		Buying:      buying,
		Quantity:    qty,
		FairPrice:   fairPrice,
		NPCOffer:    clampPrice(int(math.Round(open))),
		Disposition: disp,
		Outcome:     InProgress,
	}
}

// Offer proposes price per unit. The NPC accepts outright if it's at least
// as good for the NPC as its own current offer, accepts a compromise if the
// gap is within its trust-driven tolerance, counters otherwise, or gives up
// once its patience for this negotiation runs out.
func (s Session) Offer(price int, r *rand.Rand) Session {
	if s.Outcome != InProgress || price <= 0 {
		return s
	}
	s.Round++

	tolerance := int(float64(s.FairPrice) * (0.02 + float64(s.Disposition.Trust)/100*0.08))
	if tolerance < 1 {
		tolerance = 1
	}

	var gap int
	if s.Buying {
		gap = s.NPCOffer - price
	} else {
		gap = price - s.NPCOffer
	}

	switch {
	case gap <= 0:
		// Player's offer is already at least as good for the NPC as its
		// current ask; deal closes at the NPC's (more favorable to the
		// player) price.
		s.Outcome = Accepted
		s.ReputationDelta += acceptedReputationGain
		return s
	case gap <= tolerance:
		s.NPCOffer = (price + s.NPCOffer) / 2
		s.Outcome = Accepted
		s.ReputationDelta += acceptedReputationGain
		return s
	}

	concession := gap * concessionPct(s.Disposition.Greed) / 100
	if concession < 1 {
		concession = 1
	}
	if s.Buying {
		s.NPCOffer -= concession
	} else {
		s.NPCOffer += concession
	}
	s.NPCOffer = clampPrice(s.NPCOffer)

	if s.Round >= s.Disposition.maxRounds() {
		s.Outcome = Rejected
		s.ReputationDelta += patienceExhaustedReputationDelta
	}
	return s
}

// WalkAway attempts to bluff the player's way to a better price by
// threatening to leave. Higher-trust NPCs are more likely to cave and
// improve their offer; on failure, the negotiation ends and the bluff costs
// reputation.
func (s Session) WalkAway(r *rand.Rand) Session {
	if s.Outcome != InProgress {
		return s
	}
	s.Round++

	chance := 0.2 + float64(s.Disposition.Trust)/100*0.5
	if r.Float64() >= chance {
		s.Outcome = Rejected
		s.ReputationDelta += failedBluffReputationDelta
		return s
	}

	improvement := int(float64(s.FairPrice) * (0.03 + r.Float64()*0.07))
	if improvement < 1 {
		improvement = 1
	}
	if s.Buying {
		s.NPCOffer = clampPrice(s.NPCOffer - improvement)
	} else {
		s.NPCOffer += improvement
	}

	if s.Round >= s.Disposition.maxRounds() {
		s.Outcome = Rejected
		s.ReputationDelta += patienceExhaustedReputationDelta
	}
	return s
}

// Accept closes the negotiation at the NPC's current offer.
func (s Session) Accept() Session {
	if s.Outcome != InProgress {
		return s
	}
	s.Outcome = Accepted
	s.ReputationDelta += acceptedReputationGain
	return s
}

func skewSign(buying bool) float64 {
	if buying {
		return 1
	}
	return -1
}

// concessionPct is how much of the price gap the NPC gives up per round,
// inversely related to greed.
func concessionPct(greed int) int {
	return clamp(50-greed*35/100, 10, 50)
}

func clampPrice(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
