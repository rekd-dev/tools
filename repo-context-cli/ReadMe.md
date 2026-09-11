# repo-context

Pre-computes repository analysis signals so AI agents can understand a codebase without reading thousands of source files — reducing token consumption significantly on architecture analysis, code review, and root-cause investigation tasks.

## How it works

1. `init` — creates `.context/` (optional `--protect` for analysis-only clones)
2. `inventory` — deep signal scan plus an **evidence-aware graph** (heuristic always; TypeScript compiler / Roslyn when `--semantic=auto|required`); writes `inventory.db` + `inventory-toc.json` to `.context/`
3. `query` — SQL-powered query interface for the inventory database (raw SQL or predefined section aliases)
4. `fitness` — architecture fitness reporter (cycles, Clean Architecture layer violations, domain purity); JSON contract for CI + the viewer
5. `serve` — local HTTP API over `inventory.db` for the separate `arch-view` SPA
6. `mcp` — MCP stdio server over the same graph (search, entity, view, graph_view, fitness, path, meta)
7. `index` — shows current vs stale status across all repos at a glance

An agent can use `repo-context query --sql "..."` for precise lookups, `--section <name>` for broad overviews, or load `inventory-toc.json` (~1–3KB) for a lightweight summary.

## Installation

```bash
# Windows
.\build.ps1

# Linux / macOS
./build.sh
```

Builds to `../bin/repo-context` (or `repo-context.exe` on Windows).

**Dependencies:** `modernc.org/sqlite` (pure Go, no CGO required).

## Subcommands

### `init <path>`

Creates `.context/` for git repos in `<path>`. Working repos should not pass `--protect`.

```bash
repo-context init ./repos
repo-context init ./clone --protect   # analysis-only clones: exclude all files from git
```

**Flags:**
- `--force` — re-initialize even if already done
- `--protect` — append `*` to `.git/info/exclude` (analysis-only clones only)

### `inventory <path>`

Performs a deep signal scan and writes `inventory.db` (SQLite) + `inventory-toc.json` to each repo's `.context/` directory.

```bash
repo-context inventory ./repos
repo-context inventory ./repos/MyRepo --format table
repo-context inventory ./repos --refresh
repo-context inventory ./repos --pull
```

**Flags:**
- `--format string` — `json` (default, prints summary), `table` (detailed summary), `agent` (compact one-liner)
- `--refresh` — only re-scan files changed since last inventory (`git diff --name-only`)
- `--force` — full rescan even if inventory is current (SHA matches)
- `--pull` — run `git pull` before scanning (explicit opt-in only)
- `--semantic` — `auto` (default: run TypeScript/Roslyn sidecars when present, else heuristic), `required` (fail if a needed sidecar is missing), `off` (heuristic graph only)

**Default behaviour:** if `inventory.db` exists and the git SHA matches current HEAD, prints a "inventory is current" message and exits. Pass `--force` to override.

**Stack detection** — a repo can match multiple stacks (e.g., `dotnet-webapi` + `angular`). All matches are stored in the `stacks` array.

| Stack              | Indicator                                             |
| ------------------ | ----------------------------------------------------- |
| `dotnet-webapi`    | `.sln` + `.csproj` referencing `Microsoft.AspNetCore` |
| `dotnet-functions` | `.sln` + `.csproj` referencing Azure Functions        |
| `biztalk`          | `.sln` + `.btproj` or `.odx` files                    |
| `ssis`             | `.sln` + `.dtsx` files                                |
| `dotnet`           | `.sln` fallback                                       |
| `go`               | `go.mod`                                              |
| `angular`          | `package.json` + `@angular/core`                      |
| `react`            | `package.json` + `react`                              |
| `nextjs`           | `package.json` + `next` (always paired with `react`)  |
| `node`             | `package.json` fallback                               |
| `python`           | `pyproject.toml` or `requirements.txt`                |
| `rust`             | `Cargo.toml`                                          |
| `unknown`          | nothing matched                                       |

**Signal categories** scanned (case-insensitive, all source files):

