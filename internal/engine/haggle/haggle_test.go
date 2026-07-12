package haggle

import (
	"testing"

	"github.com/rdu90/RPG/internal/rng"
)

const fairPrice = 100

func TestNewDispositionRewardsReputation(t *testing.T) {
	low := NewDisposition(3, -40)
	high := NewDisposition(3, 40)
	if high.Trust <= low.Trust {
		t.Fatalf("expected higher reputation to raise trust: low=%d high=%d", low.Trust, high.Trust)
	}
}

func TestStartSkewsAgainstThePlayer(t *testing.T) {
	r := rng.New(1)
	buy := Start("sys-000", "food", true, fairPrice, 5, NewDisposition(3, 0), r)
	if buy.NPCOffer <= fairPrice {
		t.Fatalf("expected buying to open above fair price %d, got %d", fairPrice, buy.NPCOffer)
	}

	r = rng.New(1)
	sell := Start("sys-000", "food", false, fairPrice, 5, NewDisposition(3, 0), r)
	if sell.NPCOffer >= fairPrice {
		t.Fatalf("expected selling to open below fair price %d, got %d", fairPrice, sell.NPCOffer)
	}
}

func TestStartIsDeterministic(t *testing.T) {
	disp := NewDisposition(3, 10)
	a := Start("sys-000", "food", true, fairPrice, 5, disp, rng.New(42))
	b := Start("sys-000", "food", true, fairPrice, 5, disp, rng.New(42))
	if a != b {
		t.Fatalf("expected same seed to produce identical sessions: %+v vs %+v", a, b)
	}
}

func TestOfferAcceptsAnOfferAtLeastAsGoodAsNPCs(t *testing.T) {
	r := rng.New(2)
	s := Start("sys-000", "food", true, fairPrice, 5, NewDisposition(3, 0), r)
	s = s.Offer(s.NPCOffer, r) // matching the NPC's own ask should always close
	if s.Outcome != Accepted {
		t.Fatalf("expected Accepted, got %v", s.Outcome)
	}
	if s.ReputationDelta <= 0 {
		t.Fatalf("expected a reputation gain on acceptance, got %d", s.ReputationDelta)
	}
}

func TestOfferCountersOutsideTolerance(t *testing.T) {
	r := rng.New(3)
	s := Start("sys-000", "food", true, fairPrice, 5, NewDisposition(3, 0), r)
	opening := s.NPCOffer
	// A lowball offer should provoke a counter that concedes toward it
	// without closing the deal in one round.
	s = s.Offer(1, r)
	if s.Outcome != InProgress {
		t.Fatalf("expected negotiation to continue, got %v", s.Outcome)
	}
	if s.NPCOffer >= opening {
		t.Fatalf("expected NPC to concede from opening %d, got %d", opening, s.NPCOffer)
	}
	if s.Round != 1 {
		t.Fatalf("expected round 1, got %d", s.Round)
	}
}

func TestOfferGivesUpAfterMaxRounds(t *testing.T) {
	disp := NewDisposition(3, 0)
	r := rng.New(4)
	s := Start("sys-000", "food", true, fairPrice, 5, disp, r)
	for i := 0; i < disp.maxRounds() && s.Outcome == InProgress; i++ {
		s = s.Offer(1, r) // relentless lowball, never within tolerance
	}
	if s.Outcome != Rejected {
		t.Fatalf("expected Rejected after exhausting patience, got %v", s.Outcome)
	}
	if s.ReputationDelta >= 0 {
		t.Fatalf("expected a reputation penalty for exhausting the NPC's patience, got %d", s.ReputationDelta)
	}
}

func TestWalkAwayEitherImprovesOrEndsWithPenalty(t *testing.T) {
	disp := NewDisposition(3, 0)
	for seed := int64(0); seed < 200; seed++ {
		r := rng.New(seed)
		s := Start("sys-000", "food", true, fairPrice, 5, disp, r)
		opening := s.NPCOffer
		s = s.WalkAway(r)

		switch s.Outcome {
		case InProgress:
			if s.NPCOffer >= opening {
				t.Fatalf("seed %d: expected walk-away success to improve the offer below %d, got %d", seed, opening, s.NPCOffer)
			}
		case Rejected:
			if s.ReputationDelta >= 0 {
				t.Fatalf("seed %d: expected a reputation penalty for a failed bluff, got %d", seed, s.ReputationDelta)
			}
		default:
			t.Fatalf("seed %d: unexpected outcome %v after WalkAway", seed, s.Outcome)
		}
	}
}

func TestAcceptClosesAtCurrentOffer(t *testing.T) {
	r := rng.New(5)
	s := Start("sys-000", "food", true, fairPrice, 5, NewDisposition(3, 0), r)
	offer := s.NPCOffer
	s = s.Accept()
	if s.Outcome != Accepted {
		t.Fatalf("expected Accepted, got %v", s.Outcome)
	}
	if s.NPCOffer != offer {
		t.Fatalf("expected accept to close at the standing offer %d, got %d", offer, s.NPCOffer)
	}
	if s.ReputationDelta <= 0 {
		t.Fatalf("expected a reputation gain on acceptance, got %d", s.ReputationDelta)
	}
}

func TestActionsOnConcludedSessionAreNoops(t *testing.T) {
	r := rng.New(6)
	s := Start("sys-000", "food", true, fairPrice, 5, NewDisposition(3, 0), r)
	s = s.Accept()
	concluded := s

	if got := s.Offer(1, r); got != concluded {
		t.Fatalf("expected Offer on a concluded session to be a no-op")
	}
	if got := s.WalkAway(r); got != concluded {
		t.Fatalf("expected WalkAway on a concluded session to be a no-op")
	}
	if got := s.Accept(); got != concluded {
		t.Fatalf("expected Accept on a concluded session to be a no-op")
	}
}

func TestSellingOfferMovesTowardHigherPrice(t *testing.T) {
	r := rng.New(7)
	s := Start("sys-000", "food", false, fairPrice, 5, NewDisposition(3, 0), r)
	opening := s.NPCOffer
	s = s.Offer(fairPrice*2, r) // ambitious ask, should provoke an upward counter
	if s.Outcome != InProgress {
		t.Fatalf("expected negotiation to continue, got %v", s.Outcome)
	}
	if s.NPCOffer <= opening {
		t.Fatalf("expected NPC to concede upward from opening %d, got %d", opening, s.NPCOffer)
	}
}
