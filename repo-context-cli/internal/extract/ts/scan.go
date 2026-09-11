package ts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"repo-context-cli/internal/modules"
)

var (
	importFromRe = regexp.MustCompile(`(?m)(?:import|export)\s+(?:(type)\s+)?(?:[\s\S]*?)\s+from\s+['"]([^'"]+)['"]`)
	importSideRe = regexp.MustCompile(`(?m)^\s*import\s+['"]([^'"]+)['"]`)
	requireRe    = regexp.MustCompile(`require\(\s*['"]([^'"]+)['"]\s*\)`)
	abstractRe   = regexp.MustCompile(`(?m)export\s+(?:type\s+|interface\s+|abstract\s+class\s+)`)
)

// PathAlias maps an import prefix to a filesystem directory (repo-relative slash path).
type PathAlias struct {
	Prefix string // e.g. "@/"
	Target string // e.g. "src/"
}

// ScanResult holds modules and deps discovered under a repo.
type ScanResult struct {
	Modules []modules.Module
	Deps    []modules.Dep
}

// Scan walks TypeScript/TSX sources and builds a module graph.
func Scan(repoRoot string, aliases []PathAlias, packageRoots map[string]string, workspaceRoots []string) ScanResult {
	repoRoot = filepath.Clean(repoRoot)
	fileIndex := map[string]bool{} // slash-rel path without extension variants keyed by resolved id
	var mods []modules.Module
	pending := map[string][]importSpec{} // module id -> import specs

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
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".jsx" {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasSuffix(base, ".d.ts") || strings.Contains(base, ".test.") ||
			strings.Contains(base, ".spec.") || strings.HasSuffix(base, ".test.ts") ||
			strings.HasSuffix(base, ".test.tsx") {
			return nil
		}

		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		id := rel

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		text := string(content)

		mod := modules.Module{
			ID:        id,
			Kind:      "file",
			Path:      rel,
			Language:  "typescript",
			Abstract:  abstractRe.MatchString(text),
			DrillPath: drillPathTS(rel, workspaceRoots),
			Source:    "heuristic",
		}

		specs := extractImportSpecs(text)
		var extImps []string
		var local []importSpec
		for _, spec := range specs {
			if isRelativeOrAlias(spec.Spec, aliases) || isWorkspacePkg(spec.Spec, packageRoots) {
				local = append(local, spec)
			} else if isExternalPkg(spec.Spec) {
				pkg := packageName(spec.Spec)
				extImps = append(extImps, pkg)
			}
		}
		mod.ExtImports = unique(extImps)
		mods = append(mods, mod)
		fileIndex[id] = true
		pending[id] = local
		return nil
	})

	// Index for resolution: stem -> id
	byStem := map[string]string{}
	for id := range fileIndex {
		stem := stripExt(id)
		byStem[stem] = id
		if strings.HasSuffix(stem, "/index") {
			byStem[strings.TrimSuffix(stem, "/index")] = id
		}
	}

	var deps []modules.Dep
	seen := map[string]int{} // key -> index in deps; value import wins over type-only
	for from, specs := range pending {
		fromDir := filepath.ToSlash(filepath.Dir(from))
		for _, spec := range specs {
			target := resolveSpec(spec.Spec, fromDir, aliases, packageRoots, byStem)
			if target == "" || target == from {
				continue
			}
			key := from + "\x00" + target
			if idx, ok := seen[key]; ok {
				if deps[idx].TypeOnly && !spec.TypeOnly {
					deps[idx].TypeOnly = false
				}
				continue
			}
			seen[key] = len(deps)
			kind := "direct"
			deps = append(deps, modules.Dep{
				FromID:   from,
				ToID:     target,
				Kind:     kind,
				ViaFile:  from,
				TypeOnly: spec.TypeOnly,
			})
		}
		for _, pkg := range modsExt(mods, from) {
			key := from + "\x00ext:" + pkg
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = len(deps)
			deps = append(deps, modules.Dep{
				FromID:  from,
				ToID:    "ext:" + pkg,
				Kind:    "external",
				ViaFile: from,
			})
		}
	}

	return ScanResult{Modules: mods, Deps: deps}
}

func modsExt(mods []modules.Module, id string) []string {
	for _, m := range mods {
		if m.ID == id {
			return m.ExtImports
		}
	}
	return nil
}

type importSpec struct {
	Spec     string
	TypeOnly bool
}

