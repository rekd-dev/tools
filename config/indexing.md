# Shared Indexing Rules

## What to Index
- Entry points: `main.*`, `index.*`, `app.*`, `server.*`, `cli.*`
- Configuration: `.env.example`, `*.config.*`, `*.toml`, `*.yaml`, `*.json` (root-level)
- Manifests: `package.json`, `requirements.txt`, `go.mod`, `Cargo.toml`, `pyproject.toml`
- Documentation: `README*`, `CLAUDE.md`, `CHANGELOG*`, `docs/**`
- Schema definitions: `*.schema.*`, `migrations/**`, `*.sql`
- Public interfaces: exported functions, API route definitions, CLI command definitions

## What NOT to Index
- Build artifacts: `dist/`, `build/`, `out/`, `target/`, `.next/`
- Dependency directories: `node_modules/`, `vendor/`, `.venv/`, `__pycache__/`
- Generated files: `*.generated.*`, `*.pb.go`, `*.min.js`
- Binary and media files: `*.png`, `*.jpg`, `*.pdf`, `*.zip`, `*.exe`
- Lock files: `package-lock.json`, `yarn.lock`, `poetry.lock` (read manifests instead)
- IDE/OS noise: `.DS_Store`, `Thumbs.db`, `.idea/`, `.vscode/` (unless configuring editor)
- Secrets: `.env`, `*.key`, `*.pem`, `credentials.*`

## Search Priority Order
1. `CLAUDE.md` — project-specific overrides and instructions
2. Entry points and manifests
3. Files matching the user's explicit query
4. Related files discovered via imports/references

## Naming Conventions to Recognize
| Pattern | Likely Role |
|---|---|
| `*.service.*` | Business logic / service layer |
| `*.controller.*` / `*.handler.*` | Request handling |
| `*.model.*` / `*.entity.*` | Data models |
| `*.repo.*` / `*.store.*` | Data access layer |
| `*.util.*` / `*.helper.*` | Shared utilities |
| `*.test.*` / `*_test.*` | Tests |
| `*.types.*` / `*.d.ts` | Type definitions |

## When Indexing a New Project
1. Read `CLAUDE.md` if present — it overrides everything else.
2. Read the root manifest to identify language, framework, and scripts.
3. Glob for entry points.
4. Grep for environment variable usage (`process.env`, `os.environ`, `viper.Get`).
5. Note any unusual directory structure and ask the user before assuming conventions.
