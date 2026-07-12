package engine

// GetGame returns the current save's identity record.
type GetGame struct{}

func (GetGame) isQuery() {}
