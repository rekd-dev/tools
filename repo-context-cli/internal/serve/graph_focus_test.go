package serve

import (
	"strconv"
	"strings"
	"testing"

	"repo-context-cli/internal/model"
)

func TestPickFocusSeedPrefersConnectedType(t *testing.T) {
	file := model.Node{ID: "file:ts:a.ts", Kind: model.KindFile, Name: "a.ts", File: "pkg/a.ts"}
	typ := model.Node{ID: "type:ts:a.ts:Foo", Kind: model.KindType, Name: "Foo", File: "pkg/a.ts"}
	other := model.Node{ID: "type:ts:b.ts:Bar", Kind: model.KindType, Name: "Bar", File: "other/b.ts"}
	g := model.Graph{
		Nodes: []model.Node{file, typ, other},
		Edges: []model.Edge{
			{ID: "1", FromID: typ.ID, ToID: file.ID, Kind: model.RelContains},
			{ID: "2", FromID: other.ID, ToID: other.ID, Kind: model.RelCalls},
		},
	}
	got := pickFocusSeed(g, "pkg")
	if got != typ.ID {
		t.Fatalf("got %s", got)
	}
}

func TestFileUnderRoot(t *testing.T) {
	if !fileUnderRoot("packages/auth/src/cookie.ts", "packages/auth/src") {
		t.Fatal("cookie should be in auth/src")
	}
	if fileUnderRoot("apps/schedule-api/src/http/routes/auth.ts", "packages/auth/src") {
		t.Fatal("schedule auth route should not match packages/auth")
	}
	if !fileUnderRoot("pkg/a.ts", "") {
		t.Fatal("empty root is whole repo")
	}
}

func TestProjectFocusAutoPicksWithoutSel(t *testing.T) {
	typ := model.Node{ID: "type:ts:a.ts:Foo", Kind: model.KindType, Name: "Foo", File: "pkg/a.ts"}
	g := model.Graph{Nodes: []model.Node{typ}}
	v := projectFocus(g, GraphViewRequest{Lens: "focus", Root: "pkg"})
	if v.Selection != typ.ID {
		t.Fatalf("selection=%s reason=%s", v.Selection, v.Reason)
	}
	if len(v.Nodes) == 0 {
		t.Fatal("expected a node")
	}
}

func TestProjectFlowAttachesBoundSqliteAdapter(t *testing.T) {
	ep := model.Node{ID: "endpoint:ts:r.ts:POST:/clock-in", Kind: model.KindEndpoint, Name: "POST /clock-in", File: "r.ts"}
	uc := model.Node{ID: "type:ts:uc.ts:ClockIn", Kind: model.KindType, Name: "ClockIn", File: "uc.ts"}
	port := model.Node{ID: "type:ts:p.ts:Repo", Kind: model.KindType, Name: "Repo", File: "p.ts", Abstract: true}
	sql := model.Node{ID: "type:ts:s.ts:SqliteRepo", Kind: model.KindType, Name: "SqliteRepo", File: "s.ts"}
	sink := model.Node{ID: "sink:ts:s.ts:run", Kind: model.KindSink, Name: "run", File: "s.ts"}
	g := model.Graph{Nodes: []model.Node{ep, uc, port, sql, sink}, Edges: []model.Edge{
		{ID: "h", FromID: ep.ID, ToID: uc.ID, Kind: model.RelHandles},
		{ID: "i", FromID: uc.ID, ToID: port.ID, Kind: model.RelInjects},
		{ID: "b", FromID: sql.ID, ToID: port.ID, Kind: model.RelBinds},
		{ID: "w", FromID: sql.ID, ToID: sink.ID, Kind: model.RelWrites},
	}}
	v := projectFlow(g, GraphViewRequest{Lens: "flow", Sel: ep.ID, Overlays: []string{"data"}})
	ids := map[string]bool{}
	for _, n := range v.Nodes {
		ids[n.ID] = true
	}
	if !ids[sql.ID] {
		t.Fatalf("expected sqlite adapter on path, nodes=%v", ids)
	}
	if !ids[sink.ID] {
		t.Fatal("expected data sink when data overlay is on")
	}
}

