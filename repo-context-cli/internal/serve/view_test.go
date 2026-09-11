package serve

import (
	"encoding/json"
	"testing"

	"repo-context-cli/internal/modules"
)

func TestProjectViewEmptySlicesJSON(t *testing.T) {
	v := projectView(modules.Graph{}, "missing")
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"nodes", "edges", "cycles", "childPaths"} {
		s := string(decoded[key])
		if s == "null" {
			t.Fatalf("%s marshaled as null", key)
		}
	}
}

func TestProjectViewDrillKeepsChildren(t *testing.T) {
	g := modules.Graph{
		Modules: []modules.Module{
			{ID: "a", Path: "src/application/create.ts", Language: "typescript", DrillPath: "application/create"},
			{ID: "b", Path: "src/application/list.ts", Language: "typescript", DrillPath: "application/list"},
		},
		Deps: []modules.Dep{{FromID: "b", ToID: "a", Kind: "direct"}},
	}
	v := projectView(g, "application")
	if len(v.Nodes) != 2 {
		t.Fatalf("nodes=%d %#v", len(v.Nodes), v.Nodes)
	}
	if v.Cycles == nil {
		t.Fatal("cycles nil")
	}
}

func TestProjectViewFileRootIsPackageDir(t *testing.T) {
	g := modules.Graph{
		Modules: []modules.Module{
			{ID: "a", Path: "packages/auth/src/cookie.ts", Language: "typescript", DrillPath: "auth/cookie"},
			{ID: "b", Path: "packages/auth/src/jwt.ts", Language: "typescript", DrillPath: "auth/jwt"},
			{ID: "c", Path: "apps/schedule-api/src/http/routes/auth.ts", Language: "typescript", DrillPath: "schedule-api/http/routes/auth"},
		},
	}
	v := projectView(g, "auth")
	if v.FileRoot != "packages/auth/src" {
		t.Fatalf("view fileRoot=%q", v.FileRoot)
	}
	got := map[string]string{}
	for _, n := range v.Nodes {
		got[n.ID] = n.FileRoot
	}
	if got["cookie"] != "packages/auth/src/cookie.ts" {
		t.Fatalf("cookie fileRoot=%q", got["cookie"])
	}
	if _, ok := got["routes"]; ok {
		t.Fatal("schedule-api auth route leaked into auth package view")
	}
}

func TestProjectViewPackagesAtRoot(t *testing.T) {
	g := modules.Graph{
		Modules: []modules.Module{
			{ID: "a", Path: "apps/alpha/src/domain/a.ts", DrillPath: "alpha/domain/a"},
			{ID: "b", Path: "apps/beta/src/domain/b.ts", DrillPath: "beta/domain/b"},
			{ID: "c", Path: "packages/lib/src/index.ts", DrillPath: "lib/index"},
		},
	}
	v := projectView(g, "")
	got := map[string]bool{}
	for _, n := range v.Nodes {
		got[n.ID] = true
	}
	for _, want := range []string{"alpha", "beta", "lib"} {
		if !got[want] {
			t.Fatalf("missing %s in %#v", want, v.Nodes)
		}
	}
	if got["domain"] {
		t.Fatal("collapsed domain node should not appear at repo root")
	}
}
