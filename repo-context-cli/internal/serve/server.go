package serve

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"repo-context-cli/internal/fitness"
	"repo-context-cli/internal/graph"
	"repo-context-cli/internal/model"
	"repo-context-cli/internal/modules"

	_ "modernc.org/sqlite"
)

// Options configures the local architecture API server.
type Options struct {
	RepoPath  string
	DBPath    string
	StaticDir string
	Addr      string
	RulesPath string
}

// Run starts an HTTP server serving inventory JSON APIs and optional static SPA.
func Run(opts Options) error {
	hub := &graphHub{dbPath: opts.DBPath, repoPath: opts.RepoPath}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/modules", func(w http.ResponseWriter, r *http.Request) {
		g, err := hub.graph()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, g.Modules)
	})
	mux.HandleFunc("/api/module-deps", func(w http.ResponseWriter, r *http.Request) {
		g, err := hub.graph()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, g.Deps)
	})
	mux.HandleFunc("/api/fitness", func(w http.ResponseWriter, r *http.Request) {
		g, err := hub.graph()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		rulesPath := opts.RulesPath
		if rulesPath == "" {
			rulesPath = fitness.FindRulesFile(opts.RepoPath)
		}
		rules, src, _ := fitness.LoadRules(rulesPath)
		meta := readMeta(opts.DBPath)
		report := fitness.Analyze(meta["repo"], opts.RepoPath, meta["gitSha"], g.Modules, g.Deps, rules, src)
		writeJSON(w, report)
	})
	mux.HandleFunc("/api/view", func(w http.ResponseWriter, r *http.Request) {
		prefix := r.URL.Query().Get("path")
		g, err := hub.graph()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, projectView(g, prefix))
	})
	mux.HandleFunc("/api/source", func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" || strings.Contains(file, "..") {
			http.Error(w, "bad file", 400)
			return
		}
		full := filepath.Join(opts.RepoPath, filepath.FromSlash(file))
		data, err := os.ReadFile(full)
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/api/meta", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, readMeta(opts.DBPath))
	})
	registerGraphAPI(mux, hub)

	if opts.StaticDir != "" {
		fs := http.FileServer(http.Dir(opts.StaticDir))
		mux.Handle("/", fs)
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><html><body style="font-family:sans-serif;padding:2rem">
<h1>repo-context serve</h1>
<p>API ready. Point <code>arch-view</code> at this origin or pass <code>--static</code> to the built SPA.</p>
<ul>
<li><a href="/api/health">/api/health</a></li>
<li><a href="/api/fitness">/api/fitness</a></li>
<li><a href="/api/view">/api/view</a></li>
<li><a href="/api/search?q=IEntity">/api/search</a></li>
<li><a href="/api/coverage">/api/coverage</a></li>
</ul></body></html>`)
		})
	}

	fmt.Fprintf(os.Stderr, "serving %s on http://%s\n", opts.RepoPath, opts.Addr)
	return http.ListenAndServe(opts.Addr, withCORS(mux))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

type graphHub struct {
	dbPath      string
	repoPath    string
	mu          sync.Mutex
	cached      *modules.Graph
	cachedFacts *model.Graph
}

func (h *graphHub) graph() (modules.Graph, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cached != nil {
		return *h.cached, nil
	}
	g, err := loadGraph(h.dbPath, h.repoPath)
	if err != nil {
		return modules.Graph{}, err
	}
	h.cached = &g
	return g, nil
}

func loadGraph(dbPath, repoPath string) (modules.Graph, error) {
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return modules.Graph{}, err
	}
	defer db.Close()

	g := modules.Graph{}
	ws := modules.DiscoverWorkspaceRoots(repoPath)
	rows, err := db.Query(`SELECT id, kind, path, language, layer, abstract, namespace, drill_path, source FROM modules`)
	if err != nil {
		return g, fmt.Errorf("modules table missing — run inventory first: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m modules.Module
		var abs int
		var ns, drill, layer sql.NullString
		if err := rows.Scan(&m.ID, &m.Kind, &m.Path, &m.Language, &layer, &abs, &ns, &drill, &m.Source); err != nil {
			return g, err
		}
		m.Abstract = abs == 1
		m.Layer = layer.String
		m.Namespace = ns.String
		m.DrillPath = modules.ComputeDrillPath(m.Language, m.Namespace, m.Path, ws)
		_ = drill
		g.Modules = append(g.Modules, m)
	}

	// External imports
	extRows, err := db.Query(`SELECT module_id, package FROM module_ext_imports`)
	if err == nil {
		defer extRows.Close()
		extMap := map[string][]string{}
		for extRows.Next() {
			var id, pkg string
			extRows.Scan(&id, &pkg)
			extMap[id] = append(extMap[id], pkg)
		}
		for i := range g.Modules {
			g.Modules[i].ExtImports = extMap[g.Modules[i].ID]
		}
	}

	drows, err := db.Query(`SELECT from_id, to_id, kind, via_file FROM module_deps`)
	if err != nil {
		return g, err
	}
	defer drows.Close()
	for drows.Next() {
		var d modules.Dep
		var via sql.NullString
		drows.Scan(&d.FromID, &d.ToID, &d.Kind, &via)
		d.ViaFile = via.String
		g.Deps = append(g.Deps, d)
	}
	return g, nil
}

func readMeta(dbPath string) map[string]string {
	out := map[string]string{}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return out
	}
	defer db.Close()
	rows, err := db.Query(`SELECT key, value FROM metadata`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		out[k] = v
	}
	return out
}

// ViewNode is a projected architecture node for the SPA.
type ViewNode struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Layer     int    `json:"layer"`
	ArchLayer string `json:"archLayer,omitempty"`
	Leaf      bool   `json:"leaf"`
	Abstract  bool   `json:"abstract"`
	Cycle     bool   `json:"cycle"`
	Path      string `json:"path,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Source    string `json:"source,omitempty"`
	Line      int    `json:"line,omitempty"`
}

