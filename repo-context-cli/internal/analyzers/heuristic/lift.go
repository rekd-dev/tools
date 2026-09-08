package heuristic

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"repo-context-cli/internal/model"
	"repo-context-cli/internal/modules"
)

type Inventory struct {
	Modules    []modules.Module
	Deps       []modules.Dep
	Types      []Type
	DI         []DI
	Endpoints  []Endpoint
	Events     []Event
	RepoRoot   string
}

type Type struct {
	Name, Kind, File, Namespace, Extends string
	Implements                             []string
}

type DI struct {
	Interface, Implementation, Lifetime, File string
}

type Endpoint struct {
	Route, Method, Handler, File, ReturnType string
}

type Event struct {
	File, Direction, Mechanism, EventType string
}

var (
	csCtorRe     = regexp.MustCompile(`(?m)public\s+(\w+)\s*\(([^)]*)\)`)
	csParamRe    = regexp.MustCompile(`(?:this\s+)?([\w.<>,]+)\s+(\w+)\s*(?:,|$)`)
	tsCtorRe     = regexp.MustCompile(`constructor\s*\(([^)]*)\)`)
	tsParamRe    = regexp.MustCompile(`(?:private|public|protected|readonly)?\s*(\w+)\s*:\s*(\w+)`)
	awaitRe      = regexp.MustCompile(`\bawait\s+([\w.]+)`)
	promiseAllRe = regexp.MustCompile(`Promise\.(all|allSettled|race)\s*\(`)
	taskWhenRe   = regexp.MustCompile(`Task\.(WhenAll|WhenAny|WhenEach)\s*\(`)
	callRe       = regexp.MustCompile(`\b([A-Z][\w]*)\.([A-Z]\w*)\s*\(`)
	saveSinkRe   = regexp.MustCompile(`\b(Save|Insert|Update|Execute|query|fetch)\s*\(`)
)

func Lift(inv Inventory) model.Graph {
	g := model.Graph{
		Coverage: []model.Coverage{{Analyzer: "heuristic", Status: "ran"}},
	}
	byName := map[string][]model.Node{}
	fileLang := map[string]string{}

	for _, m := range inv.Modules {
		lang := m.Language
		if lang == "" {
			lang = model.LanguageOfPath(m.Path)
		}
		fileLang[model.Slash(m.Path)] = lang
		n := model.Node{
			ID: model.FileID(lang, m.Path), Kind: model.KindFile, Name: filepath.Base(m.Path),
			QualifiedName: m.DrillPath, Language: lang, File: model.Slash(m.Path),
			Layer: m.Layer, Abstract: m.Abstract,
			Extra: map[string]string{"moduleId": m.ID},
		}
		g.Nodes = append(g.Nodes, n)
	}
	for _, d := range inv.Deps {
		if d.Kind == "external" {
			continue
		}
		fromLang := langOf(inv, d.FromID)
		toLang := langOf(inv, d.ToID)
		fromPath := pathOf(inv, d.FromID)
		toPath := pathOf(inv, d.ToID)
		if fromPath == "" || toPath == "" {
			continue
		}
		g.Edges = append(g.Edges, edge(model.FileID(fromLang, fromPath), model.FileID(toLang, toPath), model.RelImports, model.SrcHeuristic, model.ConfidenceHeuristic, d.ViaFile, 0, "module import"))
	}

	for _, t := range inv.Types {
		lang := langOfFile(t.File, fileLang)
		n := model.Node{
			ID: model.TypeID(lang, t.File, t.Name), Kind: model.KindType, Name: t.Name,
			QualifiedName: qualify(t.Namespace, t.Name), Language: lang, File: model.Slash(t.File),
			Abstract: t.Kind == "interface" || t.Kind == "protocol",
			Extra:    map[string]string{"typeKind": t.Kind},
		}
		if fm := fileNode(inv, t.File); fm.Layer != "" {
			n.Layer = fm.Layer
		}
		g.Nodes = append(g.Nodes, n)
		byName[t.Name] = append(byName[t.Name], n)
		g.Edges = append(g.Edges, edge(model.FileID(lang, t.File), n.ID, model.RelContains, model.SrcHeuristic, model.ConfidenceHeuristic, t.File, 0, "declared in file"))
	}
	for _, t := range inv.Types {
		lang := langOfFile(t.File, fileLang)
		nID := model.TypeID(lang, t.File, t.Name)
		for _, iface := range t.Implements {
			if tgt := resolveType(iface, t.File, byName, lang); tgt != "" {
				g.Edges = append(g.Edges, edge(nID, tgt, model.RelImplements, model.SrcHeuristic, model.ConfidenceHeuristic, t.File, 0, "implements"))
			} else {
				un := model.TypeID(lang, t.File, iface)
				g.Nodes = append(g.Nodes, model.Node{ID: un, Kind: model.KindType, Name: iface, Language: lang, Abstract: true})
				e := edge(nID, un, model.RelImplements, model.SrcHeuristic, model.ConfidenceUnresolved, t.File, 0, "unresolved implementee")
				e.Unresolved = true
				g.Edges = append(g.Edges, e)
			}
		}
		if t.Extends != "" {
			if tgt := resolveType(t.Extends, t.File, byName, lang); tgt != "" {
				g.Edges = append(g.Edges, edge(nID, tgt, model.RelExtends, model.SrcHeuristic, model.ConfidenceHeuristic, t.File, 0, "extends"))
			}
		}
	}

	for _, di := range inv.DI {
		lang := langOfFile(di.File, fileLang)
		impl := resolveType(di.Implementation, di.File, byName, lang)
		iface := resolveType(di.Interface, di.File, byName, lang)
		if impl == "" || iface == "" {
			continue
		}
		e := edge(impl, iface, model.RelBinds, model.SrcHeuristic, model.ConfidenceHeuristic, di.File, 0, "lifetime="+di.Lifetime)
		g.Edges = append(g.Edges, e)
	}

	for _, ep := range inv.Endpoints {
		lang := langOfFile(ep.File, fileLang)
		id := model.EndpointID(lang, ep.File, ep.Method, ep.Route)
		g.Nodes = append(g.Nodes, model.Node{
			ID: id, Kind: model.KindEndpoint, Name: ep.Method + " " + ep.Route,
			QualifiedName: ep.Handler, Language: lang, File: model.Slash(ep.File),
			Extra: map[string]string{"handler": ep.Handler, "returnType": ep.ReturnType},
		})
		if handler := resolveType(ep.Handler, ep.File, byName, lang); handler != "" {
			g.Edges = append(g.Edges, edge(id, handler, model.RelHandles, model.SrcHeuristic, model.ConfidenceHeuristic, ep.File, 0, "handler"))
		}
		g.Edges = append(g.Edges, edge(model.FileID(lang, ep.File), id, model.RelContains, model.SrcHeuristic, model.ConfidenceHeuristic, ep.File, 0, "endpoint"))
	}

	for _, ev := range inv.Events {
		lang := langOfFile(ev.File, fileLang)
		id := model.EventID(lang, ev.File, ev.Mechanism, ev.EventType)
		g.Nodes = append(g.Nodes, model.Node{
			ID: id, Kind: model.KindEvent, Name: ev.EventType, Language: lang, File: model.Slash(ev.File),
			Extra: map[string]string{"mechanism": ev.Mechanism, "direction": ev.Direction},
		})
		kind := model.RelPublishes
		if ev.Direction == "handle" || ev.Direction == "subscribe" {
			kind = model.RelSubscribes
		}
		g.Edges = append(g.Edges, edge(model.FileID(lang, ev.File), id, kind, model.SrcHeuristic, model.ConfidenceHeuristic, ev.File, 0, ev.Mechanism))
	}

	scanFiles(&g, inv, byName, fileLang)
	return g
}

