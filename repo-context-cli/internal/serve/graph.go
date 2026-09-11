package serve

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"repo-context-cli/internal/churn"
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
			Root:     q.Get("root"),
			Overlays: strings.Split(q.Get("overlays"), ","),
		}))
	})
	mux.HandleFunc("/api/churn", func(w http.ResponseWriter, r *http.Request) {
		g, err := hub.graph()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		view := projectView(g, r.URL.Query().Get("path"))
		roots := make([]churn.Root, 0, len(view.Nodes))
		for _, n := range view.Nodes {
			if n.FileRoot != "" {
				roots = append(roots, churn.Root{ID: n.ID, FileRoot: n.FileRoot})
			}
		}
		writeJSON(w, hub.churnReport(roots))
	})
}

func (h *graphHub) facts() (model.Graph, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropStaleCache()
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
	h.rememberStamp()
	return g, nil
}

type EntityDetail struct {
	Node         model.Node   `json:"node"`
	Incoming     []GraphEdge  `json:"incoming"`
	Outgoing     []GraphEdge  `json:"outgoing"`
	Implementers []model.Node `json:"implementers,omitempty"`
	Bindings     []GraphEdge  `json:"bindings,omitempty"`
	InjectedInto []GraphEdge  `json:"injectedInto,omitempty"`
}

func entityDetail(g model.Graph, id string) EntityDetail {
	byID := g.NodeByID()
	out := EntityDetail{Incoming: []GraphEdge{}, Outgoing: []GraphEdge{}}
	if n, ok := byID[id]; ok {
		out.Node = n
	}
	for _, e := range g.Edges {
		if e.ToID == id {
			ge := toGraphEdge(e)
			out.Incoming = append(out.Incoming, ge)
			if e.Kind == model.RelImplements {
				if n, ok := byID[e.FromID]; ok {
					out.Implementers = append(out.Implementers, n)
				}
			}
			if e.Kind == model.RelBinds {
				out.Bindings = append(out.Bindings, ge)
			}
			if e.Kind == model.RelInjects {
				out.InjectedInto = append(out.InjectedInto, ge)
			}
		}
		if e.FromID == id {
			out.Outgoing = append(out.Outgoing, toGraphEdge(e))
		}
	}
	return out
}

type GraphViewRequest struct {
	Lens     string
	Scope    string
	Sel      string
	Root     string
	Overlays []string
}

type GraphView struct {
	Lens       string           `json:"lens"`
	Path       string           `json:"path"`
	Selection  string           `json:"selection,omitempty"`
	Nodes      []ViewNode       `json:"nodes"`
	Edges      []GraphEdge      `json:"edges"`
	Cycles     []string         `json:"cycles"`
	ChildPaths []string         `json:"childPaths"`
	Coverage   []model.Coverage `json:"coverage"`
	Truncated  bool             `json:"truncated,omitempty"`
	Reason     string           `json:"reason,omitempty"`
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
	empty := GraphView{Lens: "focus", Path: req.Scope, Nodes: []ViewNode{}, Edges: []GraphEdge{}, Cycles: []string{}, ChildPaths: []string{}, Coverage: g.Coverage, Reason: "Search for a type, or select a box in Architecture first."}
	sel := req.Sel
	auto := false
	if sel == "" {
		sel = pickFocusSeed(g, req.Root)
		auto = sel != ""
	}
	if sel == "" {
		return empty
	}
	byID := g.NodeByID()
	if _, ok := byID[sel]; !ok {
		return empty
	}
	keep := map[string]bool{sel: true}
	var edges []GraphEdge
	hop := map[string]bool{}
	for _, e := range g.Edges {
		if e.FromID != sel && e.ToID != sel {
			continue
		}
		keep[e.FromID] = true
		keep[e.ToID] = true
		edges = append(edges, toGraphEdge(e))
		if e.Kind == model.RelImplements || e.Kind == model.RelBinds {
			other := e.FromID
			if other == sel {
				other = e.ToID
			}
			hop[other] = true
		}
	}
	for _, e := range g.Edges {
		if e.Kind != model.RelImplements && e.Kind != model.RelBinds {
			continue
		}
		if !hop[e.FromID] && !hop[e.ToID] {
			continue
		}
		keep[e.FromID] = true
		keep[e.ToID] = true
		edges = append(edges, toGraphEdge(e))
	}
	reason := ""
	if auto {
		reason = "Starting at " + byID[sel].Name + ". Search to focus something else."
	}
	return GraphView{
		Lens: "focus", Path: req.Scope, Selection: sel,
		Nodes: nodesFrom(byID, keep), Edges: uniqueGraphEdges(edges),
		Cycles: []string{}, ChildPaths: []string{}, Coverage: g.Coverage,
		Reason: reason,
	}
}

