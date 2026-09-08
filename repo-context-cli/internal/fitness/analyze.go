package fitness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"repo-context-cli/internal/graph"
	"repo-context-cli/internal/modules"
)

// Severity levels.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// Finding is one architecture fitness issue.
type Finding struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"` // cycle | layer_violation | domain_purity | unlayered
	Severity string `json:"severity"`
	Message  string `json:"message"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Layer    string `json:"layer,omitempty"`
	Cycle    string `json:"cycle,omitempty"`
}

// Report is the fitness JSON contract (viewer + CI).
type Report struct {
	Repo     string        `json:"repo"`
	Path     string        `json:"path"`
	GitSha   string        `json:"gitSha,omitempty"`
	Rules    string        `json:"rulesSource"` // default | path
	Summary  Summary       `json:"summary"`
	Findings []Finding     `json:"findings"`
	Modules  []ModuleBrief `json:"modules,omitempty"`
	Cycles   []string      `json:"cycles,omitempty"`
}

// Summary counts by severity/kind.
type Summary struct {
	Modules   int `json:"modules"`
	Deps      int `json:"deps"`
	Errors    int `json:"errors"`
	Warnings  int `json:"warnings"`
	Cycles    int `json:"cycles"`
	LayerHits int `json:"layerViolations"`
	Purity    int `json:"domainPurity"`
	Unlayered int `json:"unlayered"`
}

// ModuleBrief is a compact module row for the report.
type ModuleBrief struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Language string `json:"language"`
	Layer    string `json:"layer,omitempty"`
	Abstract bool   `json:"abstract"`
}

// Rules is optional arch-rules.json guidance.
type Rules struct {
	Layers            map[string][]string `json:"layers"`            // layer -> globs
	Allowed           map[string][]string `json:"allowed"`           // from layer -> to layers
	ForbiddenPkgs     map[string][]string `json:"forbiddenPackages"` // layer -> packages
	Ignore            []string            `json:"ignore"`
	UnlayeredSeverity string              `json:"unlayeredSeverity"` // warning (default) | info | error | off
}

// DefaultRules returns Clean Architecture conventions for TS/.NET.
func DefaultRules() Rules {
	return Rules{
		Layers: map[string][]string{
			"domain":         {"**/domain/**", "**/*.Domain/**", "**/Domain/**"},
			"application":    {"**/application/**", "**/use-cases/**", "**/usecases/**", "**/ports/**", "**/*.Application/**", "**/Application/**"},
			"infrastructure": {"**/infrastructure/**", "**/*.Infrastructure/**", "**/Infrastructure/**"},
			"interface":      {"**/interface/**", "**/http/**", "**/Controllers/**", "**/*.Api/**", "**/*.Web/**", "**/Api/**"},
			"composition":    {"**/composition/**", "**/Program.cs", "**/Startup.cs", "**/*Composition*"},
			"web":            {"**/*-web/**", "**/arch-view/**"},
		},
		Allowed: map[string][]string{
			"composition":    {"domain", "application", "infrastructure", "interface", "web", "composition"},
			"interface":      {"application", "domain", "interface"},
			"web":            {"application", "domain", "web"},
			"infrastructure": {"application", "domain", "infrastructure"},
			"application":    {"domain", "application"},
			"domain":         {"domain"},
		},
		ForbiddenPkgs: map[string][]string{
			"domain": {
				"fastify", "express", "koa", "hapi", "better-sqlite3", "sqlite3", "pg", "mysql", "mongodb",
				"react", "react-dom", "vue", "angular", "@angular/core",
				"Microsoft.EntityFrameworkCore", "Microsoft.AspNetCore", "Microsoft.Data.SqlClient",
				"System.Data", "Dapper", "Npgsql",
			},
			"application": {
				"better-sqlite3", "sqlite3", "react", "react-dom",
				"Microsoft.EntityFrameworkCore", "Microsoft.AspNetCore",
			},
		},
		UnlayeredSeverity: SeverityWarning,
	}
}

// LoadRules merges optional JSON over defaults.
func LoadRules(path string) (Rules, string, error) {
	base := DefaultRules()
	if path == "" {
		return base, "default", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return base, "default", err
	}
	var overlay Rules
	if err := json.Unmarshal(data, &overlay); err != nil {
		return base, path, err
	}
	if overlay.Layers != nil {
		base.Layers = overlay.Layers
	}
	if overlay.Allowed != nil {
		base.Allowed = overlay.Allowed
	}
	if overlay.ForbiddenPkgs != nil {
		base.ForbiddenPkgs = overlay.ForbiddenPkgs
	}
	if overlay.Ignore != nil {
		base.Ignore = overlay.Ignore
	}
	if overlay.UnlayeredSeverity != "" {
		base.UnlayeredSeverity = overlay.UnlayeredSeverity
	}
	return base, path, nil
}

// AssignLayers sets Module.Layer from rules (path globs). More specific path length wins;
// first matching layer name in priority order breaks ties.
func AssignLayers(mods []modules.Module, rules Rules) []modules.Module {
	priority := []string{"domain", "application", "infrastructure", "interface", "composition", "web"}
	out := make([]modules.Module, len(mods))
	copy(out, mods)
	for i := range out {
		if ignored(out[i].Path, rules.Ignore) {
			continue
		}
		out[i].Layer = matchLayer(out[i].Path, out[i].Namespace, rules, priority)
	}
	return out
}

func matchLayer(path, ns string, rules Rules, priority []string) string {
	p := filepath.ToSlash(path)
	bestLayer := ""
	bestScore := -1
	for _, layer := range priority {
		globs, ok := rules.Layers[layer]
		if !ok {
			continue
		}
		for _, g := range globs {
			if matchGlob(g, p) || (ns != "" && matchNS(g, ns)) {
				score := len(g)
				if score > bestScore {
					bestScore = score
					bestLayer = layer
				}
			}
		}
	}
	// Also check layers not in priority list
	for layer, globs := range rules.Layers {
		already := false
		for _, pr := range priority {
			if pr == layer {
				already = true
				break
			}
		}
		if already {
			continue
		}
		for _, g := range globs {
			if matchGlob(g, p) {
				score := len(g)
				if score > bestScore {
					bestScore = score
					bestLayer = layer
				}
			}
		}
	}
	return bestLayer
}

func matchNS(glob, ns string) bool {
	g := strings.TrimPrefix(filepath.ToSlash(glob), "**/")
	g = strings.TrimSuffix(g, "/**")
	g = strings.ReplaceAll(g, "*", "")
	if strings.Contains(g, ".Domain") && strings.Contains(ns, ".Domain") {
		return true
	}
	if strings.HasSuffix(ns, ".Domain") && strings.Contains(strings.ToLower(glob), "domain") {
		return true
	}
	if strings.HasSuffix(ns, ".Application") && strings.Contains(strings.ToLower(glob), "application") {
		return true
	}
	if strings.HasSuffix(ns, ".Infrastructure") && strings.Contains(strings.ToLower(glob), "infrastructure") {
		return true
	}
	return false
}

func matchGlob(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)
	// Special-case Program.cs / Startup.cs basename
	if strings.HasSuffix(pattern, "Program.cs") || strings.HasSuffix(pattern, "Startup.cs") {
		base := filepath.Base(path)
		return strings.EqualFold(base, filepath.Base(pattern))
	}
	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	// **/foo/** style
	if strings.HasPrefix(pattern, "**/") {
		rest := strings.TrimPrefix(pattern, "**/")
		if strings.HasSuffix(rest, "/**") {
			mid := strings.TrimSuffix(rest, "/**")
			return strings.Contains("/"+path+"/", "/"+mid+"/")
		}
		if strings.HasSuffix(rest, "/**") {
			return false
		}
		// **/foo.tsx
		if strings.Contains(rest, "*") {
			for _, part := range strings.Split(path, "/") {
				if ok, _ := filepath.Match(rest, part); ok {
					return true
				}
			}
			if ok, _ := filepath.Match(rest, filepath.Base(path)); ok {
				return true
			}
		} else {
			return strings.Contains("/"+path+"/", "/"+rest) || strings.HasSuffix(path, rest)
		}
	}
	if strings.HasPrefix(pattern, "**/*") {
		suf := strings.TrimPrefix(pattern, "**/*")
		if strings.HasPrefix(suf, "-web/") || strings.Contains(pattern, "*-web/**") {
			return strings.Contains(path, "-web/")
		}
		if strings.HasSuffix(pattern, ".tsx") {
			return strings.HasSuffix(path, ".tsx")
		}
	}
	if strings.Contains(pattern, "*-web/**") {
		return strings.Contains(path, "-web/")
	}
	return false
}

func ignored(path string, globs []string) bool {
	for _, g := range globs {
		if matchGlob(g, path) {
			return true
		}
	}
	return false
}

// Analyze runs all fitness checks.
func Analyze(repo, path, gitSha string, mods []modules.Module, deps []modules.Dep, rules Rules, rulesSource string) Report {
	mods = AssignLayers(mods, rules)
	byID := map[string]modules.Module{}
	for _, m := range mods {
		byID[m.ID] = m
	}

	findings := make([]Finding, 0)
	idSeq := 0
	nextID := func(prefix string) string {
		idSeq++
		return prefix + "-" + itoa(idSeq)
	}

	// Cycles (in-repo edges only)
	var nodes []string
	var edges []graph.Edge
	for _, m := range mods {
		nodes = append(nodes, m.ID)
	}
	for _, d := range deps {
		if d.Kind == "external" || strings.HasPrefix(d.ToID, "ext:") {
			continue
		}
		if _, ok := byID[d.FromID]; !ok {
			continue
		}
		if _, ok := byID[d.ToID]; !ok {
			continue
		}
		edges = append(edges, graph.Edge{From: d.FromID, To: d.ToID})
	}
	gr := graph.AssignLayers(nodes, edges)
	cycleStrs := make([]string, 0)
	for _, c := range gr.Cycles {
		cs := graph.FormatCycle(c)
		cycleStrs = append(cycleStrs, cs)
		findings = append(findings, Finding{
			ID:       nextID("cycle"),
			Kind:     "cycle",
			Severity: SeverityError,
			Message:  "module dependency cycle: " + cs,
			Cycle:    cs,
		})
	}

	// Layer violations
	for _, d := range deps {
		if d.Kind == "external" || strings.HasPrefix(d.ToID, "ext:") {
			continue
		}
		from, okF := byID[d.FromID]
		to, okT := byID[d.ToID]
		if !okF || !okT {
			continue
		}
		if from.Layer == "" || to.Layer == "" {
			continue
		}
		if !allowedEdge(from.Layer, to.Layer, rules) {
			findings = append(findings, Finding{
				ID:       nextID("layer"),
				Kind:     "layer_violation",
				Severity: SeverityError,
				Message:  from.Layer + " must not depend on " + to.Layer + ": " + from.Path + " → " + to.Path,
				From:     from.ID,
				To:       to.ID,
				Layer:    from.Layer,
			})
		}
	}

	// Domain / application purity (external packages)
	for _, m := range mods {
		forbidden := rules.ForbiddenPkgs[m.Layer]
		if len(forbidden) == 0 {
			continue
		}
		sev := SeverityError
		if m.Layer == "application" {
			sev = SeverityWarning
		}
		for _, pkg := range m.ExtImports {
			if packageForbidden(pkg, forbidden) {
				findings = append(findings, Finding{
					ID:       nextID("purity"),
					Kind:     "domain_purity",
					Severity: sev,
					Message:  m.Layer + " imports forbidden package " + pkg + ": " + m.Path,
					From:     m.ID,
					To:       "ext:" + pkg,
					Layer:    m.Layer,
				})
			}
		}
	}

	// Unlayered
	unlayeredSev := rules.UnlayeredSeverity
	if unlayeredSev == "" {
		unlayeredSev = SeverityWarning
	}
	if unlayeredSev != "off" {
		for _, m := range mods {
			if m.Layer != "" || ignored(m.Path, rules.Ignore) {
				continue
			}
			findings = append(findings, Finding{
				ID:       nextID("unlayered"),
				Kind:     "unlayered",
				Severity: unlayeredSev,
				Message:  "module matched no architecture layer: " + m.Path,
				From:     m.ID,
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return sevRank(findings[i].Severity) < sevRank(findings[j].Severity)
		}
		return findings[i].Kind < findings[j].Kind
	})

	sum := Summary{Modules: len(mods), Deps: len(deps), Cycles: len(cycleStrs)}
	briefs := make([]ModuleBrief, 0, len(mods))
	for _, m := range mods {
		briefs = append(briefs, ModuleBrief{
			ID: m.ID, Path: m.Path, Language: m.Language, Layer: m.Layer, Abstract: m.Abstract,
		})
		if m.Layer == "" {
			sum.Unlayered++
		}
	}
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			sum.Errors++
		case SeverityWarning:
			sum.Warnings++
		}
		switch f.Kind {
		case "layer_violation":
			sum.LayerHits++
		case "domain_purity":
			sum.Purity++
		}
	}

	return Report{
		Repo:     repo,
		Path:     path,
		GitSha:   gitSha,
		Rules:    rulesSource,
		Summary:  sum,
		Findings: findings,
		Modules:  briefs,
		Cycles:   cycleStrs,
	}
}

func allowedEdge(from, to string, rules Rules) bool {
	allowed, ok := rules.Allowed[from]
	if !ok {
		return true // unknown layer: don't fail
	}
	for _, a := range allowed {
		if a == to {
			return true
		}
	}
	return false
}

func packageForbidden(pkg string, forbidden []string) bool {
	pkgL := strings.ToLower(pkg)
	for _, f := range forbidden {
		fL := strings.ToLower(f)
		if pkgL == fL || strings.HasPrefix(pkgL, fL+"/") || strings.HasPrefix(pkg, f+".") {
			return true
		}
		// C# style Microsoft.AspNetCore.*
		if strings.HasPrefix(pkg, f) {
			return true
		}
	}
	return false
}

func sevRank(s string) int {
	switch s {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	default:
		return 2
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// FailOn returns true if report has findings at or above threshold.
func FailOn(report Report, threshold string) bool {
	rank := sevRank(threshold)
	for _, f := range report.Findings {
		if sevRank(f.Severity) <= rank {
			return true
		}
	}
	return false
}

// FindRulesFile looks for arch-rules.json in repo root.
func FindRulesFile(repoPath string) string {
	p := filepath.Join(repoPath, "arch-rules.json")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}
