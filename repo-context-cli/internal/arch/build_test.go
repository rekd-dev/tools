package arch_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"repo-context-cli/internal/arch"
	"repo-context-cli/internal/fitness"
)

func fixtureDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

func TestTsCaFixtureIsClean(t *testing.T) {
	root := fixtureDir("ts-ca")
	g := arch.BuildFromRepo(root, fitness.DefaultRules())
	if len(g.Modules) < 4 {
		t.Fatalf("expected >=4 modules, got %d", len(g.Modules))
	}
	report := fitness.Analyze("ts-ca", root, "", g.Modules, g.Deps, fitness.DefaultRules(), "default")
	for _, f := range report.Findings {
		if f.Kind == "cycle" || f.Kind == "layer_violation" || f.Kind == "domain_purity" {
			t.Fatalf("unexpected %s: %s", f.Kind, f.Message)
		}
	}
}

func TestCsharpCaFixtureIsClean(t *testing.T) {
	root := fixtureDir("csharp-ca")
	g := arch.BuildFromRepo(root, fitness.DefaultRules())
	if len(g.Modules) < 3 {
		t.Fatalf("expected csharp modules, got %d", len(g.Modules))
	}
	report := fitness.Analyze("csharp-ca", root, "", g.Modules, g.Deps, fitness.DefaultRules(), "default")
	for _, f := range report.Findings {
		if f.Kind == "cycle" || f.Kind == "layer_violation" || f.Kind == "domain_purity" {
			t.Fatalf("unexpected %s: %s", f.Kind, f.Message)
		}
	}
}

func TestTsCaBadFixtureFindsPurityAndCycle(t *testing.T) {
	root := fixtureDir("ts-ca-bad")
	g := arch.BuildFromRepo(root, fitness.DefaultRules())
	if len(g.Modules) < 4 {
		t.Fatalf("expected >=4 modules, got %d", len(g.Modules))
	}
	report := fitness.Analyze("ts-ca-bad", root, "", g.Modules, g.Deps, fitness.DefaultRules(), "default")
	var purity, cycle, layer bool
	for _, f := range report.Findings {
		switch f.Kind {
		case "domain_purity":
			purity = true
		case "cycle":
			cycle = true
		case "layer_violation":
			layer = true
		}
	}
	if !purity {
		t.Fatal("expected domain_purity finding")
	}
	if !cycle {
		t.Fatalf("expected cycle finding; findings=%v", report.Findings)
	}
	if !layer {
		t.Fatal("expected layer_violation (application→infrastructure or similar)")
	}
}

func TestCsharpCaBadFixtureFindsCycle(t *testing.T) {
	root := fixtureDir("csharp-ca-bad")
	g := arch.BuildFromRepo(root, fitness.DefaultRules())
	if len(g.Modules) < 3 {
		t.Fatalf("expected csharp modules, got %d", len(g.Modules))
	}
	report := fitness.Analyze("csharp-ca-bad", root, "", g.Modules, g.Deps, fitness.DefaultRules(), "default")
	var cycle bool
	for _, f := range report.Findings {
		if f.Kind == "cycle" {
			cycle = true
		}
	}
	if !cycle {
		t.Fatalf("expected cycle; findings=%v layers=%v", report.Findings, report.Modules)
	}
}

func TestParentRepoSkipsTestdata(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	cliRoot := filepath.Join(filepath.Dir(file), "..", "..")
	g := arch.BuildFromRepo(cliRoot, fitness.DefaultRules())
	for _, m := range g.Modules {
		if filepath.ToSlash(m.Path) != "" && containsTestdata(m.Path) {
			t.Fatalf("testdata leaked into parent scan: %s", m.Path)
		}
	}
}

func containsTestdata(p string) bool {
	s := filepath.ToSlash(p)
	return s == "testdata" || strings.HasPrefix(s, "testdata/") || strings.Contains(s, "/testdata/")
}

func TestMonoWsPackagesAreTopLevel(t *testing.T) {
	root := fixtureDir("mono-ws")
	g := arch.BuildFromRepo(root, fitness.DefaultRules())
	if len(g.Modules) < 3 {
		t.Fatalf("modules=%d", len(g.Modules))
	}
	var domainCount int
	for _, m := range g.Modules {
		if m.DrillPath == "domain/entity" {
			domainCount++
		}
		if strings.HasPrefix(m.DrillPath, "alpha/") || m.DrillPath == "alpha" {
			continue
		}
		if strings.HasPrefix(m.DrillPath, "beta/") || m.DrillPath == "beta" {
			continue
		}
		if strings.HasPrefix(m.DrillPath, "lib/") || m.DrillPath == "lib" {
			continue
		}
		t.Fatalf("unexpected drill path %q for %s", m.DrillPath, m.Path)
	}
	if domainCount > 0 {
		t.Fatal("workspace apps collapsed into a shared domain path")
	}
	var local bool
	for _, d := range g.Deps {
		if d.Kind != "external" && strings.Contains(d.FromID, "apps/alpha") && strings.Contains(d.ToID, "apps/alpha") {
			local = true
		}
	}
	if !local {
		t.Fatal("expected in-package relative import (including .js specifier) to resolve")
	}
}