func scanFiles(g *model.Graph, inv Inventory, byName map[string][]model.Node, fileLang map[string]string) {
	if inv.RepoRoot == "" {
		return
	}
	for _, m := range inv.Modules {
		full := filepath.Join(inv.RepoRoot, filepath.FromSlash(m.Path))
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		text := string(data)
		lang := m.Language
		lines := strings.Split(text, "\n")

		if lang == "csharp" {
			for _, match := range csCtorRe.FindAllStringSubmatchIndex(text, -1) {
				typeName := text[match[2]:match[3]]
				params := text[match[4]:match[5]]
				line := lineOf(text, match[0])
				from := resolveType(typeName, m.Path, byName, lang)
				if from == "" {
					continue
				}
				for _, p := range csParamRe.FindAllStringSubmatch(params, -1) {
					ptype := stripGeneric(p[1])
					if tgt := resolveType(ptype, m.Path, byName, lang); tgt != "" {
						g.Edges = append(g.Edges, edge(from, tgt, model.RelInjects, model.SrcHeuristic, model.ConfidenceHeuristic, m.Path, line, "ctor "+p[2]))
					}
				}
			}
		}
		if lang == "typescript" {
			for _, match := range tsCtorRe.FindAllStringSubmatchIndex(text, -1) {
				params := text[match[2]:match[3]]
				line := lineOf(text, match[0])
				owner := enclosingType(m.Path, lang, byName)
				for _, p := range tsParamRe.FindAllStringSubmatch(params, -1) {
					if tgt := resolveType(p[2], m.Path, byName, lang); tgt != "" && owner != "" {
						g.Edges = append(g.Edges, edge(owner, tgt, model.RelInjects, model.SrcHeuristic, model.ConfidenceHeuristic, m.Path, line, "ctor "+p[1]))
					}
				}
			}
		}

		fromOwner := fileOrType(m, lang, byName)
		for i, line := range lines {
			if awaitRe.MatchString(line) {
				g.Edges = append(g.Edges, edge(fromOwner, fromOwner, model.RelAwaits, model.SrcHeuristic, model.ConfidenceHeuristicCall, m.Path, i+1, strings.TrimSpace(line)))
			}
			if pm := promiseAllRe.FindStringSubmatch(line); len(pm) > 1 {
				kind := model.RelForks
				if pm[1] == "race" {
					kind = model.RelRaces
				}
				g.Edges = append(g.Edges, edge(fromOwner, fromOwner, kind, model.SrcHeuristic, model.ConfidenceHeuristic, m.Path, i+1, pm[0]))
			}
			if tm := taskWhenRe.FindStringSubmatch(line); len(tm) > 1 {
				kind := model.RelForks
				if tm[1] == "WhenAny" {
					kind = model.RelRaces
				}
				g.Edges = append(g.Edges, edge(fromOwner, fromOwner, kind, model.SrcHeuristic, model.ConfidenceHeuristic, m.Path, i+1, tm[0]))
			}
			if strings.Contains(line, "CancellationToken.None") || strings.Contains(line, ".Wait()") || strings.Contains(line, ".Result") || strings.Contains(line, ".GetAwaiter().GetResult()") {
				g.Edges = append(g.Edges, edge(fromOwner, fromOwner, model.RelAwaits, model.SrcHeuristic, model.ConfidenceHeuristicCall, m.Path, i+1, "sync-over-async or dropped cancellation: "+strings.TrimSpace(line)))
			}
		}

		for _, match := range callRe.FindAllStringSubmatchIndex(text, -1) {
			recv := text[match[2]:match[3]]
			meth := text[match[4]:match[5]]
			line := lineOf(text, match[0])
			from := fileOrType(m, lang, byName)
			tgtType := resolveType(recv, m.Path, byName, lang)
			if tgtType == "" {
				continue
			}
			methodID := model.MethodID(lang, m.Path, recv, meth)
			g.Nodes = append(g.Nodes, model.Node{ID: methodID, Kind: model.KindMethod, Name: recv + "." + meth, Language: lang, File: model.Slash(m.Path), Line: line})
			g.Edges = append(g.Edges, edge(from, methodID, model.RelCalls, model.SrcHeuristic, model.ConfidenceHeuristicCall, m.Path, line, recv+"."+meth))
			if saveSinkRe.MatchString(meth + "(") {
				sink := model.SinkID(lang, m.Path, meth)
				g.Nodes = append(g.Nodes, model.Node{ID: sink, Kind: model.KindSink, Name: meth, Language: lang, File: model.Slash(m.Path)})
				g.Edges = append(g.Edges, edge(methodID, sink, model.RelWrites, model.SrcHeuristic, model.ConfidenceHeuristicCall, m.Path, line, "persistence-like call"))
				g.Edges = append(g.Edges, edge(from, sink, model.RelCarriesData, model.SrcHeuristic, model.ConfidenceHeuristicCall, m.Path, line, "symbol-level lineage via "+meth))
			}
		}
	}
}

