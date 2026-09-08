package csharp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"repo-context-cli/internal/modules"
)

var (
	nsRe         = regexp.MustCompile(`(?m)^\s*namespace\s+([\w.]+)\s*[;{]`)
	usingRe      = regexp.MustCompile(`(?m)^\s*using\s+(?:static\s+)?([\w.]+)\s*;`)
	interfaceRe  = regexp.MustCompile(`(?m)\b(?:public|internal)\s+interface\s+\w+`)
	abstractClRe = regexp.MustCompile(`(?m)\b(?:public|internal)\s+abstract\s+class\s+\w+`)
)

var skipNSPrefixes = []string{
	"System", "Microsoft", "Azure", "Newtonsoft", "Serilog", "NLog",
	"MediatR", "FluentValidation", "AutoMapper", "Swashbuckle", "Xunit", "NUnit",
}

// ScanResult holds C# modules and deps.
type ScanResult struct {
	Modules []modules.Module
	Deps    []modules.Dep
}

// Scan walks .cs files and builds a module graph via namespaces and usings.
func Scan(repoRoot string, workspaceRoots []string) ScanResult {
	repoRoot = filepath.Clean(repoRoot)
	var mods []modules.Module
	nsToIDs := map[string][]string{}
	pending := map[string][]string{} // id -> usings

	_ = filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if modules.SkipWalkDir(repoRoot, path, info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".cs" {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasSuffix(strings.ToLower(base), "assemblyinfo.cs") ||
			strings.Contains(strings.ToLower(base), ".designer.cs") {
			return nil
		}

		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		text := string(content)

		ns := ""
		if m := nsRe.FindStringSubmatch(text); len(m) > 1 {
			ns = m[1]
		}

		mod := modules.Module{
			ID:        rel,
			Kind:      "file",
			Path:      rel,
			Language:  "csharp",
			Abstract:  interfaceRe.MatchString(text) || abstractClRe.MatchString(text),
			Namespace: ns,
			DrillPath: drillPathCS(ns, rel, workspaceRoots),
			Source:    "heuristic",
		}

		var usings []string
		var ext []string
		for _, m := range usingRe.FindAllStringSubmatch(text, -1) {
			u := m[1]
			if isFrameworkNS(u) {
				ext = append(ext, rootPackage(u))
				continue
			}
			usings = append(usings, u)
		}
		mod.ExtImports = unique(ext)
		mods = append(mods, mod)
		if ns != "" {
			nsToIDs[ns] = append(nsToIDs[ns], rel)
		}
		pending[rel] = usings
		return nil
	})

	var deps []modules.Dep
	seen := map[string]bool{}
	for from, usings := range pending {
		for _, u := range usings {
			targets := resolveNamespace(u, nsToIDs)
			for _, to := range targets {
				if to == from {
					continue
				}
				key := from + "\x00" + to
				if seen[key] {
					continue
				}
				seen[key] = true
				deps = append(deps, modules.Dep{
					FromID: from, ToID: to, Kind: "direct", ViaFile: from,
				})
			}
		}
		for _, m := range mods {
			if m.ID != from {
				continue
			}
			for _, pkg := range m.ExtImports {
				key := from + "\x00ext:" + pkg
				if seen[key] {
					continue
				}
				seen[key] = true
				deps = append(deps, modules.Dep{
					FromID: from, ToID: "ext:" + pkg, Kind: "external", ViaFile: from,
				})
			}
		}
	}

	return ScanResult{Modules: mods, Deps: deps}
}

func resolveNamespace(u string, nsToIDs map[string][]string) []string {
	if ids, ok := nsToIDs[u]; ok {
		return ids
	}
	// Longest prefix match among known namespaces.
	best := ""
	for ns := range nsToIDs {
		if strings.HasPrefix(u, ns+".") || u == ns {
			if len(ns) > len(best) {
				best = ns
			}
		}
		if strings.HasPrefix(ns, u+".") {
			// using parent of a more specific ns — include children? skip for precision
		}
	}
	if best != "" {
		return nsToIDs[best]
	}
	return nil
}

func isFrameworkNS(u string) bool {
	for _, p := range skipNSPrefixes {
		if u == p || strings.HasPrefix(u, p+".") {
			return true
		}
	}
	return false
}

func rootPackage(u string) string {
	parts := strings.SplitN(u, ".", 2)
	return parts[0]
}

func drillPathCS(ns, rel string, workspaceRoots []string) string {
	return modules.ComputeDrillPath("csharp", ns, rel, workspaceRoots)
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
