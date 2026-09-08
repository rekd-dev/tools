# tools

Local-first architecture tooling.

## Contents

### `config/`

Global agent configuration loaded by all projects via `CLAUDE.md`.

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

**Subcommands:** `init`, `inventory`, `query`, `fitness`, `serve`, `index`

`inventory --semantic=auto` (default) runs TypeScript/Roslyn sidecars when they are installed; `init --protect` is only for analysis-only clones (appends `*` to `.git/info/exclude`).

Builds to `bin/repo-context.exe`:

```bash
# Windows
cd repo-context-cli
.\build.ps1
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
```

Deferred research features: [docs/backlog.md](docs/backlog.md).

### `safe-log-helper/`

Google Apps Script macro (`macro.gs.gs`) for reviewing a Google Sheet log. Setup notes in `info.txt`.

## Agents

Specialized agents live in `~/.claude/agents/`:

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
| `safe-log-helper/*.json` | Google service account credentials |
| `config/.claude/` | Machine-specific Claude permission settings |
