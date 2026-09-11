# Architecture viewer backlog

Two piles: **research** (deferred on purpose) and **graph quality** (the remaining work that still changes what you see).

## Graph quality (active)

From `.cursor/plans/graph-quality-punch.md`. Highest leftover first.

### Landed
- P0-1 / P0-2 — TypeScript sidecar method nodes; calls attributed to the method; method-level `implements`
- P0-3 — unresolved / low-confidence calls quarantined from default Flow (`overlays=unresolved` to opt in)
- P0-4 — method names cleaned at Merge (no SQL blobs / `this.deps.*`)
- P0-5 — reads vs writes; one data sink per adapter class (Data overlay is writes, not `findById`)
- P1-3 — type-only imports are fitness `info`, not errors
- P1-6 — `graphHub` reloads when `inventory.db` mtime/size changes (no serve restart after rescan)
- P1-7 — Focus second hop is implements/binds of the selected node, not injects of every neighbour
- P2-4 — `/api/path` returns `edges: []` instead of JSON `null`

### Viewer chrome (landed this arc)
- Contextual guide strip (lens / overlays / selection)
- Header controls only for the current pane (Churn on Architecture Graph, not Matrix; Flow overlays on Flow)
- Source dock with Dark+ highlighting
- App-family clustering, DSM matrix, edge inspector, git churn on the graph

### Next slices
- **P1-1** Heuristic owner attribution: class nearest above the site, never a random type in the file
- **P1-2** Stop double-declaring functions as `type:` nodes (unify with sidecar `method:` ids)
- **P1-4** Sidecar walks arrow/route plugins (`me-time.ts` still has 0 `calls`; endpoint→UC is heuristic `handles`)
- **P1-5** Inline object-type ctor params + `import type` aliases
- **P1-8** `spineInject` from resolved calls, not port name tokens
- **P2** Hygiene: `nodehttp` closure `handles`, skip-rule unify, stable heuristic edge ids, SignalR mislabel, inherit layer onto methods, fitness import line numbers

### Viewer ideas still open
- Zoom-to-this-app (Fit View still fits the whole canvas)
- Finding click opening both `from` and `to`
- Guided tours / review comments (the guide strip is the lightweight stand-in)

## Runtime evidence (deferred)
- Native runtime instrumentation and trace import
- Object identity / DI scope timelines
- Observed execution vs static call graphs
- Deployed service instances

## Deeper lineage (deferred)
- Field/column taint analysis
- Arbitrary reflection and dynamic dispatch
- Cross-process message payload schemas

## Evolution and ownership
- Git ownership, churn, and time travel — **churn overlay landed** (Architecture Graph → Churn). Time travel and intended-model drift remain deferred.
- Architecture drift against an editable intended model
- Cross-repository graphs

## Alternative visual lenses
- 3D / city views
- DSM / matrix — **landed**
- ELK layered Flow layout — **landed**
- Sankey and treemap projections (the graph contract already allows them)

## Collaboration and agents
- Hosted multi-user product
- MCP server — **landed** as `repo-context mcp <path>`
- Playwright smoke — **landed** as `arch-view` `npm run test:e2e`
- Guided tours and review comments
