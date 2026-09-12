# tools

Local-first architecture tooling: a Go CLI that inventories a repository into SQLite, a React viewer for that graph, and shared agent config.

Forks and private use are welcome under [MIT](LICENSE). I do not take unsolicited pull requests; see [CONTRIBUTING.md](CONTRIBUTING.md).

## Contents

### `config/`

Agent instructions loaded by this repo’s [`CLAUDE.md`](CLAUDE.md). To reuse them in another project, `@`-import the files from this `config/` directory (absolute path on your machine).

| File | Purpose |
|---|---|
| `system.md` | Agent identity, tone, and core decision-making rules |
| `skills.md` | Shared skill workflows (repo analysis, summarization, git history, etc.) |
| `prompts.md` | Reusable prompt templates for common tasks |
| `indexing.md` | What to index, what to skip, search priority rules |
| `agent-build-guidelines.md` | Architecture and language defaults for building new components |
| `github.md` | GitHub defaults: repo initialization, atomic commits, pre-commit gate |

### `repo-context-cli/`

Go CLI that pre-computes repository analysis signals into a SQLite database so AI agents can understand a codebase without reading thousands of source files.

**Subcommands:** `init`, `inventory`, `query`, `fitness`, `serve`, `mcp`, `index`

`inventory --semantic=auto` (default) runs TypeScript/Roslyn sidecars when they are installed; `init --protect` is only for analysis-only clones (appends `*` to `.git/info/exclude`).

Builds to `bin/repo-context` (or `repo-context.exe` on Windows):

```bash
cd repo-context-cli
./build.ps1    # Windows
./build.sh     # Linux / macOS
```

See [repo-context-cli/ReadMe.md](repo-context-cli/ReadMe.md) for full documentation.

### `arch-view/`

React + TypeScript SPA that visualizes the graph from `repo-context serve`. Lenses: **Architecture** (layered drill-down), **Focus** (interface resolution / DI neighborhood), **Flow** (bounded path with data/async overlays), plus a fitness overlay.

```bash
cd arch-view
npm install
npm run dev          # proxies /api to http://127.0.0.1:8787
# with backend:
#   repo-context inventory <repo> --force --semantic=auto
#   repo-context serve <repo>
#   repo-context mcp <repo>     # stdio MCP for agents
```

Deferred research features: [docs/backlog.md](docs/backlog.md).

## Agents

Specialized agents live in `~/.claude/agents/` (and Cursor’s agent list):

| Agent | Purpose |
|---|---|
| `pre-commit` | Quality gate: build check, test check, doc update before each commit |
| `repo-scanner` | Repository structure analysis using `repo-context` CLI |
| `flow-mapper` | Execution and data flow mapping |
| `feature-synthesizer` | User-observable feature extraction |
| `gap-detector` | Missing implementations and dead code detection |
| `interrogation` | Generates decisions for blocking gaps |
| `decision-recorder` | Converts user answers into structured decisions |
| `planner` | Generates implementation plans from analysis output |
| `code-analysis` | Structured code analysis, returns JSON |

## Excluded from this repo

| Path | Reason |
|---|---|
| `bin/` | Compiled binaries |
| `data/` | SQLite databases |
| `indexer/` | Separate project with its own git history |
| `.cursor/` | Local editor plans |
| `.claude/` | Machine-specific Claude permission settings |
