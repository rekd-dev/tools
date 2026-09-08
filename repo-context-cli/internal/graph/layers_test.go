package graph

import "testing"

func TestAssignLayersSimpleChain(t *testing.T) {
	nodes := []string{"domain", "app", "infra"}
	edges := []Edge{
		{From: "app", To: "domain"},
		{From: "infra", To: "app"},
	}
	r := AssignLayers(nodes, edges)
	if r.Layers["domain"] != 0 {
		t.Fatalf("domain want layer 0, got %d", r.Layers["domain"])
	}
	if r.Layers["app"] != 1 {
		t.Fatalf("app want layer 1, got %d", r.Layers["app"])
	}
	if r.Layers["infra"] != 2 {
		t.Fatalf("infra want layer 2, got %d", r.Layers["infra"])
	}
}

func TestAssignLayersDetectsCycle(t *testing.T) {
	nodes := []string{"a", "b", "c"}
	edges := []Edge{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
		{From: "c", To: "a"},
	}
	r := AssignLayers(nodes, edges)
	if len(r.Cycles) == 0 {
		t.Fatal("expected a cycle")
	}
	if len(r.CycleEdges) == 0 {
		t.Fatal("expected cycle edges")
	}
}