func pickFocusSeed(g model.Graph, root string) string {
	deg := map[string]int{}
	for _, e := range g.Edges {
		deg[e.FromID]++
		deg[e.ToID]++
	}
	best := ""
	bestKind := 0
	bestDeg := -1
	for _, n := range g.Nodes {
		if !fileUnderRoot(n.File, root) {
			continue
		}
		k := 0
		switch n.Kind {
		case model.KindEndpoint:
			k = 3
		case model.KindType:
			k = 2
		case model.KindFile:
			k = 1
		default:
			continue
		}
		d := deg[n.ID]
		if k > bestKind || (k == bestKind && d > bestDeg) || (k == bestKind && d == bestDeg && (best == "" || n.ID < best)) {
			best, bestKind, bestDeg = n.ID, k, d
		}
	}
	return best
}

func fileUnderRoot(file, root string) bool {
	f := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	r := strings.ToLower(strings.ReplaceAll(root, "\\", "/"))
	if r == "" {
		return true
	}
	return f == r || strings.HasPrefix(f, r+"/")
}

func projectFlow(g model.Graph, req GraphViewRequest) GraphView {
	start := req.Sel
	if start == "" {
		start = pickFocusSeed(g, req.Root)
	}
	wantOverlay := map[string]bool{}
	for _, o := range req.Overlays {
		o = strings.TrimSpace(o)
		if o != "" {
			wantOverlay[o] = true
		}
	}
	kinds := []string{model.RelCalls, model.RelHandles, model.RelWrites, model.RelReads, model.RelCarriesData}
	if wantOverlay["deps"] {
		kinds = append(kinds, model.RelInjects)
	}
	if wantOverlay["async"] {
		kinds = append(kinds, model.RelAwaits, model.RelForks, model.RelJoins, model.RelRaces)
	}
	if !wantOverlay["data"] {
		kinds = filterOut(kinds, model.RelCarriesData, model.RelWrites, model.RelReads)
	}
	skipUnresolved := !wantOverlay["unresolved"]
	minConf := 0.5
	if wantOverlay["unresolved"] {
		minConf = 0
	}
	res := query.BoundedPath(g, query.PathRequest{
		From: start, MaxDepth: 8, MaxNodes: 60, Kinds: kinds,
		MinConfidence: minConf, SkipUnresolved: skipUnresolved,
	})
	keep := map[string]bool{}
	for _, n := range res.Nodes {
		keep[n.ID] = true
	}
	edges := make([]GraphEdge, 0, len(res.Edges))
	for _, e := range res.Edges {
		if isSelfLoopOverlay(e) {
			continue
		}
		edges = append(edges, toGraphEdge(e))
	}
	hidden := 0
	if !wantOverlay["deps"] {
		edges, keep, hidden = attachSpineInjects(g, keep, edges)
	}
	edges, keep = attachBoundAdapters(g, keep, edges, wantOverlay["data"])
	reason := res.Reason
	if hidden > 0 {
		extra := fmt.Sprintf("Hiding %d extra constructor ports. Toggle Deps to show them.", hidden)
		if reason != "" {
			reason = reason + " " + extra
		} else {
			reason = extra
		}
	}
	return GraphView{
		Lens: "flow", Path: req.Scope, Selection: start,
		Nodes: nodesFrom(g.NodeByID(), keep), Edges: uniqueGraphEdges(edges),
		Cycles: []string{}, ChildPaths: []string{}, Coverage: g.Coverage,
		Truncated: res.Truncated, Reason: reason,
	}
}

