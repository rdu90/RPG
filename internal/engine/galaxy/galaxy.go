// Package galaxy models the sparse coordinate-node graph a save's galaxy is
// built from: star systems (Node) connected by warp lanes (Edge). It has no
// dependency on persistence or transport — Generate is a pure function of a
// seed, so the same seed always produces the same galaxy.
package galaxy

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/rdu90/RPG/internal/rng"
)

// NodeID identifies a star system within a galaxy.
type NodeID string

// Node is a single star system: a point in coordinate space with a
// development level driving what its market produces and needs.
type Node struct {
	ID               NodeID
	Name             string
	X, Y             int
	DevelopmentLevel int // 1 (frontier outpost) .. 5 (core world)
}

// Edge is a warp lane between two systems, costing TurnCost turns to
// traverse in either direction.
type Edge struct {
	From, To NodeID
	TurnCost int
}

// Galaxy is the full generated node graph for a save.
type Galaxy struct {
	Nodes []Node
	Edges []Edge
}

// Node returns the node with the given ID, if present.
func (g Galaxy) Node(id NodeID) (Node, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

// EdgeBetween returns the warp lane connecting a and b, in either direction.
func (g Galaxy) EdgeBetween(a, b NodeID) (Edge, bool) {
	for _, e := range g.Edges {
		if (e.From == a && e.To == b) || (e.From == b && e.To == a) {
			return e, true
		}
	}
	return Edge{}, false
}

// Neighbors returns the edges touching id, normalized so e.From is always
// id and e.To is the neighboring node.
func (g Galaxy) Neighbors(id NodeID) []Edge {
	var out []Edge
	for _, e := range g.Edges {
		switch id {
		case e.From:
			out = append(out, e)
		case e.To:
			out = append(out, Edge{From: e.To, To: e.From, TurnCost: e.TurnCost})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].To < out[j].To })
	return out
}

const (
	coordMax = 99

	// extraEdgeChance is the probability, per candidate pair beyond the
	// spanning tree, that an additional short warp lane is added for route
	// variety (so the galaxy isn't a single linear path).
	extraEdgeChance = 0.35
)

var starNames = []string{
	"Aldrin", "Bellatrix", "Castor", "Draco", "Elysium", "Farsight", "Gilgamesh",
	"Helios", "Icarus", "Juno", "Kepler", "Lyra", "Meridian", "Nyx", "Osiris",
	"Perseus", "Quasar", "Rigel", "Solon", "Tethys", "Umbriel", "Vega", "Wyrm",
	"Xanadu", "Ymir", "Zenith",
}

// Generate deterministically builds a galaxy of size systems from seed: the
// same seed and size always produce the same nodes and warp lanes.
func Generate(seed int64, size int) Galaxy {
	r := rng.New(seed)

	nodes := make([]Node, 0, size)
	occupied := make(map[[2]int]bool, size)
	for i := 0; i < size; i++ {
		var x, y int
		for {
			x, y = r.IntN(coordMax+1), r.IntN(coordMax+1)
			if !occupied[[2]int{x, y}] {
				break
			}
		}
		occupied[[2]int{x, y}] = true

		nodes = append(nodes, Node{
			ID:               NodeID(fmt.Sprintf("sys-%03d", i)),
			Name:             nodeName(i),
			X:                x,
			Y:                y,
			DevelopmentLevel: developmentLevel(r),
		})
	}

	edges := spanningTree(nodes)
	edges = append(edges, extraLanes(r, nodes, edges)...)

	return Galaxy{Nodes: nodes, Edges: edges}
}

func nodeName(i int) string {
	base := starNames[i%len(starNames)]
	if i < len(starNames) {
		return base
	}
	return fmt.Sprintf("%s-%d", base, i/len(starNames)+1)
}

// developmentLevel weights toward the mid-range (2-4) with core worlds (5)
// and raw frontier (1) rarer, giving markets more texture than a uniform
// distribution would.
func developmentLevel(r *rand.Rand) int {
	weights := [5]int{1, 3, 4, 3, 1} // level 1..5
	total := 0
	for _, w := range weights {
		total += w
	}
	roll := r.IntN(total)
	for level, w := range weights {
		if roll < w {
			return level + 1
		}
		roll -= w
	}
	return 3
}

// turnCost converts Euclidean distance between two systems into a turn
// cost, always at least 1.
func turnCost(a, b Node) int {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	dist := math.Sqrt(dx*dx + dy*dy)
	cost := int(math.Round(dist / 12))
	if cost < 1 {
		cost = 1
	}
	return cost
}

// spanningTree connects every node using Prim's algorithm over Euclidean
// distance, guaranteeing the galaxy is fully connected. Ties are broken
// deterministically by node order, so no randomness is needed here.
func spanningTree(nodes []Node) []Edge {
	if len(nodes) < 2 {
		return nil
	}

	inTree := map[NodeID]bool{nodes[0].ID: true}
	edges := make([]Edge, 0, len(nodes)-1)

	for len(inTree) < len(nodes) {
		var best Edge
		bestDist := math.MaxFloat64
		found := false

		for _, a := range nodes {
			if !inTree[a.ID] {
				continue
			}
			for _, b := range nodes {
				if inTree[b.ID] {
					continue
				}
				dx := float64(a.X - b.X)
				dy := float64(a.Y - b.Y)
				dist := dx*dx + dy*dy
				if dist < bestDist {
					bestDist = dist
					best = Edge{From: a.ID, To: b.ID, TurnCost: turnCost(a, b)}
					found = true
				}
			}
		}

		if !found {
			break
		}
		edges = append(edges, best)
		inTree[best.To] = true
	}

	return edges
}

// extraLanes adds a handful of additional short warp lanes on top of the
// spanning tree so the galaxy has route choices instead of a single path.
func extraLanes(r *rand.Rand, nodes []Node, existing []Edge) []Edge {
	has := make(map[[2]NodeID]bool, len(existing))
	for _, e := range existing {
		has[[2]NodeID{e.From, e.To}] = true
		has[[2]NodeID{e.To, e.From}] = true
	}

	var extra []Edge
	for i, a := range nodes {
		for _, b := range nodes[i+1:] {
			if has[[2]NodeID{a.ID, b.ID}] {
				continue
			}
			dx := float64(a.X - b.X)
			dy := float64(a.Y - b.Y)
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist > 25 {
				continue // only nearby systems get bonus lanes
			}
			if r.Float64() < extraEdgeChance {
				extra = append(extra, Edge{From: a.ID, To: b.ID, TurnCost: turnCost(a, b)})
				has[[2]NodeID{a.ID, b.ID}] = true
				has[[2]NodeID{b.ID, a.ID}] = true
			}
		}
	}
	return extra
}