func TestProjectFlowDataOverlayUsesMethodWrites(t *testing.T) {
	ep := model.Node{ID: "endpoint:ts:r.ts:POST:/clock-in", Kind: model.KindEndpoint, Name: "POST /clock-in", File: "r.ts"}
	uc := model.Node{ID: "type:ts:uc.ts:ClockIn", Kind: model.KindType, Name: "ClockIn", File: "uc.ts"}
	port := model.Node{ID: "type:ts:p.ts:Repo", Kind: model.KindType, Name: "Repo", File: "p.ts", Abstract: true}
	sql := model.Node{ID: "type:ts:s.ts:SqliteRepo", Kind: model.KindType, Name: "SqliteRepo", File: "s.ts"}
	insert := model.Node{ID: "method:ts:s.ts:SqliteRepo.insertSession", Kind: model.KindMethod, Name: "SqliteRepo.insertSession", File: "s.ts"}
	portInsert := model.Node{ID: "method:ts:p.ts:Repo.insertSession", Kind: model.KindMethod, Name: "insertSession", File: "p.ts"}
	sink := model.Node{ID: "sink:ts:s.ts:SqliteRepo", Kind: model.KindSink, Name: "SqliteRepo", File: "s.ts"}
	g := model.Graph{Nodes: []model.Node{ep, uc, port, sql, insert, portInsert, sink}, Edges: []model.Edge{
		{ID: "h", FromID: ep.ID, ToID: uc.ID, Kind: model.RelHandles},
		{ID: "i", FromID: uc.ID, ToID: port.ID, Kind: model.RelInjects},
		{ID: "b", FromID: sql.ID, ToID: port.ID, Kind: model.RelBinds},
		{ID: "call", FromID: uc.ID, ToID: portInsert.ID, Kind: model.RelCalls, Confidence: 0.92},
		{ID: "c", FromID: sql.ID, ToID: insert.ID, Kind: model.RelContains},
		{ID: "im", FromID: insert.ID, ToID: portInsert.ID, Kind: model.RelImplements, Confidence: 0.92},
		{ID: "w", FromID: insert.ID, ToID: sink.ID, Kind: model.RelWrites},
	}}
	v := projectFlow(g, GraphViewRequest{Lens: "flow", Sel: ep.ID, Overlays: []string{"data"}})
	ids := map[string]bool{}
	for _, n := range v.Nodes {
		ids[n.ID] = true
	}
	if !ids[insert.ID] || !ids[sink.ID] {
		t.Fatalf("expected method write sink, nodes=%v", ids)
	}
	find := model.Node{ID: "method:ts:s.ts:SqliteRepo.findById", Kind: model.KindMethod, Name: "findById", File: "s.ts"}
	g.Nodes = append(g.Nodes, find)
	g.Edges = append(g.Edges,
		model.Edge{ID: "c2", FromID: sql.ID, ToID: find.ID, Kind: model.RelContains},
		model.Edge{ID: "r2", FromID: find.ID, ToID: sink.ID, Kind: model.RelReads},
	)
	v = projectFlow(g, GraphViewRequest{Lens: "flow", Sel: ep.ID, Overlays: []string{"data"}})
	ids = map[string]bool{}
	for _, n := range v.Nodes {
		ids[n.ID] = true
	}
	if ids[find.ID] {
		t.Fatal("findById reads should not appear on the data overlay")
	}
	if !ids[sink.ID] {
		t.Fatal("adapter sink should stay on the data overlay via writes")
	}
}

func TestProjectFlowHidesSupportInjects(t *testing.T) {
	ep := model.Node{ID: "endpoint:ts:r.ts:POST:/clock-in", Kind: model.KindEndpoint, Name: "POST /clock-in", File: "r.ts"}
	uc := model.Node{ID: "type:ts:uc.ts:ClockInUseCase", Kind: model.KindType, Name: "ClockInUseCase", File: "uc.ts"}
	repo := model.Node{ID: "type:ts:p.ts:TimeSessionRepository", Kind: model.KindType, Name: "TimeSessionRepository", File: "p.ts", Abstract: true}
	clock := model.Node{ID: "type:ts:c.ts:Clock", Kind: model.KindType, Name: "Clock", File: "c.ts", Abstract: true}
	disp := model.Node{ID: "type:ts:n.ts:UserNotificationDispatcher", Kind: model.KindType, Name: "UserNotificationDispatcher", File: "n.ts"}
	sql := model.Node{ID: "type:ts:s.ts:SqliteRepo", Kind: model.KindType, Name: "SqliteRepo", File: "s.ts"}
	g := model.Graph{Nodes: []model.Node{ep, uc, repo, clock, disp, sql}, Edges: []model.Edge{
		{ID: "h", FromID: ep.ID, ToID: uc.ID, Kind: model.RelHandles},
		{ID: "i1", FromID: uc.ID, ToID: repo.ID, Kind: model.RelInjects},
		{ID: "i2", FromID: uc.ID, ToID: clock.ID, Kind: model.RelInjects},
		{ID: "i3", FromID: uc.ID, ToID: disp.ID, Kind: model.RelInjects},
		{ID: "b", FromID: sql.ID, ToID: repo.ID, Kind: model.RelBinds},
	}}
	v := projectFlow(g, GraphViewRequest{Lens: "flow", Sel: ep.ID, Overlays: []string{"data"}})
	ids := map[string]bool{}
	for _, n := range v.Nodes {
		ids[n.ID] = true
	}
	if !ids[repo.ID] || !ids[sql.ID] {
		t.Fatalf("expected persistence spine, nodes=%v", ids)
	}
	if ids[clock.ID] || ids[disp.ID] {
		t.Fatalf("support injects should be hidden, nodes=%v", ids)
	}
	if !strings.Contains(v.Reason, "Deps") {
		t.Fatalf("reason should mention Deps overlay: %q", v.Reason)
	}
	full := projectFlow(g, GraphViewRequest{Lens: "flow", Sel: ep.ID, Overlays: []string{"data", "deps"}})
	fullIDs := map[string]bool{}
	for _, n := range full.Nodes {
		fullIDs[n.ID] = true
	}
	if !fullIDs[clock.ID] || !fullIDs[disp.ID] {
		t.Fatalf("deps overlay should show support injects, nodes=%v", fullIDs)
	}
}

