package query

import (
	"testing"

	"repo-context-cli/internal/model"
)

func TestBoundedPathSkipsUnresolvedAndLowConfidence(t *testing.T) {
	start := "type:ts:a.ts:UseCase"
	real := "method:ts:a.ts:Repo.save"
	phantom := "method:ts:lib.ts:Math.floor"
	g := model.Graph{
		Nodes: []model.Node{
			{ID: start, Name: "UseCase"},
			{ID: real, Name: "save"},
			{ID: phantom, Name: "floor"},
		},
		Edges: []model.Edge{
			{ID: "u", FromID: start, ToID: phantom, Kind: model.RelCalls, Confidence: 0.25, Unresolved: true},
			{ID: "ok", FromID: start, ToID: real, Kind: model.RelCalls, Confidence: 0.92},
		},
	}

	t.Run("skip unresolved does not consume MaxNodes", func(t *testing.T) {
		r := BoundedPath(g, PathRequest{From: start, MaxNodes: 2, SkipUnresolved: true, MinConfidence: 0.5})
		ids := nodeIDs(r)
		if ids[phantom] {
			t.Fatalf("unresolved phantom should be skipped: %#v", ids)
		}
		if !ids[real] {
			t.Fatalf("resolved call should appear: %#v", ids)
		}
		if r.Truncated {
			t.Fatal("skipped edges must not consume MaxNodes")
		}
		if edgeIDs(r)["u"] {
			t.Fatal("unresolved edge should not be walked")
		}
	})

	t.Run("zero confidence still allowed", func(t *testing.T) {
		zero := "method:ts:a.ts:local"
		g2 := g
		g2.Nodes = append(g2.Nodes, model.Node{ID: zero, Name: "local"})
		g2.Edges = append(g2.Edges, model.Edge{ID: "z", FromID: start, ToID: zero, Kind: model.RelCalls})
		r := BoundedPath(g2, PathRequest{From: start, SkipUnresolved: true, MinConfidence: 0.5})
		if !nodeIDs(r)[zero] {
			t.Fatal("missing/zero confidence should be allowed")
		}
	})

	t.Run("unresolved opt-in walks phantom", func(t *testing.T) {
		r := BoundedPath(g, PathRequest{From: start, SkipUnresolved: false, MinConfidence: 0})
		if !nodeIDs(r)[phantom] || !edgeIDs(r)["u"] {
			t.Fatalf("expected phantom when not skipping: %#v", nodeIDs(r))
		}
	})
}

func TestBoundedPathUnknownTargetEmptyEdges(t *testing.T) {
	a := "a"
	b := "b"
	g := model.Graph{
		Nodes: []model.Node{{ID: a, Name: "A"}, {ID: b, Name: "B"}},
		Edges: []model.Edge{{ID: "1", FromID: a, ToID: b, Kind: model.RelCalls, Confidence: 0.92}},
	}
	r := BoundedPath(g, PathRequest{From: a, To: "missing"})
	if r.Edges == nil {
		t.Fatal("edges must be empty slice, not nil")
	}
	if len(r.Edges) != 0 {
		t.Fatalf("len=%d", len(r.Edges))
	}
}

func nodeIDs(r PathResult) map[string]bool {
	out := map[string]bool{}
	for _, n := range r.Nodes {
		out[n.ID] = true
	}
	return out
}

func edgeIDs(r PathResult) map[string]bool {
	out := map[string]bool{}
	for _, e := range r.Edges {
		out[e.ID] = true
	}
	return out
}
