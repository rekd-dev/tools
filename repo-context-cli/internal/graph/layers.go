package graph

import (
	"sort"
)

// Edge is a directed edge from -> to.
type Edge struct {
	From string
	To   string
}

// Cycle is an ordered list of node IDs forming a cycle (last equals first conceptually via wrap).
type Cycle []string

// Result holds topo layers and detected cycles.
type Result struct {
	// Layers maps node -> layer index (0 = most depended-upon / bottom).
	// After ranking for architecture view we flip so high-level (dependents) are at top.
	Layers map[string]int
	// MaxLayer is the highest layer index.
	MaxLayer int
	// Cycles lists simple cycles found (as node id paths without repeating start).
	Cycles []Cycle
	// CycleEdges are edges that participate in at least one cycle.
	CycleEdges map[Edge]bool
}

// AssignLayers detects cycles, temporarily removes cycle edges, then topo-ranks.
// Layer 0 = sinks (no outgoing deps after cycle break). Higher layers depend on lower ones.
// This matches Uncle Bob: high-level (many dependents) ends up on higher indices;
// callers that want top-to-bottom UI put MaxLayer at the top of the screen.
func AssignLayers(nodes []string, edges []Edge) Result {
	nodeSet := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		nodeSet[n] = true
	}

	// Keep only edges between known nodes; drop self-loops for ranking.
	var filtered []Edge
	outgoing := make(map[string][]string)
	incoming := make(map[string][]string)
	for _, e := range edges {
		if !nodeSet[e.From] || !nodeSet[e.To] || e.From == e.To {
			continue
		}
		filtered = append(filtered, e)
		outgoing[e.From] = append(outgoing[e.From], e.To)
		incoming[e.To] = append(incoming[e.To], e.From)
	}

	cycles := findCycles(nodeSet, outgoing)
	cycleEdges := make(map[Edge]bool)
	for _, c := range cycles {
		for i := 0; i < len(c); i++ {
			from := c[i]
			to := c[(i+1)%len(c)]
			cycleEdges[Edge{From: from, To: to}] = true
		}
	}

	// Acyclic subgraph for ranking.
	outDegree := make(map[string]int, len(nodeSet))
	rev := make(map[string][]string)
	for n := range nodeSet {
		outDegree[n] = 0
	}
	for _, e := range filtered {
		if cycleEdges[e] {
			continue
		}
		outDegree[e.From]++
		rev[e.To] = append(rev[e.To], e.From)
	}

	// Kahn from sinks (out-degree 0) upward — dependents get higher layers.
	layers := make(map[string]int, len(nodeSet))
	var queue []string
	for n := range nodeSet {
		if outDegree[n] == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue)

	maxLayer := 0
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, pred := range rev[n] {
			if layers[pred] < layers[n]+1 {
				layers[pred] = layers[n] + 1
				if layers[pred] > maxLayer {
					maxLayer = layers[pred]
				}
			}
			outDegree[pred]--
			if outDegree[pred] == 0 {
				queue = append(queue, pred)
			}
		}
		sort.Strings(queue)
	}

	// Nodes still stuck in unresolved cycles get layer 0 if unset.
	for n := range nodeSet {
		if _, ok := layers[n]; !ok {
			layers[n] = 0
		}
	}

	return Result{
		Layers:     layers,
		MaxLayer:   maxLayer,
		Cycles:     cycles,
		CycleEdges: cycleEdges,
	}
}

func findCycles(nodes map[string]bool, outgoing map[string][]string) []Cycle {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(nodes))
	var stack []string
	var cycles []Cycle
	seen := make(map[string]bool) // canonicalize cycle key

	var dfs func(string)
	dfs = func(u string) {
		color[u] = gray
		stack = append(stack, u)
		for _, v := range outgoing[u] {
			if !nodes[v] {
				continue
			}
			switch color[v] {
			case white:
				dfs(v)
			case gray:
				// back edge -> cycle
				idx := -1
				for i, s := range stack {
					if s == v {
						idx = i
						break
					}
				}
				if idx >= 0 {
					c := append(Cycle{}, stack[idx:]...)
					key := cycleKey(c)
					if !seen[key] {
						seen[key] = true
						cycles = append(cycles, c)
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[u] = black
	}

	var order []string
	for n := range nodes {
		order = append(order, n)
	}
	sort.Strings(order)
	for _, n := range order {
		if color[n] == white {
			dfs(n)
		}
	}
	return cycles
}

func cycleKey(c Cycle) string {
	if len(c) == 0 {
		return ""
	}
	// Rotate so lexicographically smallest node is first.
	minI := 0
	for i := 1; i < len(c); i++ {
		if c[i] < c[minI] {
			minI = i
		}
	}
	rotated := append(append([]string{}, c[minI:]...), c[:minI]...)
	out := ""
	for i, s := range rotated {
		if i > 0 {
			out += "->"
		}
		out += s
	}
	return out
}

// FormatCycle renders a->b->c->a style.
func FormatCycle(c Cycle) string {
	if len(c) == 0 {
		return ""
	}
	s := ""
	for i, n := range c {
		if i > 0 {
			s += "->"
		}
		s += n
	}
	return s + "->" + c[0]
}
