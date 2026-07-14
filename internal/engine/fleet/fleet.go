// Package fleet models the player's ship as a combat unit: Attack and
// Defense scale damage dealt and received each combat round, Hull is
// current structural integrity capped at MaxHull. It has no dependency on
// persistence or transport — Stats is a plain value, the same shape as
// haggle.Session and espionage.Spy.
package fleet

// Stats are a ship's combat capabilities.
type Stats struct {
	Attack  int
	Defense int
	Hull    int
	MaxHull int
}

// Alive reports whether the ship can still fight.
func (s Stats) Alive() bool { return s.Hull > 0 }

// Damaged reports whether the ship has taken hull damage not yet repaired.
func (s Stats) Damaged() bool { return s.Hull < s.MaxHull }

// minDamage is the least damage a hit ever deals, even against defense
// exceeding attack, so combat always progresses to a conclusion.
const minDamage = 1

// Damage returns the damage an attacker with attackerAttack deals to a
// defender with defenderDefense in a single round.
func Damage(attackerAttack, defenderDefense int) int {
	d := attackerAttack - defenderDefense
	if d < minDamage {
		d = minDamage
	}
	return d
}

// TakeDamage returns s after absorbing dmg, hull floored at 0.
func (s Stats) TakeDamage(dmg int) Stats {
	s.Hull -= dmg
	if s.Hull < 0 {
		s.Hull = 0
	}
	return s
}

// Repaired returns s with Hull restored to MaxHull.
func (s Stats) Repaired() Stats {
	s.Hull = s.MaxHull
	return s
}
