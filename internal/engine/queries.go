package engine

// GetGame returns the current save's identity record.
type GetGame struct{}

func (GetGame) isQuery() {}

// GetGalaxy returns the save's full generated galaxy graph.
type GetGalaxy struct{}

func (GetGalaxy) isQuery() {}

// GetPlayer returns the current player state.
type GetPlayer struct{}

func (GetPlayer) isQuery() {}

// GetMarket returns commodity prices at the player's current system.
type GetMarket struct{}

func (GetMarket) isQuery() {}

// GetAnomaly returns whether the player's current system hides an anomaly,
// and whether it has already been claimed.
type GetAnomaly struct{}

func (GetAnomaly) isQuery() {}

// GetColony returns the colony (if any) at the player's current system.
type GetColony struct{}

func (GetColony) isQuery() {}

// GetColonies returns every colony in the save.
type GetColonies struct{}

func (GetColonies) isQuery() {}

// GetTechTree returns the fixed tech catalog alongside the player's current
// research progress.
type GetTechTree struct{}

func (GetTechTree) isQuery() {}
