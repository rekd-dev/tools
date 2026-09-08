package analyzers

import (
	"testing"

	"repo-context-cli/internal/model"
)

func TestMergeKeepsAllEvidenceAndPicksHighestConfidence(t *testing.T) {
	from := model.TypeID("csharp", "Api/C.cs", "Controller")
	to := model.TypeID("csharp", "App/IRepo.cs", "IRepo")
	low := model.Graph{
		Nodes: []model.Node{{ID: from, Kind: model.KindType, Name: "Controller"}, {ID: to, Kind: model.KindType, Name: "IRepo"}},
		Edges: []model.Edge{{
			FromID: from, ToID: to, Kind: model.RelInjects,
			Source: model.SrcHeuristic, Confidence: 0.4, Analyzer: "heuristic", File: "Api/C.cs", Line: 9,
		}},
	}
	high := model.Graph{
		Edges: []model.Edge{{
			FromID: from, ToID: to, Kind: model.RelInjects,
			Source: model.SrcRoslyn, Confidence: 0.95, Analyzer: "roslyn", File: "Api/C.cs", Line: 9,
		}},
	}
	g := Merge(low, high)
	if len(g.Edges) != 1 {
		t.Fatalf("edges=%d", len(g.Edges))
	}
	e := g.Edges[0]
	if e.Confidence != 0.95 || e.Source != model.SrcRoslyn {
		t.Fatalf("%#v", e)
	}
	if len(e.Evidence) != 2 {
		t.Fatalf("evidence=%d", len(e.Evidence))
	}
}
