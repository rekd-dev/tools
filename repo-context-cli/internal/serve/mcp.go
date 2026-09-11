package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"repo-context-cli/internal/fitness"
	"repo-context-cli/internal/query"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunMCP serves the inventory graph over MCP stdio until the client disconnects.
func RunMCP(opts Options) error {
	hub := &graphHub{dbPath: opts.DBPath, repoPath: opts.RepoPath}
	api := &mcpAPI{hub: hub, opts: opts}
	server := mcp.NewServer(&mcp.Implementation{Name: "repo-context", Version: "2.2.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search",
		Description: "Search types, endpoints, files, and methods in the inventoried graph.",
	}, api.search)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "entity",
		Description: "Show a graph node plus incoming/outgoing relations, implementers, and DI bindings.",
	}, api.entity)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "view",
		Description: "Architecture drill-down boxes for a folder path (empty path = repo root apps).",
	}, api.view)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "graph_view",
		Description: "Focus or Flow projection. Flow overlays: data, async, deps.",
	}, api.graphView)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "fitness",
		Description: "Clean Architecture findings: cycles, layer violations, domain purity.",
	}, api.fitnessReport)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "meta",
		Description: "Repo identity, git SHA, and analyzer coverage.",
	}, api.meta)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "path",
		Description: "Bounded static path between graph node ids.",
	}, api.path)
	return server.Run(context.Background(), &mcp.StdioTransport{})
}

type mcpAPI struct {
	hub  *graphHub
	opts Options
}

type searchArgs struct {
	Query string `json:"query" jsonschema:"Search text"`
	Limit int    `json:"limit,omitempty" jsonschema:"Max hits, default 20"`
}

type idArgs struct {
	ID string `json:"id" jsonschema:"Graph node or edge id"`
}

type viewArgs struct {
	Path string `json:"path,omitempty" jsonschema:"Architecture folder path, empty for repo root"`
}

type graphViewArgs struct {
	Lens     string `json:"lens" jsonschema:"focus or flow"`
	Path     string `json:"path,omitempty"`
	Sel      string `json:"sel,omitempty" jsonschema:"Selected node id"`
	Root     string `json:"root,omitempty" jsonschema:"File root to seed Focus/Flow"`
	Overlays string `json:"overlays,omitempty" jsonschema:"Comma list: data,async,deps"`
}

type fitnessArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"Max findings to return, default 30"`
}

type pathArgs struct {
	From     string `json:"from" jsonschema:"Start node id"`
	To       string `json:"to,omitempty" jsonschema:"Optional end node id"`
	MaxDepth int    `json:"maxDepth,omitempty"`
	MaxNodes int    `json:"maxNodes,omitempty"`
	Kinds    string `json:"kinds,omitempty" jsonschema:"Comma-separated edge kinds"`
}

type emptyArgs struct{}

func (a *mcpAPI) search(_ context.Context, _ *mcp.CallToolRequest, args searchArgs) (*mcp.CallToolResult, any, error) {
	g, err := a.hub.facts()
	if err != nil {
		return nil, nil, err
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 20
	}
	return jsonTool(query.Search(g, args.Query, limit))
}

func (a *mcpAPI) entity(_ context.Context, _ *mcp.CallToolRequest, args idArgs) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.ID) == "" {
		return nil, nil, fmt.Errorf("id is required")
	}
	g, err := a.hub.facts()
	if err != nil {
		return nil, nil, err
	}
	return jsonTool(entityDetail(g, args.ID))
}

func (a *mcpAPI) view(_ context.Context, _ *mcp.CallToolRequest, args viewArgs) (*mcp.CallToolResult, any, error) {
	g, err := a.hub.graph()
	if err != nil {
		return nil, nil, err
	}
	return jsonTool(projectView(g, args.Path))
}

func (a *mcpAPI) graphView(_ context.Context, _ *mcp.CallToolRequest, args graphViewArgs) (*mcp.CallToolResult, any, error) {
	g, err := a.hub.facts()
	if err != nil {
		return nil, nil, err
	}
	lens := strings.ToLower(strings.TrimSpace(args.Lens))
	if lens == "" {
		lens = "focus"
	}
	return jsonTool(projectGraphView(g, GraphViewRequest{
		Lens:     lens,
		Scope:    args.Path,
		Sel:      args.Sel,
		Root:     args.Root,
		Overlays: strings.Split(args.Overlays, ","),
	}))
}

func (a *mcpAPI) fitnessReport(_ context.Context, _ *mcp.CallToolRequest, args fitnessArgs) (*mcp.CallToolResult, any, error) {
	g, err := a.hub.graph()
	if err != nil {
		return nil, nil, err
	}
	rulesPath := a.opts.RulesPath
	if rulesPath == "" {
		rulesPath = fitness.FindRulesFile(a.opts.RepoPath)
	}
	rules, src, _ := fitness.LoadRules(rulesPath)
	meta := readMeta(a.opts.DBPath)
	report := fitness.Analyze(meta["repo"], a.opts.RepoPath, meta["gitSha"], g.Modules, g.Deps, rules, src)
	limit := args.Limit
	if limit <= 0 {
		limit = 30
	}
	out := map[string]any{
		"repo":        report.Repo,
		"gitSha":      report.GitSha,
		"rulesSource": report.Rules,
		"summary":     report.Summary,
	}
	findings := report.Findings
	if len(findings) > limit {
		out["findings"] = findings[:limit]
		out["truncated"] = true
		out["findingCount"] = len(report.Findings)
	} else {
		out["findings"] = findings
	}
	return jsonTool(out)
}

func (a *mcpAPI) meta(_ context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
	g, err := a.hub.facts()
	if err != nil {
		return nil, nil, err
	}
	m := readMeta(a.opts.DBPath)
	return jsonTool(map[string]any{
		"repo":     m["repo"],
		"path":     m["path"],
		"gitSha":   m["gitSha"],
		"coverage": g.Coverage,
	})
}

func (a *mcpAPI) path(_ context.Context, _ *mcp.CallToolRequest, args pathArgs) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.From) == "" {
		return nil, nil, fmt.Errorf("from is required")
	}
	g, err := a.hub.facts()
	if err != nil {
		return nil, nil, err
	}
	return jsonTool(query.BoundedPath(g, query.PathRequest{
		From: args.From, To: args.To, MaxDepth: args.MaxDepth, MaxNodes: args.MaxNodes,
		Kinds: query.ParseKinds(args.Kinds),
	}))
}

func jsonTool(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, v, nil
}
