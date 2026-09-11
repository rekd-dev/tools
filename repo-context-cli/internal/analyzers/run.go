package analyzers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"repo-context-cli/internal/analyzers/heuristic"
	"repo-context-cli/internal/model"
	"repo-context-cli/internal/store"
)

type Options struct {
	RepoPath string
	DBPath   string
	Semantic string // auto | required | off
	Inv      heuristic.Inventory
}

type ndjsonRec struct {
	V          int               `json:"v"`
	Kind       string            `json:"kind"`
	ID         string            `json:"id"`
	NodeKind   string            `json:"nodeKind"`
	Name       string            `json:"name"`
	Qualified  string            `json:"qualifiedName"`
	Language   string            `json:"language"`
	File       string            `json:"file"`
	Line       int               `json:"line"`
	Column     int               `json:"column"`
	Abstract   bool              `json:"abstract"`
	Extra      map[string]string `json:"extra"`
	From       string            `json:"from"`
	To         string            `json:"to"`
	EdgeKind   string            `json:"edgeKind"`
	Source     string            `json:"source"`
	Confidence float64           `json:"confidence"`
	Analyzer   string            `json:"analyzer"`
	Detail     string            `json:"detail"`
	Unresolved bool              `json:"unresolved"`
	Status     string            `json:"status"`
	Message    string            `json:"message"`
}

func Populate(opts Options) (model.Graph, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.Semantic))
	if mode == "" {
		mode = "auto"
	}
	opts.Inv.RepoRoot = opts.RepoPath
	parts := []model.Graph{heuristic.Lift(opts.Inv)}

	needTS := hasLang(opts.Inv, "typescript")
	needCS := hasLang(opts.Inv, "csharp")

	if mode != "off" {
		if needTS {
			g, err := runSidecar("typescript", opts.RepoPath)
			if err != nil {
				cov := model.Graph{Coverage: []model.Coverage{{Analyzer: "typescript", Status: statusFor(err), Message: err.Error()}}}
				if mode == "required" {
					return model.Graph{}, fmt.Errorf("typescript analyzer: %w", err)
				}
				parts = append(parts, cov)
			} else {
				parts = append(parts, g)
			}
		} else {
			parts = append(parts, model.Graph{Coverage: []model.Coverage{{Analyzer: "typescript", Status: "skipped", Message: "no TypeScript files"}}})
		}
		if needCS {
			g, err := runSidecar("roslyn", opts.RepoPath)
			if err != nil {
				if mode == "required" {
					return model.Graph{}, fmt.Errorf("roslyn analyzer: %w", err)
				}
				parts = append(parts, model.Graph{Coverage: []model.Coverage{{Analyzer: "roslyn", Status: statusFor(err), Message: err.Error()}}})
			} else {
				parts = append(parts, g)
			}
		} else {
			parts = append(parts, model.Graph{Coverage: []model.Coverage{{Analyzer: "roslyn", Status: "skipped", Message: "no C# files"}}})
		}
	} else {
		parts = append(parts, model.Graph{Coverage: []model.Coverage{
			{Analyzer: "typescript", Status: "skipped", Message: "semantic=off"},
			{Analyzer: "roslyn", Status: "skipped", Message: "semantic=off"},
		}})
	}

	g := Merge(parts...)
	db, err := store.Open(opts.DBPath)
	if err != nil {
		return g, err
	}
	defer db.Close()
	if err := store.ReplaceGraph(db, g); err != nil {
		return g, err
	}
	return g, nil
}

func hasLang(inv heuristic.Inventory, lang string) bool {
	for _, m := range inv.Modules {
		if m.Language == lang {
			return true
		}
	}
	return false
}

func statusFor(err error) string {
	if err == nil {
		return "ran"
	}
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "missing") {
		return "missing"
	}
	return "failed"
}

func runSidecar(name, repo string) (model.Graph, error) {
	root := findAnalyzersRoot()
	var cmd *exec.Cmd
	switch name {
	case "typescript":
		script := filepath.Join(root, "typescript", "analyze.mjs")
		if _, err := os.Stat(script); err != nil {
			return model.Graph{}, fmt.Errorf("typescript analyzer missing at %s", script)
		}
		if _, err := exec.LookPath("node"); err != nil {
			return model.Graph{}, fmt.Errorf("node not found")
		}
		cmd = exec.Command("node", script, repo)
		cmd.Dir = filepath.Join(root, "typescript")
	case "roslyn":
		proj := filepath.Join(root, "roslyn", "Analyzer.csproj")
		if _, err := os.Stat(proj); err != nil {
			return model.Graph{}, fmt.Errorf("roslyn analyzer missing at %s", proj)
		}
		if _, err := exec.LookPath("dotnet"); err != nil {
			return model.Graph{}, fmt.Errorf("dotnet not found")
		}
		cmd = exec.Command("dotnet", "run", "--project", proj, "--", repo)
		cmd.Dir = filepath.Join(root, "roslyn")
		cmd.Env = append(os.Environ(), "NUGET_PLUGIN_HANDSHAKE_TIMEOUT_IN_SECONDS=5")
	default:
		return model.Graph{}, fmt.Errorf("unknown analyzer %s", name)
	}
	cmd.Stderr = os.Stderr
	out, err := cmdStdout(cmd, 15*time.Minute)
	if err != nil {
		return model.Graph{}, err
	}
	return parseNDJSON(out, name)
}

func cmdStdout(cmd *exec.Cmd, timeout time.Duration) ([]byte, error) {
	done := make(chan error, 1)
	var out []byte
	go func() {
		var err error
		out, err = cmd.Output()
		done <- err
	}()
	select {
	case err := <-done:
		return out, err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("analyzer timed out")
	}
}

func parseNDJSON(data []byte, analyzer string) (model.Graph, error) {
	g := model.Graph{}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec ndjsonRec
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		switch rec.Kind {
		case "coverage":
			g.Coverage = append(g.Coverage, model.Coverage{Analyzer: analyzer, Status: rec.Status, Message: rec.Message})
		case "node":
			g.Nodes = append(g.Nodes, model.Node{
				ID: rec.ID, Kind: rec.NodeKind, Name: rec.Name, QualifiedName: rec.Qualified,
				Language: rec.Language, File: rec.File, Line: rec.Line, Column: rec.Column,
				Abstract: rec.Abstract, Extra: rec.Extra,
			})
		case "edge":
			conf := rec.Confidence
			if conf == 0 {
				conf = model.ConfidenceCompiler
			}
			g.Edges = append(g.Edges, model.Edge{
				ID: rec.ID, FromID: rec.From, ToID: rec.To, Kind: rec.EdgeKind,
				Source: rec.Source, Confidence: conf, Analyzer: analyzer,
				File: rec.File, Line: rec.Line, Column: rec.Column, Detail: rec.Detail,
				Unresolved: rec.Unresolved,
			})
		}
	}
	if len(g.Coverage) == 0 {
		g.Coverage = []model.Coverage{{Analyzer: analyzer, Status: "ran"}}
	}
	return g, sc.Err()
}

func findAnalyzersRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		cand := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "analyzers"))
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand
		}
	}
	if exe, err := os.Executable(); err == nil {
		for _, cand := range []string{
			filepath.Join(filepath.Dir(exe), "analyzers"),
			filepath.Join(filepath.Dir(exe), "..", "repo-context-cli", "analyzers"),
		} {
			if st, err := os.Stat(cand); err == nil && st.IsDir() {
				return cand
			}
		}
	}
	return filepath.Join("analyzers")
}