| Category       | Patterns                                                                                                                       |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| orchestration  | `Orchestrat`, `Pipeline`, `Workflow`, `Coordinator`                                                                            |
| business-logic | `Validator`, `Resolver`, `Calculator`, `Processor`, `StateMachine`                                                             |
| handler        | `Handler`, `Dispatcher`, `Mediator`                                                                                            |
| integration    | `Repository`, `EventPublisher`, `EventSubscriber`, `MessageBus`, `ServiceBus`, `EventHub`                                      |
| resilience     | `retry`, `backoff`, `Polly`, `CircuitBreaker`, `exponential`                                                                   |
| async-csharp   | `async Task`, `await `, `ContinueWith`, `WhenAll`, `WhenAny`                                                                   |
| async-js       | `async `, `await `, `Observable`, `switchMap`, `mergeMap`, `combineLatest`                                                     |
| logging        | `ILogger`, `LoggingService`, `ErrorHandler`, `appInsights`, telemetry, console, `Serilog`, `NLog`, `Winston`, `Bunyan`, `Pino` |
| react-hooks    | `useState`, `useEffect`, `useCallback`, `useMemo`, `useReducer`, `useContext`                                                  |

**Symbol extraction** (regex-based, per-language):

| Language           | Extracted                                                                                                        |
| ------------------ | ---------------------------------------------------------------------------------------------------------------- |
| C#                 | Classes, interfaces, structs, enums, records, namespaces, DI registrations, endpoint signatures, auth attributes |
| TypeScript/Angular | Classes, interfaces, components, injectables, exported functions                                                 |
| TSX/JSX (React)    | Components (PascalCase exported functions and consts), classes, interfaces                                        |
| Go                 | Structs, interfaces, exported functions, method receivers                                                        |

**Event flow detection** (pub/sub topology):

| Mechanism     | C#                                                  | TypeScript                           |
| ------------- | --------------------------------------------------- | ------------------------------------ |
| MediatR       | Publish/Send, INotificationHandler, IRequestHandler | —                                    |
| ServiceBus    | SendMessageAsync, ProcessMessageAsync               | —                                    |
| EventHub      | SendAsync, ProcessEventAsync                        | —                                    |
| Domain Events | AddDomainEvent, IDomainEvent                        | —                                    |
| SignalR       | Clients.SendAsync, IHubContext                      | .on(), .invoke(), .send()            |
| WebSocket     | —                                                   | new WebSocket, .onmessage, .send     |
| EventEmitter  | —                                                   | @Output() EventEmitter               |
| NgRx          | —                                                   | store.dispatch, createEffect, ofType |

### `query <path>`

Queries the `inventory.db` database with raw SQL or predefined section aliases. The primary agent-facing interface.

```bash
# Raw SQL — maximum flexibility
repo-context query ./repos/MyRepo --sql "SELECT * FROM types WHERE kind = 'interface'"
repo-context query ./repos/MyRepo --sql "SELECT t.name, t.file FROM types t JOIN type_implements ti ON t.name = ti.type_name WHERE ti.interface = 'IOrderService'"
repo-context query ./repos/MyRepo --sql "SELECT * FROM event_flows WHERE event_type = 'OrderUpdated' ORDER BY direction"

# Predefined section aliases
repo-context query ./repos/MyRepo --section structure
repo-context query ./repos/MyRepo --section signals --min-score 2
repo-context query ./repos/MyRepo --section types
repo-context query ./repos/MyRepo --section di
repo-context query ./repos/MyRepo --section endpoints
repo-context query ./repos/MyRepo --section events
```

**Flags:**
- `--sql string` — raw SQL query against `inventory.db`
- `--section string` — predefined section alias (see below)
- `--min-score int` — minimum complexity score, signals section only (default 3)
- `--format string` — `json` (default), `table`, `csv`