func extractImportSpecs(text string) []importSpec {
	var out []importSpec
	for _, m := range importFromRe.FindAllStringSubmatch(text, -1) {
		out = append(out, importSpec{Spec: m[2], TypeOnly: m[1] == "type"})
	}
	for _, m := range importSideRe.FindAllStringSubmatch(text, -1) {
		out = append(out, importSpec{Spec: m[1]})
	}
	for _, m := range requireRe.FindAllStringSubmatch(text, -1) {
		out = append(out, importSpec{Spec: m[1]})
	}
	return out
}

func isRelativeOrAlias(spec string, aliases []PathAlias) bool {
	if strings.HasPrefix(spec, ".") {
		return true
	}
	for _, a := range aliases {
		if strings.HasPrefix(spec, a.Prefix) {
			return true
		}
	}
	return false
}

func isWorkspacePkg(spec string, packageRoots map[string]string) bool {
	name := packageName(spec)
	_, ok := packageRoots[name]
	return ok
}

func isExternalPkg(spec string) bool {
	if strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") {
		return false
	}
	return true
}

func packageName(spec string) string {
	if strings.HasPrefix(spec, "@") {
		parts := strings.SplitN(spec, "/", 3)
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
		return spec
	}
	parts := strings.SplitN(spec, "/", 2)
	return parts[0]
}

func resolveSpec(spec, fromDir string, aliases []PathAlias, packageRoots map[string]string, byStem map[string]string) string {
	var candidate string
	if strings.HasPrefix(spec, ".") {
		candidate = filepath.ToSlash(filepath.Clean(fromDir + "/" + spec))
	} else if root, ok := packageRoots[packageName(spec)]; ok {
		rest := strings.TrimPrefix(spec, packageName(spec))
		rest = strings.TrimPrefix(rest, "/")
		if rest == "" {
			candidate = strings.TrimSuffix(root, "/")
		} else {
			candidate = filepath.ToSlash(filepath.Clean(root + "/" + rest))
		}
	} else {
		for _, a := range aliases {
			if strings.HasPrefix(spec, a.Prefix) {
				rest := strings.TrimPrefix(spec, a.Prefix)
				candidate = filepath.ToSlash(filepath.Clean(a.Target + rest))
				break
			}
		}
	}
	if candidate == "" {
		return ""
	}
	candidate = strings.TrimPrefix(candidate, "./")
	candidate = stripExt(candidate)
	if id, ok := byStem[candidate]; ok {
		return id
	}
	if id, ok := byStem[candidate+"/index"]; ok {
		return id
	}
	return ""
}

func stripExt(p string) string {
	ext := filepath.Ext(p)
	return strings.TrimSuffix(p, ext)
}

func drillPathTS(rel string, workspaceRoots []string) string {
	return modules.ComputeDrillPath("typescript", "", rel, workspaceRoots)
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

// LoadTsconfigPaths reads a tsconfig.json paths map into aliases (best-effort).
func LoadTsconfigPaths(repoRoot, tsconfigRel string) []PathAlias {
	path := filepath.Join(repoRoot, tsconfigRel)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	compiler, ok := raw["compilerOptions"]
	if !ok {
		return nil
	}
	var opts struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	}
	if json.Unmarshal(compiler, &opts) != nil {
		return nil
	}
	base := opts.BaseURL
	if base == "" {
		base = "."
	}
	cfgDir := filepath.ToSlash(filepath.Dir(tsconfigRel))
	var aliases []PathAlias
	for pattern, targets := range opts.Paths {
		if len(targets) == 0 {
			continue
		}
		prefix := strings.TrimSuffix(pattern, "*")
		target := strings.TrimSuffix(targets[0], "*")
		target = filepath.ToSlash(filepath.Clean(cfgDir + "/" + base + "/" + target))
		if !strings.HasSuffix(target, "/") && prefix != "" {
			// directory-ish
		}
		if !strings.HasSuffix(target, "/") {
			// keep as-is; resolveSpec cleans
		}
		aliases = append(aliases, PathAlias{Prefix: prefix, Target: target + "/"})
	}
	return aliases
}

// DiscoverPackageRoots finds workspace package.json name -> src root (repo-relative).
func DiscoverPackageRoots(repoRoot string) map[string]string {
	out := map[string]string{}
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
		if info.Name() != "package.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var pkg struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &pkg) != nil || pkg.Name == "" {
			return nil
		}
		dir, _ := filepath.Rel(repoRoot, filepath.Dir(path))
		dir = filepath.ToSlash(dir)
		src := dir + "/src"
		if st, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(src))); err == nil && st.IsDir() {
			out[pkg.Name] = src
		} else {
			out[pkg.Name] = dir
		}
		return nil
	})
	return out
}
