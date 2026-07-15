// Package faction is a small data-driven catalog of rival human factions
// that can hold colonies in the galaxy. It is presentation/identity data
// only — no behavior beyond lookup; combat and ownership logic live in
// internal/engine/colony and internal/engine/combat.
package faction

// Faction is a rival power capable of owning a colony.
type Faction struct {
	ID   string
	Name string
}

// Catalog is the fixed set of rival factions a colony can be seeded under.
// Names deliberately omit a leading "The": callers that need one supply it
// in the surrounding sentence, so a name never doubles up ("the The Crimson
// Hand").
var Catalog = []Faction{
	{ID: "crimson-hand", Name: "Crimson Hand"},
	{ID: "free-traders", Name: "Free Traders Coalition"},
	{ID: "iron-concord", Name: "Iron Concord"},
	{ID: "veiled-syndicate", Name: "Veiled Syndicate"},
	{ID: "solar-dominion", Name: "Solar Dominion"},
}

// Find looks up a faction definition by ID.
func Find(id string) (Faction, bool) {
	for _, f := range Catalog {
		if f.ID == id {
			return f, true
		}
	}
	return Faction{}, false
}