// ViewEdge is an aggregated edge between view nodes.
type ViewEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// View is the drill-down projection.
type View struct {
	Path       string     `json:"path"`
	Nodes      []ViewNode `json:"nodes"`
	Edges      []ViewEdge `json:"edges"`
	Cycles     []string   `json:"cycles"`
	ChildPaths []string   `json:"childPaths"`
}

func projectView(g modules.Graph, prefix string) View {
	prefixParts := modules.SplitDrillPath(prefix)
	childMods := map[string][]modules.Module{}
	for _, m := range g.Modules {
		parts := modules.SplitDrillPath(m.DrillPath)
		if !hasPrefix(parts, prefixParts) {
			continue
		}
		if len(parts) <= len(prefixParts) {
			continue
		}
		child := parts[len(prefixParts)]
		childMods[child] = append(childMods[child], m)
	}

	nodes := make([]ViewNode, 0)
	nodeIDs := map[string]bool{}
	for child, ms := range childMods {
		leaf := true
		abs := false
		arch := ""
		path := ""
		layers := map[string]bool{}
		for _, m := range ms {
			parts := modules.SplitDrillPath(m.DrillPath)
			if len(parts) > len(prefixParts)+1 {
				leaf = false
			}
			if m.Abstract {
				abs = true
			}
			if m.Layer != "" {
				layers[m.Layer] = true
			}
			if path == "" {
				path = m.Path
			}
		}
		if len(layers) == 1 {
			for l := range layers {
				arch = l
			}
		}
		nodes = append(nodes, ViewNode{
			ID: child, Label: child, Leaf: leaf, Abstract: abs,
			ArchLayer: arch, Path: path,
		})
		nodeIDs[child] = true
	}

	modToChild := map[string]string{}
	for child, ms := range childMods {
		for _, m := range ms {
			modToChild[m.ID] = child
		}
	}

	edges := make([]ViewEdge, 0)
	edgeSeen := map[string]bool{}
	var gEdges []graph.Edge
	for _, d := range g.Deps {
		if d.Kind == "external" {
			continue
		}
		fc, okF := modToChild[d.FromID]
		tc, okT := modToChild[d.ToID]
		if !okF || !okT || fc == tc {
			continue
		}
		key := fc + "->" + tc
		if edgeSeen[key] {
			continue
		}
		edgeSeen[key] = true
		edges = append(edges, ViewEdge{From: fc, To: tc, Kind: d.Kind})
		gEdges = append(gEdges, graph.Edge{From: fc, To: tc})
	}

	nids := make([]string, 0, len(nodeIDs))
	for id := range nodeIDs {
		nids = append(nids, id)
	}
	lr := graph.AssignLayers(nids, gEdges)
	for i := range nodes {
		if layer, ok := lr.Layers[nodes[i].ID]; ok {
			nodes[i].Layer = layer
		}
		for _, c := range lr.Cycles {
			for _, n := range c {
				if n == nodes[i].ID {
					nodes[i].Cycle = true
				}
			}
		}
	}
	cycles := make([]string, 0)
	for _, c := range lr.Cycles {
		cycles = append(cycles, graph.FormatCycle(c))
	}
	children := make([]string, 0, len(childMods))
	for c := range childMods {
		children = append(children, modules.JoinDrillPath(append(append([]string{}, prefixParts...), c)...))
	}

	return View{
		Path:       prefix,
		Nodes:      nodes,
		Edges:      edges,
		Cycles:     cycles,
		ChildPaths: children,
	}
}

func hasPrefix(parts, prefix []string) bool {
	if len(prefix) > len(parts) {
		return false
	}
	for i := range prefix {
		if parts[i] != prefix[i] {
			return false
		}
	}
	return true
}
