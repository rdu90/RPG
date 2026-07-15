package companion

import (
	"strings"
	"testing"
)

func TestColonyHintAffordable(t *testing.T) {
	s := ColonyHint(true, 1600, 5, 0)
	if !containsAll(s, "1600 credits", "5 turns", "p at your convenience") {
		t.Fatalf("expected affordable hint to mention cost/turns/prompt, got: %s", s)
	}
}

func TestColonyHintUnaffordable(t *testing.T) {
	s := ColonyHint(false, 1600, 5, 400)
	if !containsAll(s, "1600 credits", "5 turns", "short by 400 credits") {
		t.Fatalf("expected unaffordable hint to mention the shortfall, got: %s", s)
	}
}

type stringerKind string

func (k stringerKind) String() string { return string(k) }

func TestAlreadyInvestigated(t *testing.T) {
	s := AlreadyInvestigated(stringerKind("derelict wreck"))
	if !containsAll(s, "derelict wreck") {
		t.Fatalf("expected message to name the anomaly kind, got: %s", s)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