func edge(from, to, kind, src string, conf float64, file string, line int, detail string) model.Edge {
	return model.Edge{
		ID: fmt.Sprintf("%s|%s|%s|%s|%d", from, to, kind, model.Slash(file), line),
		FromID: from, ToID: to, Kind: kind, Source: src, Confidence: conf, Analyzer: "heuristic",
		File: model.Slash(file), Line: line, Detail: detail,
	}
}

func langOf(inv Inventory, id string) string {
	for _, m := range inv.Modules {
		if m.ID == id {
			return m.Language
		}
	}
	return model.LanguageOfPath(id)
}

func pathOf(inv Inventory, id string) string {
	for _, m := range inv.Modules {
		if m.ID == id {
			return m.Path
		}
	}
	return id
}

func langOfFile(file string, fileLang map[string]string) string {
	if l, ok := fileLang[model.Slash(file)]; ok {
		return l
	}
	return model.LanguageOfPath(file)
}

func fileNode(inv Inventory, file string) modules.Module {
	want := model.Slash(file)
	for _, m := range inv.Modules {
		if model.Slash(m.Path) == want || m.ID == file {
			return m
		}
	}
	return modules.Module{}
}

func qualify(ns, name string) string {
	if ns == "" {
		return name
	}
	return ns + "." + name
}

func stripGeneric(s string) string {
	if i := strings.Index(s, "<"); i >= 0 {
		return s[:i]
	}
	return s
}

func resolveType(name, fromFile string, byName map[string][]model.Node, lang string) string {
	name = stripGeneric(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	cands := byName[name]
	if len(cands) == 1 {
		return cands[0].ID
	}
	slash := model.Slash(fromFile)
	for _, n := range cands {
		if n.File == slash {
			return n.ID
		}
	}
	if len(cands) > 0 {
		return cands[0].ID
	}
	return ""
}

func enclosingType(file, lang string, byName map[string][]model.Node) string {
	slash := model.Slash(file)
	for _, nodes := range byName {
		for _, n := range nodes {
			if n.File == slash && n.Language == lang {
				return n.ID
			}
		}
	}
	return model.FileID(lang, file)
}

func fileOrType(m modules.Module, lang string, byName map[string][]model.Node) string {
	if id := enclosingType(m.Path, lang, byName); id != "" {
		return id
	}
	return model.FileID(lang, m.Path)
}

func lineOf(text string, idx int) int {
	if idx <= 0 {
		return 1
	}
	return strings.Count(text[:idx], "\n") + 1
}
