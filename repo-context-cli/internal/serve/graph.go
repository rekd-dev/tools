package serve

import (
	"net/http"
	"strconv"
	"strings"

	"repo-context-cli/internal/model"
	"repo-context-cli/internal/query"
	"repo-context-cli/internal/store"
)

func registerGraphAPI(mux *http.ServeMux, hub *graphHub) {
	mux.HandleFunc("/api/coverage", func(w http.ResponseWriter, r *http.Request) {
		g, err := hub.facts()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, g.Coverage)
	})
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		g, err := hub.facts()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		q := r.URL.Query().Get("q")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		writeJSON(w, query.Search(g, q, limit))
	})
	mux.HandleFunc("/api/entity", func(w http.ResponseWriter, r *http.Request) {
		g, err := hub.facts()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		id := r.URL.Query().Get("id")
		writeJSON(w, entityDetail(g, id))
	})
	mux.HandleFunc("/api/edge", func(w http.ResponseWriter, r *http.Request) {
		g, err := hub.facts()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		id := r.URL.Query().Get("id")
		for _, e := range g.Edges {
			if e.ID == id {
				writeJSON(w, e)
				return
			}
		}
		http.Error(w, "not found", 404)
	})
	mux.HandleFunc("/api/path", func(w http.ResponseWriter, r *http.Request) {
		g, err := hub.facts()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		q := r.URL.Query()
		depth, _ := strconv.Atoi(q.Get("maxDepth"))
		maxN, _ := strconv.Atoi(q.Get("maxNodes"))
		writeJSON(w, query.BoundedPath(g, query.PathRequest{
			From: q.Get("from"), To: q.Get("to"), MaxDepth: depth, MaxNodes: maxN,
			Kinds: query.ParseKinds(q.Get("kinds")),
		}))
	})
	mux.HandleFunc("/api/graph-view", func(w http.ResponseWriter, r *http.Request) {
		g, err := hub.facts()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		q := r.URL.Query()
		writeJSON(w, projectGraphView(g, GraphViewRequest{
			Lens:     q.Get("lens"),
			Scope:    q.Get("path"),
			Sel:      q.Get("sel"),
			Overlays: strings.Split(q.Get("overlays"), ","),
		}))
	})
}

func (h *graphHub) facts() (model.Graph, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cachedFacts != nil {
		return *h.cachedFacts, nil
	}
	db, err := store.Open(h.dbPath)
	if err != nil {
		return model.Graph{}, err
	}
	defer db.Close()
	g, err := store.LoadGraph(db)
	if err != nil || len(g.Nodes) == 0 {
		return model.Graph{Nodes: []model.Node{}, Edges: []model.Edge{}, Coverage: []model.Coverage{{
			Analyzer: "heuristic", Status: "missing", Message: "graph tables empty — re-run inventory",
		}}}, nil
	}
	h.cachedFacts = &g
	return g, nil
}

type EntityDetail struct {
	Node         model.Node   `json:"node"`
	Incoming     []model.Edge `json:"incoming"`
	Outgoing     []model.Edge `json:"outgoing"`
	Implementers []model.Node `json:"implementers,omitempty"`
	Bindings     []model.Edge `json:"bindings,omitempty"`
	InjectedInto []model.Edge `json:"injectedInto,omitempty"`
}

func entityDetail(g model.Graph, id string) EntityDetail {
	byID := g.NodeByID()
	out := EntityDetail{Incoming: []model.Edge{}, Outgoing: []model.Edge{}}
	if n, ok := byID[id]; ok {
		out.Node = n
	}
	for _, e := range g.Edges {
		if e.ToID == id {
			out.Incoming = append(out.Incoming, e)
			if e.Kind == model.RelImplements {
				if n, ok := byID[e.FromID]; ok {
					out.Implementers = append(out.Implementers, n)
				}
			}
			if e.Kind == model.RelBinds {
				out.Bindings = append(out.Bindings, e)
			}
			if e.Kind == model.RelInjects {
				out.InjectedInto = append(out.InjectedInto, e)
			}
		}
		if e.FromID == id {
			out.Outgoing = append(out.Outgoing, e)
		}
	}
	return out
}

type GraphViewRequest struct {
	Lens     string
	Scope    string
	Sel      string
	Overlays []string
}

type GraphView struct {
	Lens       string        `json:"lens"`
	Path       string        `json:"path"`
	Selection  string        `json:"selection,omitempty"`
	Nodes      []ViewNode    `json:"nodes"`
	Edges      []GraphEdge   `json:"edges"`
	Cycles     []string      `json:"cycles"`
	ChildPaths []string      `json:"childPaths"`
	Coverage   []model.Coverage `json:"coverage"`
	Truncated  bool          `json:"truncated,omitempty"`
	Reason     string        `json:"reason,omitempty"`
}