**Sections:**
- `structure` — metadata, projects, project dependencies, external deps, entry points, file counts
- `signals` — complexity signals filtered by `--min-score`
- `routes` — route patterns (HTTP endpoints, function triggers)
- `messaging` — messaging signals (ServiceBus, EventHub, SignalR)
- `triggers` — function trigger signals
- `types` — type symbols with implements relationships
- `di` — dependency injection mappings
- `endpoints` — detailed endpoint signatures (route, method, handler, auth)
- `events` — event flow topology (publish/subscribe/handle across stacks)
- `modules` — architecture module graph nodes (TS/C# files + layer)
- `module-deps` — module dependency edges (direct / external)
- `all` — all sections

### `fitness <path>`

Architecture fitness reporter. Builds or reads the module graph, applies Clean Architecture conventions (optional `arch-rules.json`), and emits the **viewer/CI JSON contract**.

```bash
repo-context fitness ./repos/MyRepo --format table
repo-context fitness ./repos/MyRepo --rules ./arch-rules.json --fail-on error
repo-context fitness ./testdata/ts-ca --rescan --format json
repo-context fitness ./testdata/ts-ca-bad --rescan --format json
```

**Checks:**
1. **Cycles** — module dependency cycles (error)
2. **Layer violations** — disallowed edges between `domain` / `application` / `infrastructure` / `interface` / `composition` / `web` (error)
3. **Domain purity** — forbidden framework packages in `domain` (error) and `application` (warning)
4. **Unlayered** — files that matched no layer (warning; silence via `ignore` globs)

**Flags:**
- `--format` — `json` (default) or `table`
- `--rules` — path to `arch-rules.json` (else `<repo>/arch-rules.json` if present, else built-in defaults)
- `--fail-on` — exit `1` if any finding ≥ this severity (`error` default)
- `--rescan` — rebuild module graph from source instead of `inventory.db`

Example `arch-rules.json`: see `testdata/arch-rules.example.json`.

**JSON contract (stdout):**

```json
{
  "repo": "MyRepo",
  "path": "...",
  "gitSha": "abc1234",
  "rulesSource": "default",
  "summary": { "modules": 0, "deps": 0, "errors": 0, "warnings": 0, "cycles": 0, "layerViolations": 0, "domainPurity": 0, "unlayered": 0 },
  "findings": [{ "id": "cycle-1", "kind": "cycle", "severity": "error", "message": "...", "cycle": "a->b->a" }],
  "modules": [{ "id": "...", "path": "...", "language": "typescript", "layer": "domain", "abstract": false }],
  "cycles": ["a->b->a"]
}
```

### `serve <path>`

Local HTTP API for the separate `arch-view` React SPA.

```bash
repo-context serve ./repos/MyRepo --addr 127.0.0.1:8787
repo-context serve ./repos/MyRepo --static ../arch-view/dist
repo-context serve ./repos/MyRepo --rules ./arch-rules.json
```

**Flags:** `--addr` (default `127.0.0.1:8787`), `--static` (optional `arch-view` `dist/`), `--rules` (optional `arch-rules.json`).

**Endpoints:** `/api/health`, `/api/meta`, `/api/modules`, `/api/module-deps`, `/api/fitness`, `/api/view?path=`, `/api/source?file=`, `/api/search?q=`, `/api/entity?id=`, `/api/edge?id=`, `/api/path?from=&to=&maxDepth=&maxNodes=&kinds=`, `/api/graph-view?lens=architecture|focus|flow&path=&sel=&overlays=`, `/api/coverage`, `/api/churn?path=`

`/api/graph-view` lenses: `architecture` (module drill-down still uses `/api/view`), `focus` (neighborhood around `sel`), `flow` (bounded path from `sel`). Flow overlays: `data` (persistence writes/reads), `async` (awaits/forks), `deps` (constructor injects that the default spine hides). `/api/churn` maps 90 days of `git log` onto the current architecture boxes. The SPA hash restores `lens`, `path`, `sel`, overlays (`data`, `async`, `deps`, `churn`), and `view=matrix` for the architecture DSM.

### `mcp <path>`

MCP stdio server over `inventory.db`. Same projections as `serve`, for agent clients.

```bash
repo-context mcp ./repos/MyRepo
repo-context mcp ./repos/MyRepo --rules ./arch-rules.json
```

**Tools:** `search`, `entity`, `view`, `graph_view` (`focus` or `flow`; overlays `data,async,deps`), `fitness`, `path`, `meta`.

Example Cursor MCP config:

```json
{
  "mcpServers": {
    "repo-context": {
      "command": "R:\\Projects\\tools\\bin\\repo-context.exe",
      "args": ["mcp", "/path/to/your-repo"]
    }
  }
}
```

### `index <path>`

Shows the status of all git repos found inside `<path>`.

```bash
repo-context index ./repos
repo-context index ./repos --format json
repo-context index ./repos --format agent
```

**Flags:**
- `--format string` — `table` (default), `json`, `agent` (compact pipe-delimited)

**Status values:**
- `current` — `inventory.db` exists and SHA matches `git rev-parse --short HEAD`
- `STALE` — context dir exists but inventory is missing or SHA has moved
- `STALE (v1 json)` — legacy `inventory.json` found (needs re-scan with v2.0)
- `NOT INITIALIZED` — no `.context/` directory

### `version` / `help`

```bash
repo-context version
repo-context help
```

## Database schema

`inventory.db` is a SQLite database with the following tables:

| Table                | Description                                                   |
| -------------------- | ------------------------------------------------------------- |
| `metadata`           | Key-value pairs: repo, path, scannedAt, gitSha, stack, stacks |
| `projects`           | Project files (name, path, framework)                         |
| `project_deps`       | Inter-project dependency edges                                |
| `external_deps`      | External package dependencies                                 |
| `entry_points`       | Application entry point file paths                            |
| `file_counts`        | Source file counts by directory and extension                 |
| `complexity_signals` | Files with complexity scores (1-5) and LOC                    |
| `signal_matches`     | Individual signal pattern matches per file                    |
| `routes`             | Route pattern matches with counts                             |
| `messaging_signals`  | Messaging SDK usage signals                                   |
| `trigger_signals`    | Azure Function trigger signals                                |
| `types`              | Type declarations (class, interface, struct, enum, component) |
| `type_implements`    | Interface implementation relationships                        |
| `di_mappings`        | DI registrations (interface → implementation, lifetime)       |
| `endpoints`          | HTTP endpoint signatures (route, method, handler, auth)       |
| `event_flows`        | Event pub/sub topology (direction, mechanism, event_type)     |
| `modules`            | Architecture module nodes (file, language, layer, abstract)   |
| `module_deps`        | Module dependency edges (direct / abstract / external)        |
| `module_ext_imports` | External packages imported by a module (purity checks)        |
| `graph_nodes`        | Evidence-aware graph nodes (file, type, method, endpoint…)   |
| `graph_edges`        | Typed relations with analyzer + confidence                     |
| `graph_evidence`     | All observations for an edge                                  |
| `analysis_coverage`  | Whether heuristic / TypeScript / Roslyn ran                  |

All tables have a `source` column (`heuristic` by default) to support future Roslyn enrichment.

## Typical workflow

```bash
# 1. Initialize repos (one-time setup per clone)
repo-context init ./repos

# 2. Run first inventory scan
repo-context inventory ./repos --format table

# 3. Architecture fitness (CI)
repo-context fitness ./repos/MyRepo --fail-on error

# 4. Query specific data (agent-friendly)
repo-context query ./repos/MyRepo --section modules
repo-context query ./repos/MyRepo --section types
repo-context query ./repos/MyRepo --sql "SELECT * FROM di_mappings WHERE interface = 'IOrderService'"
repo-context query ./repos/MyRepo --section events --format table

# 5. Viewer API + SPA
repo-context serve ./repos/MyRepo --static ../arch-view/dist

# 6. Check status across all repos
repo-context index ./repos

# 7. After code changes, refresh incrementally
repo-context inventory ./repos --refresh
```

## Output files

| File                 | Purpose                                               |
| -------------------- | ----------------------------------------------------- |
| `inventory.db`       | SQLite database — the authoritative queryable store   |
| `inventory-toc.json` | Lightweight summary with metadata and counts (~1–3KB) |
| `arch-rules.json`    | Optional layer/purity guidance at repo root           |

## Notes

- **`.git/info/exclude` protection** — `init --protect` appends `*` for analysis-only clones. Do not use `--protect` on working repos.
- **Semantic analyzers** — optional Node TypeScript compiler sidecar (`analyzers/typescript`) and Roslyn sidecar (`analyzers/roslyn`). Heuristic facts stay available; compiler-resolved facts have higher confidence. Unresolved dispatch is kept and marked.
- **Pure Go SQLite** — uses `modernc.org/sqlite` (no CGO). Adds ~8MB to binary size.
- **Project dependency detection** — parses `.csproj` `ProjectReference` elements, `angular.json` workspace projects, and TypeScript `paths` aliases to build a cross-project dependency graph with topological sorting and cycle detection.
- **Module graph** — TypeScript imports (relative, path aliases, workspace packages) and C# `using` of in-repo namespaces; framework namespaces (`System.*`, `Microsoft.*`) are external for purity checks.
- **Roslyn sidecar** — optional .NET analyzer under `analyzers/roslyn`, invoked via `inventory --semantic=auto|required` (not a separate `--roslyn` flag).
- **File walking** — skips: `bin`, `obj`, `node_modules`, `.git`, `packages`, `dist`, `vendor`, `coverage`, `test-results`.
- **Backward compatibility** — v1 `inventory.json` files are not read or deleted. The `index` command reports them as `STALE (v1 json)`. Run `inventory --force` to migrate.
- **Golden targets** — the target repo `apps/schedule-api` and `apps/safelog-api`: domain trees should stay clean; composition/http wiring is outer-layer and should not false-positive as domain.