func isSelfLoopOverlay(e model.Edge) bool {
	if e.FromID != e.ToID {
		return false
	}
	switch e.Kind {
	case model.RelAwaits, model.RelForks, model.RelJoins, model.RelRaces:
		return true
	default:
		return false
	}
}

func attachSpineInjects(g model.Graph, keep map[string]bool, edges []GraphEdge) ([]GraphEdge, map[string]bool, int) {
	byID := g.NodeByID()
	called := map[string]map[string]bool{}
	writers := portsWithWriterAdapter(g)
	injectsFrom := map[string][]model.Edge{}
	for _, e := range g.Edges {
		switch e.Kind {
		case model.RelCalls:
			if called[e.FromID] == nil {
				called[e.FromID] = map[string]bool{}
			}
			called[e.FromID][e.ToID] = true
		case model.RelInjects:
			injectsFrom[e.FromID] = append(injectsFrom[e.FromID], e)
		}
	}
	hidden := 0
	seed := make([]string, 0, len(keep))
	for id := range keep {
		seed = append(seed, id)
	}
	for _, id := range seed {
		for _, e := range injectsFrom[id] {
			if keep[e.ToID] {
				edges = append(edges, toGraphEdge(e))
				continue
			}
			port := byID[e.ToID]
			if spineInject(byID[id], port, called[id], writers) {
				keep[e.ToID] = true
				edges = append(edges, toGraphEdge(e))
				continue
			}
			hidden++
		}
	}
	return edges, keep, hidden
}

func spineInject(consumer, port model.Node, called map[string]bool, writers map[string]bool) bool {
	if port.ID == "" {
		return false
	}
	if called[port.ID] {
		return true
	}
	if isSupportPort(port.Name) {
		return false
	}
	if isPersistencePort(port.Name) || writers[port.ID] {
		return true
	}
	return tokenOverlap(consumer.Name, port.Name) >= 1
}

func isSupportPort(name string) bool {
	n := strings.ToLower(name)
	for _, s := range []string{"dispatcher", "notifier", "notification", "mailer", "email", "queue", "logger", "template", "metrics", "telemetry"} {
		if strings.Contains(n, s) {
			return true
		}
	}
	return n == "clock" || strings.HasSuffix(n, "clock")
}

func isPersistencePort(name string) bool {
	n := strings.ToLower(name)
	for _, s := range []string{"repository", "repo", "store", "database", "sqlite", "dbcontext"} {
		if strings.Contains(n, s) {
			return true
		}
	}
	return false
}

func tokenOverlap(a, b string) int {
	sa := contentTokens(a)
	sb := contentTokens(b)
	n := 0
	for t := range sa {
		if sb[t] {
			n++
		}
	}
	return n
}

func contentTokens(name string) map[string]bool {
	out := map[string]bool{}
	cur := strings.Builder{}
	flush := func() {
		t := strings.ToLower(cur.String())
		cur.Reset()
		if t == "" || t == "use" || t == "case" || t == "service" || t == "port" || t == "impl" {
			return
		}
		out[t] = true
	}
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			flush()
			cur.WriteRune(r)
			continue
		}
		if r == '-' || r == '_' {
			flush()
			continue
		}
		cur.WriteRune(r)
	}
	flush()
	return out
}

func portsWithWriterAdapter(g model.Graph) map[string]bool {
	adapterOf := map[string][]string{}
	for _, e := range g.Edges {
		if e.Kind == model.RelBinds || e.Kind == model.RelImplements {
			adapterOf[e.ToID] = append(adapterOf[e.ToID], e.FromID)
		}
	}
	writes := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == model.RelWrites || e.Kind == model.RelCarriesData {
			writes[e.FromID] = true
		}
	}
	out := map[string]bool{}
	for port, adapters := range adapterOf {
		for _, a := range adapters {
			if writes[a] {
				out[port] = true
				break
			}
		}
	}
	return out
}