func TestMethodLooksLikeWrite(t *testing.T) {
	if !methodLooksLikeWrite("insertSession") || !methodLooksLikeWrite("SqliteRepo.insertSession") {
		t.Fatal("insertSession is a write")
	}
	if methodLooksLikeWrite("findById") || methodLooksLikeWrite("getPaidBreakSettleDelayMinutes") {
		t.Fatal("reads and Settle must not count as writes")
	}
	if !methodLooksLikeWrite("setStoreHours") {
		t.Fatal("setStoreHours is a write")
	}
}

func TestProjectFlowQuarantinesUnresolvedCalls(t *testing.T) {
	start := model.Node{ID: "type:ts:a.ts:UseCase", Kind: model.KindType, Name: "UseCase", File: "a.ts"}
	real := model.Node{ID: "method:ts:a.ts:Repo.save", Kind: model.KindMethod, Name: "save", File: "a.ts"}
	nodes := []model.Node{start, real}
	edges := []model.Edge{{ID: "ok", FromID: start.ID, ToID: real.ID, Kind: model.RelCalls, Confidence: 0.92}}
	for i := 0; i < 60; i++ {
		phantom := model.Node{ID: "method:ts:lib.ts:Math.floor." + strconv.Itoa(i), Kind: model.KindMethod, Name: "floor", File: "lib.ts"}
		nodes = append(nodes, phantom)
		edges = append([]model.Edge{{
			ID: "u" + strconv.Itoa(i), FromID: start.ID, ToID: phantom.ID, Kind: model.RelCalls,
			Confidence: 0.25, Unresolved: true,
		}}, edges...)
	}
	g := model.Graph{Nodes: nodes, Edges: edges}

	v := projectFlow(g, GraphViewRequest{Lens: "flow", Sel: start.ID})
	ids := map[string]bool{}
	for _, n := range v.Nodes {
		ids[n.ID] = true
	}
	if !ids[real.ID] {
		t.Fatalf("resolved 0.92 call must appear, truncated=%v nodes=%d", v.Truncated, len(ids))
	}
	for id := range ids {
		if strings.Contains(id, "Math.floor") {
			t.Fatalf("unresolved library call leaked onto default Flow: %s", id)
		}
	}
	if v.Truncated {
		t.Fatal("unresolved calls must not consume MaxNodes")
	}

	shown := projectFlow(g, GraphViewRequest{Lens: "flow", Sel: start.ID, Overlays: []string{"unresolved"}})
	found := false
	for _, n := range shown.Nodes {
		if strings.Contains(n.ID, "Math.floor") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("unresolved overlay should bring phantom calls back")
	}
}

