package query

import (
	"strings"

	"repo-context-cli/internal/model"
)

type PathRequest struct {
	From           string
	To             string
	MaxDepth       int
	Kinds          []string
	MaxNodes       int
	MinConfidence  float64
	SkipUnresolved bool
}

type PathResult struct {
	Nodes     []model.Node `json:"nodes"`
	Edges     []model.Edge `json:"edges"`
	Truncated bool         `json:"truncated,omitempty"`
	Reason    string       `json:"reason,omitempty"`
}

func BoundedPath(g model.Graph, req PathRequest) PathResult {
	if req.MaxDepth <= 0 {
		req.MaxDepth = 6
	}
	if req.MaxDepth > 12 {
		req.MaxDepth = 12
	}
	if req.MaxNodes <= 0 {
		req.MaxNodes = 80
	}
	kindOK := map[string]bool{}
	for _, k := range req.Kinds {
		kindOK[k] = true
	}
	allow := func(k string) bool {
		if len(kindOK) == 0 {
			return k != model.RelContains
		}
		return kindOK[k]
	}

	byID := g.NodeByID()
	out := map[string][]model.Edge{}
	in := map[string][]model.Edge{}
	for _, e := range g.Edges {
		if !allow(e.Kind) {
			continue
		}
		out[e.FromID] = append(out[e.FromID], e)
		in[e.ToID] = append(in[e.ToID], e)
	}

	if req.From == "" {
		return PathResult{Nodes: []model.Node{}, Edges: []model.Edge{}, Reason: "missing start"}
	}
	if _, ok := byID[req.From]; !ok {
		return PathResult{Nodes: []model.Node{}, Edges: []model.Edge{}, Reason: "unknown start"}
	}

	seen := map[string]bool{req.From: true}
	type step struct {
		id    string
		depth int
	}
	queue := []step{{req.From, 0}}
	edges := []model.Edge{}
	truncated := false
	walk := func(e model.Edge) bool {
		if req.SkipUnresolved && e.Unresolved {
			return false
		}
		if req.MinConfidence > 0 && e.Confidence > 0 && e.Confidence < req.MinConfidence {
			return false
		}
		return true
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= req.MaxDepth {
			continue
		}
		for _, e := range out[cur.id] {
			if !walk(e) {
				continue
			}
			if req.To != "" && e.ToID == req.To {
				edges = append(edges, e)
				seen[e.ToID] = true
				continue
			}
			if seen[e.ToID] {
				continue
			}
			if len(seen) >= req.MaxNodes {
				truncated = true
				continue
			}
			seen[e.ToID] = true
			edges = append(edges, e)
			queue = append(queue, step{e.ToID, cur.depth + 1})
		}
	}

	if req.To != "" {
		edges = reachableTo(req.From, req.To, edges)
		seen = map[string]bool{}
		for _, e := range edges {
			seen[e.FromID] = true
			seen[e.ToID] = true
		}
		seen[req.From] = true
	}

	nodes := make([]model.Node, 0, len(seen))
	for id := range seen {
		if n, ok := byID[id]; ok {
			nodes = append(nodes, n)
		}
	}
	reason := ""
	if truncated {
		reason = "node limit reached; showing a focused projection"
	}
	return PathResult{Nodes: nodes, Edges: edges, Truncated: truncated, Reason: reason}
}

func reachableTo(from, to string, edges []model.Edge) []model.Edge {
	out := map[string][]model.Edge{}
	for _, e := range edges {
		out[e.FromID] = append(out[e.FromID], e)
	}
	var keep []model.Edge
	var dfs func(string, []model.Edge, map[string]bool)
	dfs = func(id string, path []model.Edge, visited map[string]bool) {
		if id == to {
			keep = append(keep, path...)
			return
		}
		for _, e := range out[id] {
			if visited[e.ToID] {
				continue
			}
			visited[e.ToID] = true
			dfs(e.ToID, append(path, e), visited)
			delete(visited, e.ToID)
		}
	}
	dfs(from, nil, map[string]bool{from: true})
	return uniqueEdges(keep)
}

func uniqueEdges(in []model.Edge) []model.Edge {
	seen := map[string]bool{}
	out := []model.Edge{}
	for _, e := range in {
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		out = append(out, e)
	}
	return out
}

func ParseKinds(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
