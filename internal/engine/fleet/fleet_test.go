package fleet

import "testing"

func TestDamageFloorsAtMinimum(t *testing.T) {
	if d := Damage(5, 20); d != minDamage {
		t.Fatalf("expected damage to floor at %d, got %d", minDamage, d)
	}
}

func TestDamageSubtractsDefense(t *testing.T) {
	if d := Damage(20, 5); d != 15 {
		t.Fatalf("expected 15, got %d", d)
	}
}

func TestTakeDamageFloorsAtZero(t *testing.T) {
	s := Stats{Hull: 10, MaxHull: 10}
	s = s.TakeDamage(50)
	if s.Hull != 0 {
		t.Fatalf("expected hull to floor at 0, got %d", s.Hull)
	}
	if s.Alive() {
		t.Fatal("expected a ship at 0 hull to not be alive")
	}
}

func TestTakeDamagePartial(t *testing.T) {
	s := Stats{Hull: 10, MaxHull: 10}
	s = s.TakeDamage(4)
	if s.Hull != 6 {
		t.Fatalf("expected 6, got %d", s.Hull)
	}
	if !s.Alive() {
		t.Fatal("expected a ship with hull remaining to be alive")
	}
	if !s.Damaged() {
		t.Fatal("expected a ship below max hull to be damaged")
	}
}

func TestRepairedRestoresMaxHull(t *testing.T) {
	s := Stats{Hull: 3, MaxHull: 40}
	s = s.Repaired()
	if s.Hull != 40 {
		t.Fatalf("expected hull restored to 40, got %d", s.Hull)
	}
	if s.Damaged() {
		t.Fatal("expected a fully repaired ship to not be damaged")
	}
}