func TestProjectFocusDoesNotExpandNeighborInjects(t *testing.T) {
	port := model.Node{ID: "type:ts:p.ts:P", Kind: model.KindType, Name: "P", File: "p.ts", Abstract: true}
	sql := model.Node{ID: "type:ts:s.ts:SqliteP", Kind: model.KindType, Name: "SqliteP", File: "s.ts"}
	other := model.Node{ID: "type:ts:q.ts:Q", Kind: model.KindType, Name: "Q", File: "q.ts", Abstract: true}
	nodes := []model.Node{port, sql, other}
	edges := []model.Edge{{ID: "impl", FromID: sql.ID, ToID: port.ID, Kind: model.RelImplements}}
	for i := 0; i < 8; i++ {
		uc := model.Node{ID: "type:ts:uc.ts:UC" + strconv.Itoa(i), Kind: model.KindType, Name: "UC" + strconv.Itoa(i), File: "uc.ts"}
		nodes = append(nodes, uc)
		edges = append(edges,
			model.Edge{ID: "inj" + strconv.Itoa(i), FromID: uc.ID, ToID: port.ID, Kind: model.RelInjects},
			model.Edge{ID: "sib" + strconv.Itoa(i), FromID: uc.ID, ToID: other.ID, Kind: model.RelInjects},
		)
	}
	g := model.Graph{Nodes: nodes, Edges: edges}
	v := projectFocus(g, GraphViewRequest{Lens: "focus", Sel: port.ID})
	ids := map[string]bool{}
	for _, n := range v.Nodes {
		ids[n.ID] = true
	}
	if !ids[sql.ID] {
		t.Fatal("expected sqlite implementer")
	}
	for i := 0; i < 8; i++ {
		if !ids["type:ts:uc.ts:UC"+strconv.Itoa(i)] {
			t.Fatalf("expected consumer UC%d", i)
		}
	}
	if ids[other.ID] {
		t.Fatalf("must not pull sibling-port injects, nodes=%v", ids)
	}
}

func TestProjectFlowAttachesMethodImplements(t *testing.T) {
	ep := model.Node{ID: "endpoint:ts:r.ts:POST:/clock-in", Kind: model.KindEndpoint, Name: "POST /clock-in", File: "r.ts"}
	handle := model.Node{ID: "method:ts:uc.ts:ClockIn.handle", Kind: model.KindMethod, Name: "handle", File: "uc.ts"}
	portM := model.Node{ID: "method:ts:p.ts:Repo.save", Kind: model.KindMethod, Name: "save", File: "p.ts"}
	sqlM := model.Node{ID: "method:ts:s.ts:SqliteRepo.save", Kind: model.KindMethod, Name: "save", File: "s.ts"}
	g := model.Graph{Nodes: []model.Node{ep, handle, portM, sqlM}, Edges: []model.Edge{
		{ID: "h", FromID: ep.ID, ToID: handle.ID, Kind: model.RelHandles, Confidence: 0.92},
		{ID: "c", FromID: handle.ID, ToID: portM.ID, Kind: model.RelCalls, Confidence: 0.92},
		{ID: "im", FromID: sqlM.ID, ToID: portM.ID, Kind: model.RelImplements, Confidence: 0.92},
	}}
	v := projectFlow(g, GraphViewRequest{Lens: "flow", Sel: ep.ID})
	ids := map[string]bool{}
	for _, n := range v.Nodes {
		ids[n.ID] = true
	}
	if !ids[sqlM.ID] {
		t.Fatalf("expected adapter method via implements, nodes=%v", ids)
	}
}

func TestSelfLoopAwaitsDroppedFromFlow(t *testing.T) {
	n := model.Node{ID: "type:ts:a.ts:Foo", Kind: model.KindType, Name: "Foo", File: "a.ts"}
	g := model.Graph{Nodes: []model.Node{n}, Edges: []model.Edge{
		{ID: "a", FromID: n.ID, ToID: n.ID, Kind: model.RelAwaits},
	}}
	v := projectFlow(g, GraphViewRequest{Lens: "flow", Sel: n.ID, Overlays: []string{"async"}})
	if len(v.Edges) != 0 {
		t.Fatalf("self-loop awaits should not appear: %#v", v.Edges)
	}
}
func TestEntityDetailJSONUsesFromTo(t *testing.T) {
	file := model.Node{ID: "file:ts:a.ts", Kind: model.KindFile, Name: "a.ts", File: "a.ts"}
	typ := model.Node{ID: "type:ts:a.ts:Foo", Kind: model.KindType, Name: "Foo", File: "a.ts"}
	g := model.Graph{
		Nodes: []model.Node{file, typ},
		Edges: []model.Edge{{ID: "1", FromID: file.ID, ToID: typ.ID, Kind: model.RelContains, Source: model.SrcHeuristic}},
	}
	d := entityDetail(g, file.ID)
	if len(d.Outgoing) != 1 || d.Outgoing[0].To != typ.ID || d.Outgoing[0].From != file.ID {
		t.Fatalf("%#v", d.Outgoing)
	}
}
