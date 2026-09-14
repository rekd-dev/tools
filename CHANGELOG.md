# Changelog

## Unreleased

### Changed

- MIT license; contribution policy is invite-only (forks welcome).
- Removed store-local `safe-log-helper/` from the published tree (still gitignored locally).
- Docs use generic inventory paths instead of a private product checkout.

### Added

- `repo-context-cli/build.sh` for Linux/macOS (alongside `build.ps1`).
- Local **arch-view** SPA (Architecture / Focus / Flow lenses, fitness overlay) served by `repo-context serve`.
- Evidence-aware graph store (`graph_nodes` / `graph_edges` / `graph_evidence`) and `inventory --semantic=auto|required|off`.
- `init --protect` for analysis-only clones (`.git/info/exclude`).
- Serve APIs: `/api/search`, `/api/entity`, `/api/edge`, `/api/path`, `/api/graph-view`, `/api/coverage` plus existing module/fitness/view/source/meta/health endpoints.
