package analyzers

import (
	"fmt"
	"sort"
	"strings"

	"repo-context-cli/internal/model"
)

// Merge combines facts from multiple analyzers. All evidence is kept; the
// displayed edge uses the highest-confidence observation.
func Merge(parts ...model.Graph) model.Graph {
	nodes := map[string]model.Node{}
	grouped := map[string]*model.Edge{}
	var coverage []model.Coverage

	for _, g := range parts {
		for _, n := range g.Nodes {
			n = normalizeNode(n)
			if existing, ok := nodes[n.ID]; ok {
				nodes[n.ID] = preferNode(existing, n)
			} else {
				nodes[n.ID] = n
			}
		}
		for _, e := range g.Edges {
			key := model.EdgeGroupKey(e.FromID, e.ToID, e.Kind)
			ev := e.Evidence
			if len(ev) == 0 {
				ev = []model.Evidence{{
					Source: e.Source, Confidence: e.Confidence, Analyzer: e.Analyzer,
					File: e.File, Line: e.Line, Column: e.Column, Detail: e.Detail, Snippet: "",
				}}
			}
			if cur, ok := grouped[key]; ok {
				cur.Evidence = append(cur.Evidence, ev...)
				if e.Confidence > cur.Confidence {
					copyEdgeMeta(cur, e)
				}
				if e.Unresolved {
					cur.Unresolved = true
				}
			} else {
				cp := e
				cp.Evidence = append([]model.Evidence{}, ev...)
				grouped[key] = &cp
			}
		}
		coverage = append(coverage, g.Coverage...)
	}

	out := model.Graph{Coverage: coverage}
	for _, n := range nodes {
		out.Nodes = append(out.Nodes, n)
	}
	for _, e := range grouped {
		analyzers := map[string]bool{}
		for _, ev := range e.Evidence {
			analyzers[ev.Analyzer] = true
		}
		var names []string
		for n := range analyzers {
			names = append(names, n)
		}
		sort.Strings(names)
		e.Analyzer = strings.Join(names, ",")
		if e.ID == "" {
			e.ID = fmt.Sprintf("%s|%s|%s", e.FromID, e.ToID, e.Kind)
		}
		out.Edges = append(out.Edges, *e)
	}
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].ID < out.Nodes[j].ID })
	sort.Slice(out.Edges, func(i, j int) bool { return out.Edges[i].ID < out.Edges[j].ID })
	return out
}

func preferNode(a, b model.Node) model.Node {
	a = normalizeNode(a)
	b = normalizeNode(b)
	if a.Kind == model.KindMethod && b.Kind == model.KindMethod && a.ID == b.ID {
		if methodNodeScore(b) > methodNodeScore(a) {
			a, b = b, a
		}
	}
	if a.File == "" {
		a.File = b.File
	}
	if a.Line == 0 && b.Line > 0 {
		a.Line, a.Column = b.Line, b.Column
	}
	if a.Layer == "" {
		a.Layer = b.Layer
	}
	if !a.Abstract {
		a.Abstract = b.Abstract
	}
	if a.QualifiedName == "" {
		a.QualifiedName = b.QualifiedName
	}
	if a.Extra == nil && b.Extra != nil {
		a.Extra = b.Extra
	}
	if a.Name == "" {
		a.Name = b.Name
	}
	return normalizeNode(a)
}

const maxMethodNameLen = 80

func normalizeNode(n model.Node) model.Node {
	if n.Kind != model.KindMethod {
		return n
	}
	_, tail := methodIDFileAndTail(n.ID)
	n.Name = cleanMethodName(n.Name, tail)
	return n
}

func cleanMethodName(name, tail string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\r", ""))
	if name == "" || strings.ContainsAny(name, "\n(") || strings.Contains(name, "this.") || runeLen(name) > maxMethodNameLen {
		name = tail
	}
	name = trimRunes(name, maxMethodNameLen)
	if name == "" {
		name = trimRunes(tail, maxMethodNameLen)
	}
	return name
}

func methodNodeScore(n model.Node) int {
	fileSeg, tail := methodIDFileAndTail(n.ID)
	score := 0
	if n.File != "" && model.Slash(n.File) == fileSeg {
		score += 4
	}
	if n.Line > 0 {
		score += 2
	}
	if isDeclarationMethodName(n.Name, tail) {
		score += 2
	}
	if n.Name == tail {
		score++
	}
	return score
}

func isDeclarationMethodName(name, tail string) bool {
	if name == "" || strings.ContainsAny(name, "\n(") {
		return false
	}
	if name == tail {
		return true
	}
	return runeLen(name) <= runeLen(tail)
}

func methodIDFileAndTail(id string) (file, tail string) {
	rest, ok := strings.CutPrefix(id, "method:")
	if !ok {
		return "", ""
	}
	_, rest, ok = strings.Cut(rest, ":")
	if !ok {
		return "", ""
	}
	i := strings.LastIndex(rest, ":")
	if i < 0 {
		return rest, ""
	}
	return rest[:i], rest[i+1:]
}

func runeLen(s string) int {
	return len([]rune(s))
}

func trimRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func copyEdgeMeta(dst *model.Edge, src model.Edge) {
	dst.Source = src.Source
	dst.Confidence = src.Confidence
	dst.File = src.File
	dst.Line = src.Line
	dst.Column = src.Column
	dst.Detail = src.Detail
	dst.Unresolved = src.Unresolved
	if src.ID != "" {
		dst.ID = src.ID
	}
}
