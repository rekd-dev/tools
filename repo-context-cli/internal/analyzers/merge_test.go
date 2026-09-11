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

func TestMergePrefersDeclarationMethodNameOverCallSite(t *testing.T) {
	id := model.MethodID("typescript", "a.ts", "Clock", "now")
	callSite := model.Graph{
		Nodes: []model.Node{{
			ID: id, Kind: model.KindMethod, Name: "this.deps.clock.now", File: "other.ts",
		}},
	}
	decl := model.Graph{
		Nodes: []model.Node{{
			ID: id, Kind: model.KindMethod, Name: "now", File: "a.ts", Line: 12,
		}},
	}
	g := Merge(callSite, decl)
	if len(g.Nodes) != 1 {
		t.Fatalf("nodes=%d", len(g.Nodes))
	}
	got := g.Nodes[0].Name
	if got != "Clock.now" && got != "now" {
		t.Fatalf("Name=%q want Clock.now or now", got)
	}
	if g.Nodes[0].ID != id {
		t.Fatalf("ID mutated: %s", g.Nodes[0].ID)
	}
}

func TestMergeReplacesMethodNameWithNewlineOrSQL(t *testing.T) {
	id := model.MethodID("typescript", "db.ts", "", "queryUsers")
	blob := "SELECT * FROM users\nWHERE id = $1"
	g := Merge(model.Graph{Nodes: []model.Node{{
		ID: id, Kind: model.KindMethod, Name: blob,
	}}})
	if g.Nodes[0].Name != "queryUsers" {
		t.Fatalf("Name=%q want id tail queryUsers", g.Nodes[0].Name)
	}

	paren := Merge(model.Graph{Nodes: []model.Node{{
		ID: id, Kind: model.KindMethod, Name: "sql`SELECT foo()`",
	}}})
	if paren.Nodes[0].Name != "queryUsers" {
		t.Fatalf("paren Name=%q want queryUsers", paren.Nodes[0].Name)
	}
}

func TestMergeRewritesLoneCallSiteThisName(t *testing.T) {
	id := model.MethodID("typescript", "a.ts", "Clock", "now")
	g := Merge(model.Graph{Nodes: []model.Node{{
		ID: id, Kind: model.KindMethod, Name: "this.deps.clock.now", File: "a.ts", Line: 4,
	}}})
	if g.Nodes[0].Name != "Clock.now" {
		t.Fatalf("Name=%q want Clock.now", g.Nodes[0].Name)
	}
}

func TestMergeEmptyMethodNameUsesIDTail(t *testing.T) {
	id := model.MethodID("typescript", "util.ts", "", "clockHourInZone")
	g := Merge(model.Graph{Nodes: []model.Node{{
		ID: id, Kind: model.KindMethod, Name: "   ",
	}}})
	if g.Nodes[0].Name != "clockHourInZone" {
		t.Fatalf("Name=%q want clockHourInZone", g.Nodes[0].Name)
	}
}

func TestMergeLeavesNonMethodNamesUnchanged(t *testing.T) {
	id := model.TypeID("typescript", "a.ts", "Clock")
	name := "this.deps.clock.now\nSELECT foo()"
	g := Merge(model.Graph{Nodes: []model.Node{{
		ID: id, Kind: model.KindType, Name: name,
	}}})
	if g.Nodes[0].Name != name {
		t.Fatalf("Name=%q want unchanged", g.Nodes[0].Name)
	}
}
