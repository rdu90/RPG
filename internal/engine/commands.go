package engine

// CreateGame initializes a brand-new save with the given name.
type CreateGame struct {
	Name string
}

func (CreateGame) isCommand() {}
