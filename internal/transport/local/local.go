// Package local is the in-process transport adapter: it wraps an
// internal/engine.Engine directly, with no serialization or network hop.
// A future transport/grpc package implements the same Execute/Query shape
// against a remote engine, so callers (the TUI) never need to know which
// one they're talking to.
package local

import (
	"context"

	"github.com/rdu90/RPG/internal/engine"
)

// Client is the transport-facing handle callers above the boundary use.
type Client struct {
	eng *engine.Engine
}

// New wraps an Engine as a local Client.
func New(eng *engine.Engine) *Client {
	return &Client{eng: eng}
}

// Execute dispatches a command to the wrapped engine.
func (c *Client) Execute(ctx context.Context, cmd engine.Command) (any, error) {
	return c.eng.Execute(ctx, cmd)
}

// Query dispatches a query to the wrapped engine.
func (c *Client) Query(ctx context.Context, q engine.Query) (any, error) {
	return c.eng.Query(ctx, q)
}
