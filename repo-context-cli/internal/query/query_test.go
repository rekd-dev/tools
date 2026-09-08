package query

import (
	"testing"

	"repo-context-cli/internal/model"
)

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