// attachBoundAdapters adds the composition-chosen (or sole) implementation of
// each port already on the path, so endpoint → use case → port reaches sqlite.
func attachBoundAdapters(g model.Graph, keep map[string]bool, edges []GraphEdge, wantData bool) ([]GraphEdge, map[string]bool) {
	byID := g.NodeByID()
	boundTo := map[string][]model.Edge{}
	implTo := map[string][]model.Edge{}
	implFrom := map[string][]model.Edge{}
	for _, e := range g.Edges {
		switch e.Kind {
		case model.RelBinds:
			boundTo[e.ToID] = append(boundTo[e.ToID], e)
		case model.RelImplements:
			implTo[e.ToID] = append(implTo[e.ToID], e)
			implFrom[e.FromID] = append(implFrom[e.FromID], e)
		}
	}
	added := map[string]bool{}
	seed := make([]string, 0, len(keep))
	for id := range keep {
		seed = append(seed, id)
	}
	attachFrom := func(e model.Edge) {
		if keep[e.FromID] {
			return
		}
		keep[e.FromID] = true
		added[e.FromID] = true
		edges = append(edges, toGraphEdge(e))
	}
	for _, id := range seed {
		cands := boundTo[id]
		if len(cands) == 0 && len(implTo[id]) == 1 {
			cands = implTo[id]
		}
		for _, e := range cands {
			attachFrom(e)
		}
		n, ok := byID[id]
		if !ok || n.Kind != model.KindMethod {
			continue
		}
		for _, e := range implTo[id] {
			attachFrom(e)
		}
		for _, e := range implFrom[id] {
			if keep[e.ToID] {
				continue
			}
			keep[e.ToID] = true
			added[e.ToID] = true
			edges = append(edges, toGraphEdge(e))
		}
	}
	if wantData {
		fromWriter := map[string]bool{}
		for id := range added {
			fromWriter[id] = true
		}
		implOnPath := map[string]bool{}
		for _, e := range g.Edges {
			if e.Kind == model.RelImplements && keep[e.ToID] {
				implOnPath[e.FromID] = true
			}
		}
		picked := map[string]bool{}
		for _, e := range g.Edges {
			if e.Kind != model.RelContains || !added[e.FromID] {
				continue
			}
			if implOnPath[e.ToID] {
				fromWriter[e.ToID] = true
				picked[e.FromID] = true
			}
		}
		for _, e := range g.Edges {
			if e.Kind != model.RelContains || !added[e.FromID] || picked[e.FromID] {
				continue
			}
			n, ok := byID[e.ToID]
			if !ok || n.Kind != model.KindMethod || !methodLooksLikeWrite(n.Name) {
				continue
			}
			fromWriter[e.ToID] = true
			picked[e.FromID] = true
		}
		for _, e := range g.Edges {
			if !fromWriter[e.FromID] {
				continue
			}
			if e.Unresolved {
				continue
			}
			if e.Kind != model.RelWrites && e.Kind != model.RelCarriesData {
				continue
			}
			if n, ok := byID[e.FromID]; ok && n.Kind == model.KindMethod && !methodLooksLikeWrite(n.Name) {
				continue
			}
			if !keep[e.FromID] {
				keep[e.FromID] = true
			}
			keep[e.ToID] = true
			edges = append(edges, toGraphEdge(e))
		}
	}
	return edges, keep
}

func methodLooksLikeWrite(name string) bool {
	n := strings.ToLower(name)
	if i := strings.LastIndex(n, "."); i >= 0 {
		n = n[i+1:]
	}
	if strings.HasPrefix(n, "prepare") || strings.Contains(n, "transaction") {
		return false
	}
	for _, tok := range []string{"insert", "update", "upsert", "delete", "save", "set", "run", "exec"} {
		if strings.HasPrefix(n, tok) {
			return true
		}
	}
	return false
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
			Abstract: n.Abstract, Path: n.File, FileRoot: n.File, Kind: n.Kind, Source: extraSource(n), Line: n.Line,
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
