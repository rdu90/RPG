package player

import (
	"testing"

	"github.com/rdu90/RPG/internal/engine/economy"
	"github.com/rdu90/RPG/internal/engine/galaxy"
)

func TestReputationAtDefaultsToZero(t *testing.T) {
	p := Player{Reputation: map[galaxy.NodeID]int{}}
	if got := p.ReputationAt("sys-000"); got != 0 {
		t.Fatalf("expected 0 for an unvisited system, got %d", got)
	}
}

func TestHasDiscoveredAndHasClaimedAnomalyDefaultFalse(t *testing.T) {
	p := Player{
		Discovered:       map[galaxy.NodeID]bool{"sys-000": true},
		ClaimedAnomalies: map[galaxy.NodeID]bool{"sys-000": true},
	}
	if !p.HasDiscovered("sys-000") {
		t.Fatal("expected sys-000 to be discovered")
	}
	if p.HasDiscovered("sys-001") {
		t.Fatal("expected an unsurveyed system to not be discovered")
	}
	if !p.HasClaimedAnomaly("sys-000") {
		t.Fatal("expected sys-000's anomaly to be claimed")
	}
	if p.HasClaimedAnomaly("sys-001") {
		t.Fatal("expected an unvisited system's anomaly to not be claimed")
	}
}

func TestAlignmentNudgeMovesTowardContribution(t *testing.T) {
	var a Alignment
	for i := 0; i < 50; i++ {
		a = a.Nudge(ContributionFor(economy.CategoryImmoral))
	}
	if a.Morality > -0.9 {
		t.Fatalf("expected sustained immoral trading to pull morality near -1, got %v", a.Morality)
	}
	if a.Legality != 0 {
		t.Fatalf("expected immoral trades to leave legality untouched, got %v", a.Legality)
	}
}

func TestAlignmentNudgeIsGradualNotInstant(t *testing.T) {
	a := Alignment{Legality: 1, Morality: 1}
	a = a.Nudge(ContributionFor(economy.CategoryIllegal))
	if a.Legality <= -1 || a.Legality >= 1 {
		t.Fatalf("expected a single trade to move alignment gradually, got %v", a.Legality)
	}
}