type GraphEdge struct {
	ID         string  `json:"id"`
	From       string  `json:"from"`
	To         string  `json:"to"`
	Kind       string  `json:"kind"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
	Unresolved bool    `json:"unresolved,omitempty"`
	File       string  `json:"file,omitempty"`
	Line       int     `json:"line,omitempty"`
	Detail     string  `json:"detail,omitempty"`
}

func projectGraphView(g model.Graph, req GraphViewRequest) GraphView {
	lens := req.Lens
	if lens == "" {
		lens = "architecture"
	}
	switch lens {
	case "focus":
		return projectFocus(g, req)
	case "flow":
		return projectFlow(g, req)
	default:
		return projectArchitectureFacts(g, req)
	}
}

func projectArchitectureFacts(g model.Graph, req GraphViewRequest) GraphView {
	return GraphView{
		Lens: "architecture", Path: req.Scope, Selection: req.Sel,
		Nodes: []ViewNode{}, Edges: []GraphEdge{}, Cycles: []string{}, ChildPaths: []string{}, Coverage: g.Coverage,
		Reason: "use /api/view for architecture drill-down",
	}
}

func projectFocus(g model.Graph, req GraphViewRequest) GraphView {
	if req.Sel == "" {
		return GraphView{Lens: "focus", Path: req.Scope, Nodes: []ViewNode{}, Edges: []GraphEdge{}, Cycles: []string{}, ChildPaths: []string{}, Coverage: g.Coverage, Reason: "select a type or interface"}
	}
	byID := g.NodeByID()
	keep := map[string]bool{req.Sel: true}
	var edges []GraphEdge
	for _, e := range g.Edges {
		if e.FromID == req.Sel || e.ToID == req.Sel {
			keep[e.FromID] = true
			keep[e.ToID] = true
			edges = append(edges, toGraphEdge(e))
			if e.Kind == model.RelImplements || e.Kind == model.RelBinds || e.Kind == model.RelInjects {
				for _, e2 := range g.Edges {
					if e2.FromID == e.FromID || e2.ToID == e.FromID || e2.FromID == e.ToID || e2.ToID == e.ToID {
						if e2.Kind == model.RelInjects || e2.Kind == model.RelBinds || e2.Kind == model.RelImplements {
							keep[e2.FromID] = true
							keep[e2.ToID] = true
							edges = append(edges, toGraphEdge(e2))
						}
					}
				}
			}
		}
	}
	return GraphView{
		Lens: "focus", Path: req.Scope, Selection: req.Sel,
		Nodes: nodesFrom(byID, keep), Edges: uniqueGraphEdges(edges),
		Cycles: []string{}, ChildPaths: []string{}, Coverage: g.Coverage,
	}
}

func projectFlow(g model.Graph, req GraphViewRequest) GraphView {
	start := req.Sel
	if start == "" {
		for _, n := range g.Nodes {
			if n.Kind == model.KindEndpoint {
				start = n.ID
				break
			}
		}
	}
	kinds := []string{model.RelCalls, model.RelAwaits, model.RelForks, model.RelJoins, model.RelRaces, model.RelInjects, model.RelHandles, model.RelWrites, model.RelCarriesData}
	wantOverlay := map[string]bool{}
	for _, o := range req.Overlays {
		o = strings.TrimSpace(o)
		if o != "" {
			wantOverlay[o] = true
		}
	}
	if !wantOverlay["data"] {
		kinds = filterOut(kinds, model.RelCarriesData, model.RelWrites, model.RelReads)
	}
	if !wantOverlay["async"] {
		kinds = filterOut(kinds, model.RelAwaits, model.RelForks, model.RelJoins, model.RelRaces)
	}
	res := query.BoundedPath(g, query.PathRequest{From: start, MaxDepth: 8, MaxNodes: 60, Kinds: kinds})
	keep := map[string]bool{}
	for _, n := range res.Nodes {
		keep[n.ID] = true
	}
	edges := make([]GraphEdge, 0, len(res.Edges))
	for _, e := range res.Edges {
		edges = append(edges, toGraphEdge(e))
	}
	return GraphView{
		Lens: "flow", Path: req.Scope, Selection: start,
		Nodes: nodesFrom(g.NodeByID(), keep), Edges: edges,
		Cycles: []string{}, ChildPaths: []string{}, Coverage: g.Coverage,
		Truncated: res.Truncated, Reason: res.Reason,
	}
}

func toGraphEdge(e model.Edge) GraphEdge {
	return GraphEdge{
		ID: e.ID, From: e.FromID, To: e.ToID, Kind: e.Kind, Source: e.Source,
		Confidence: e.Confidence, Unresolved: e.Unresolved, File: e.File, Line: e.Line, Detail: e.Detail,
	}
}

func uniqueGraphEdges(in []GraphEdge) []GraphEdge {
	seen := map[string]bool{}
	var out []GraphEdge
	for _, e := range in {
		key := e.ID
		if key == "" {
			key = e.From + "->" + e.To + ":" + e.Kind
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	return out
}

func nodesFrom(byID map[string]model.Node, keep map[string]bool) []ViewNode {
	var out []ViewNode
	for id := range keep {
		n, ok := byID[id]
		if !ok {
			continue
		}
		out = append(out, ViewNode{
			ID: n.ID, Label: n.Name, ArchLayer: n.Layer, Leaf: n.Kind != model.KindFile && n.Kind != model.KindModule,
			Abstract: n.Abstract, Path: n.File, Kind: n.Kind, Source: extraSource(n), Line: n.Line,
		})
	}
	return out
}

func extraSource(n model.Node) string {
	if n.Extra != nil {
		if s := n.Extra["source"]; s != "" {
			return s
		}
	}
	return ""
}

func filterOut(kinds []string, drop ...string) []string {
	dropSet := map[string]bool{}
	for _, d := range drop {
		dropSet[d] = true
	}
	var out []string
	for _, k := range kinds {
		if !dropSet[k] {
			out = append(out, k)
		}
	}
	return out
}
