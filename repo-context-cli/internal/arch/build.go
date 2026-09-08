package arch

import (
	"path/filepath"

	csharp "repo-context-cli/internal/extract/csharp"
	ts "repo-context-cli/internal/extract/ts"
	"repo-context-cli/internal/fitness"
	"repo-context-cli/internal/modules"
)

// BuildFromRepo scans TypeScript and C# sources and returns a merged graph with layers.
func BuildFromRepo(repoRoot string, rules fitness.Rules) modules.Graph {
	aliases := collectAliases(repoRoot)
	pkgRoots := ts.DiscoverPackageRoots(repoRoot)
	wsRoots := modules.DiscoverWorkspaceRoots(repoRoot)
	tsRes := ts.Scan(repoRoot, aliases, pkgRoots, wsRoots)
	csRes := csharp.Scan(repoRoot, wsRoots)

	mods := append(tsRes.Modules, csRes.Modules...)
	deps := append(tsRes.Deps, csRes.Deps...)
	mods = fitness.AssignLayers(mods, rules)

	return modules.Graph{Modules: mods, Deps: deps}
}

func collectAliases(repoRoot string) []ts.PathAlias {
	var aliases []ts.PathAlias
	for _, name := range []string{"tsconfig.json", "tsconfig.base.json", "tsconfig.app.json"} {
		aliases = append(aliases, ts.LoadTsconfigPaths(repoRoot, name)...)
	}
	entries, _ := filepath.Glob(filepath.Join(repoRoot, "apps", "*", "tsconfig.json"))
	for _, e := range entries {
		rel, err := filepath.Rel(repoRoot, e)
		if err != nil {
			continue
		}
		aliases = append(aliases, ts.LoadTsconfigPaths(repoRoot, filepath.ToSlash(rel))...)
	}
	entries, _ = filepath.Glob(filepath.Join(repoRoot, "packages", "*", "tsconfig.json"))
	for _, e := range entries {
		rel, err := filepath.Rel(repoRoot, e)
		if err != nil {
			continue
		}
		aliases = append(aliases, ts.LoadTsconfigPaths(repoRoot, filepath.ToSlash(rel))...)
	}
	return aliases
}
