# Architecture Viewer

React + TypeScript SPA that visualizes a repository module graph produced by `repo-context`.

Inspired by [unclebob/arch-view](https://github.com/unclebob/arch-view): topo-layered modules, drill-down, cycle highlighting, abstract (interface) nodes, and click-to-source — plus fitness overlay, interface resolution, and a bounded flow lens with data/async overlays.

## Prerequisites

1. Build `repo-context` (`cd ../repo-context-cli && .\build.ps1`)
2. Optional semantic sidecars: Node 22 + `npm install` in `../repo-context-cli/analyzers/typescript`, and .NET 8 for the Roslyn analyzer
3. Inventory a target repo:

```bash
repo-context inventory "./path/to/your-repo" --force
```

`--semantic=auto` (default) runs compiler sidecars when they are installed. Heuristic facts are always stored. Confidence on each edge says whether it is heuristic or compiler-resolved.

## Dev

Terminal A — API:

```bash
repo-context serve "./path/to/your-repo" --addr 127.0.0.1:8787
```

Terminal B — SPA:

```bash
npm install
npm run dev
```

Open http://localhost:5190 (`vite` proxies `/api` to the serve process).

## Lenses

- **Architecture** — outer-to-inner drill. Single-click selects, double-click drills into folders.
- **Focus** — neighborhood around a type/interface: implementers, DI bindings, constructor injection, consumers, evidence.
- **Flow** — bounded static call path from an endpoint or symbol. **Data** and **Async** overlays stay on the same selection.

Shareable URL hash restores `lens`, `path`, `sel`, and overlays. Architecture drill-down uses `/api/view`; Focus/Flow use `/api/graph-view?lens=focus|flow&sel=&path=&overlays=`.

## Production-ish local

```bash
npm run build
repo-context serve "./path/to/your-repo" --static ./dist
```

Then open http://127.0.0.1:8787

## Legend

| Visual | Meaning |
| --- | --- |
| Layered rows | Outer modules depend on inner ones |
| Green "interface" | Abstract type |
| Red border | Cycle or fitness error |
| Dashed edge | Unresolved or low-confidence |
| Coverage chips | Whether heuristic / TypeScript / Roslyn ran |

## Fitness panel

Loaded from `/api/fitness` — same JSON as `repo-context fitness --format json`. Clicking an edge or finding opens the source line when the fact includes file/line evidence.
