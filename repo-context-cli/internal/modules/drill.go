package modules

import (
	"path/filepath"
	"strings"
)

const DrillSep = "/"

// ComputeDrillPath builds a slash-separated drill path (folder/namespace segments).
// Filenames keep internal dots (e.g. vite.config).
// workspaceRoots are repo-relative package dirs from npm/pnpm workspaces; when set,
// the first path segment is the workspace package so apps are not collapsed.
func ComputeDrillPath(language, ns, rel string, workspaceRoots []string) string {
	rel = filepath.ToSlash(rel)
	if root := LongestWorkspaceRoot(rel, workspaceRoots); root != "" {
		key := WorkspaceKey(root, workspaceRoots)
		rest := strings.TrimPrefix(rel, root)
		rest = strings.TrimPrefix(rest, "/")
		rest = stripLeadingSrc(rest)
		rest = stripFileExtKeepDots(rest)
		if rest == "" {
			return key
		}
		return key + DrillSep + rest
	}
	if language == "csharp" && ns != "" {
		return strings.ReplaceAll(ns, ".", DrillSep)
	}
	return stripSrcAnywhere(rel)
}

func stripLeadingSrc(p string) string {
	if p == "src" {
		return ""
	}
	return strings.TrimPrefix(p, "src/")
}

func stripFileExtKeepDots(p string) string {
	if p == "" {
		return ""
	}
	dir, file := pathSplit(p)
	file = strings.TrimSuffix(file, filepath.Ext(file))
	if dir == "" {
		return file
	}
	return dir + "/" + file
}

func pathSplit(p string) (dir, file string) {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return "", p
	}
	return p[:i], p[i+1:]
}

func stripSrcAnywhere(rel string) string {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		if p == "src" && i+1 < len(parts) {
			parts = parts[i+1:]
			break
		}
	}
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	last = strings.TrimSuffix(last, filepath.Ext(last))
	parts[len(parts)-1] = last
	return strings.Join(parts, DrillSep)
}

// SplitDrillPath splits a viewer path into segments.
// Slash paths keep filename dots (arch-view/vite.config).
// Dot-only paths are treated as C# namespaces (Fixture.Application).
func SplitDrillPath(p string) []string {
	p = strings.Trim(p, "/")
	p = strings.Trim(p, ".")
	if p == "" {
		return nil
	}
	sep := DrillSep
	if !strings.Contains(p, "/") {
		sep = "."
	}
	var out []string
	for _, s := range strings.Split(p, sep) {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// JoinDrillPath joins segments with /.
func JoinDrillPath(parts ...string) string {
	var out []string
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, DrillSep)
}
