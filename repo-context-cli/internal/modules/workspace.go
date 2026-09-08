package modules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverWorkspaceRoots returns repo-relative slash paths of npm/pnpm workspace packages.
func DiscoverWorkspaceRoots(repoRoot string) []string {
	repoRoot = filepath.Clean(repoRoot)
	patterns := workspaceGlobs(repoRoot)
	if len(patterns) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var roots []string
	for _, pat := range patterns {
		matches, _ := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(pat)))
		for _, m := range matches {
			st, err := os.Stat(m)
			if err != nil || !st.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(m, "package.json")); err != nil {
				continue
			}
			rel, err := filepath.Rel(repoRoot, m)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if rel == "." || seen[rel] {
				continue
			}
			seen[rel] = true
			roots = append(roots, rel)
		}
	}
	return roots
}

func workspaceGlobs(repoRoot string) []string {
	var globs []string
	globs = append(globs, npmWorkspaceGlobs(filepath.Join(repoRoot, "package.json"))...)
	globs = append(globs, pnpmWorkspaceGlobs(filepath.Join(repoRoot, "pnpm-workspace.yaml"))...)
	return uniqueStrings(globs)
}

func npmWorkspaceGlobs(pkgPath string) []string {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil
	}
	var raw struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}
	if json.Unmarshal(data, &raw) != nil || len(raw.Workspaces) == 0 {
		return nil
	}
	var list []string
	if json.Unmarshal(raw.Workspaces, &list) == nil {
		return list
	}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if json.Unmarshal(raw.Workspaces, &obj) == nil {
		return obj.Packages
	}
	return nil
}

func pnpmWorkspaceGlobs(yamlPath string) []string {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil
	}
	var globs []string
	inPackages := false
	for _, line := range strings.Split(string(data), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasPrefix(trim, "packages:") {
			inPackages = true
			continue
		}
		if inPackages && strings.HasPrefix(trim, "-") {
			g := strings.TrimSpace(strings.TrimPrefix(trim, "-"))
			g = strings.Trim(g, `"'`)
			if g != "" {
				globs = append(globs, g)
			}
			continue
		}
		if inPackages && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
	}
	return globs
}

func uniqueStrings(in []string) []string {
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

// WorkspaceKey is the first drill-path segment for a workspace root.
func WorkspaceKey(root string, all []string) string {
	root = filepath.ToSlash(root)
	base := root
	if i := strings.LastIndex(root, "/"); i >= 0 {
		base = root[i+1:]
	}
	n := 0
	for _, r := range all {
		r = filepath.ToSlash(r)
		b := r
		if i := strings.LastIndex(r, "/"); i >= 0 {
			b = r[i+1:]
		}
		if b == base {
			n++
		}
	}
	if n > 1 {
		return root
	}
	return base
}

// LongestWorkspaceRoot is the longest workspace prefix of rel, or "".
func LongestWorkspaceRoot(rel string, roots []string) string {
	rel = filepath.ToSlash(rel)
	best := ""
	for _, r := range roots {
		r = filepath.ToSlash(r)
		if rel == r || strings.HasPrefix(rel, r+"/") {
			if len(r) > len(best) {
				best = r
			}
		}
	}
	return best
}
