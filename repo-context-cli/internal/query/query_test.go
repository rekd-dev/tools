package query

import (
	"testing"

	"repo-context-cli/internal/model"
)

func TestSearchEmptyQueryIsEmptySlice(t *testing.T) {
	hits := Search(model.Graph{}, "", 10)
	if hits == nil {
		t.Fatal("nil")
	}
	if len(hits) != 0 {
		t.Fatalf("len=%d", len(hits))
	}
}

func TestSearchRanksExactNameFirst(t *testing.T) {
	g := model.Graph{Nodes: []model.Node{
		{ID: "type:csharp:a.cs:IEntityRepository", Kind: "type", Name: "IEntityRepository", File: "a.cs"},
		{ID: "type:csharp:b.cs:Entity", Kind: "type", Name: "Entity", File: "b.cs"},
	}}
	hits := Search(g, "IEntityRepository", 10)
	if len(hits) == 0 || hits[0].Name != "IEntityRepository" {
		t.Fatalf("%#v", hits)
	}
}

func TestSearchRanksEndpointAboveSameNamedFile(t *testing.T) {
	g := model.Graph{Nodes: []model.Node{
		{ID: "file:ts:clock-in.ts", Kind: "file", Name: "clock-in.ts", File: "clock-in.ts"},
		{ID: "endpoint:ts:r.ts:POST:/api/me/time/clock-in", Kind: "endpoint", Name: "POST /api/me/time/clock-in", File: "r.ts"},
		{ID: "type:ts:clock-in.ts:ClockInUseCase", Kind: "type", Name: "ClockInUseCase", File: "clock-in.ts"},
	}}
	hits := Search(g, "clock-in", 10)
	if len(hits) == 0 || hits[0].Kind != "endpoint" {
		t.Fatalf("%#v", hits)
	}
}

func TestBoundedPathStopsAtDepth(t *testing.T) {
	a := "a"
	b := "b"
	c := "c"
	g := model.Graph{
		Nodes: []model.Node{{ID: a, Name: "A"}, {ID: b, Name: "B"}, {ID: c, Name: "C"}},
		Edges: []model.Edge{
			{ID: "1", FromID: a, ToID: b, Kind: model.RelCalls},
			{ID: "2", FromID: b, ToID: c, Kind: model.RelCalls},
		},
	}
	r := BoundedPath(g, PathRequest{From: a, MaxDepth: 1})
	if len(r.Nodes) != 2 {
		t.Fatalf("nodes=%d", len(r.Nodes))
	}
}
