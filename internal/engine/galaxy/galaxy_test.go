package galaxy

import "testing"

func TestGenerateDeterministic(t *testing.T) {
	a := Generate(42, 20)
	b := Generate(42, 20)

	if len(a.Nodes) != len(b.Nodes) || len(a.Edges) != len(b.Edges) {
		t.Fatalf("same seed produced different sized galaxies: %d/%d vs %d/%d",
			len(a.Nodes), len(a.Edges), len(b.Nodes), len(b.Edges))
	}
	for i := range a.Nodes {
		if a.Nodes[i] != b.Nodes[i] {
			t.Fatalf("node %d differs: %+v vs %+v", i, a.Nodes[i], b.Nodes[i])
		}
	}
	for i := range a.Edges {
		if a.Edges[i] != b.Edges[i] {
			t.Fatalf("edge %d differs: %+v vs %+v", i, a.Edges[i], b.Edges[i])
		}
	}
}

func TestGenerateDifferentSeedsDiffer(t *testing.T) {
	a := Generate(1, 20)
	b := Generate(2, 20)

	same := true
	for i := range a.Nodes {
		if a.Nodes[i] != b.Nodes[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatalf("different seeds produced identical galaxies")
	}
}

func TestGenerateConnected(t *testing.T) {
	g := Generate(7, 16)
	if len(g.Nodes) != 16 {
		t.Fatalf("expected 16 nodes, got %d", len(g.Nodes))
	}

	seen := map[NodeID]bool{g.Nodes[0].ID: true}
	queue := []NodeID{g.Nodes[0].ID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range g.Neighbors(cur) {
			if !seen[e.To] {
				seen[e.To] = true
				queue = append(queue, e.To)
			}
		}
	}

	for _, n := range g.Nodes {
		if !seen[n.ID] {
			t.Errorf("node %s unreachable from %s", n.ID, g.Nodes[0].ID)
		}
	}
}

func TestGenerateEdgesPositiveCost(t *testing.T) {
	g := Generate(99, 12)
	if len(g.Edges) == 0 {
		t.Fatal("expected at least one edge")
	}
	for _, e := range g.Edges {
		if e.TurnCost < 1 {
			t.Errorf("edge %s->%s has non-positive turn cost %d", e.From, e.To, e.TurnCost)
		}
	}
}

func TestEdgeBetweenBothDirections(t *testing.T) {
	g := Generate(5, 10)
	e := g.Edges[0]

	if _, ok := g.EdgeBetween(e.From, e.To); !ok {
		t.Fatalf("expected EdgeBetween(%s, %s) to be found", e.From, e.To)
	}
	if _, ok := g.EdgeBetween(e.To, e.From); !ok {
		t.Fatalf("expected EdgeBetween(%s, %s) to be found", e.To, e.From)
	}
}

func TestNeighborsNormalizesDirection(t *testing.T) {
	g := Generate(5, 10)
	e := g.Edges[0]

	for _, n := range g.Neighbors(e.From) {
		if n.To == e.To {
			return
		}
	}
	t.Fatalf("expected Neighbors(%s) to include %s", e.From, e.To)
}
