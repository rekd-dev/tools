package serve

import (
	"context"
	"strings"
	"testing"

	"repo-context-cli/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPSearchEntityAndFlow(t *testing.T) {
	uc := model.Node{ID: "type:ts:uc.ts:ClockInUseCase", Kind: model.KindType, Name: "ClockInUseCase", File: "uc.ts"}
	clock := model.Node{ID: "type:ts:c.ts:Clock", Kind: model.KindType, Name: "Clock", File: "c.ts"}
	g := model.Graph{
		Nodes: []model.Node{uc, clock},
		Edges: []model.Edge{{ID: "i", FromID: uc.ID, ToID: clock.ID, Kind: model.RelInjects}},
	}
	api := &mcpAPI{hub: &graphHub{cachedFacts: &g}}

	res, _, err := api.search(context.Background(), nil, searchArgs{Query: "ClockIn", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, res)
	if !strings.Contains(text, "ClockInUseCase") {
		t.Fatalf("search missed use case: %s", text)
	}

	_, detail, err := api.entity(context.Background(), nil, idArgs{ID: uc.ID})
	if err != nil {
		t.Fatal(err)
	}
	ent := detail.(EntityDetail)
	if ent.Node.Name != "ClockInUseCase" || len(ent.Outgoing) != 1 {
		t.Fatalf("%#v", ent)
	}

	_, gv, err := api.graphView(context.Background(), nil, graphViewArgs{Lens: "flow", Sel: uc.ID, Overlays: "deps"})
	if err != nil {
		t.Fatal(err)
	}
	view := gv.(GraphView)
	if view.Selection != uc.ID {
		t.Fatalf("selection=%s", view.Selection)
	}
	ids := map[string]bool{}
	for _, n := range view.Nodes {
		ids[n.ID] = true
	}
	if !ids[clock.ID] {
		t.Fatalf("deps overlay should include Clock, nodes=%v", ids)
	}
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatal("empty tool result")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content %T", res.Content[0])
	}
	return tc.Text
}
