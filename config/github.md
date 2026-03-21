# GitHub Awareness

## GitHub as Default Remote

Every project git repo should have a GitHub remote. When starting work in a project directory:

1. Run `git remote -v` to check for a remote.
2. If no remote exists, follow the **New Project Initialization** steps below before writing any code.

## New Project Initialization

**Trigger:** Directory has no `.git` folder, or is a git repo with no remote.

Steps:
1. `git init` (if not already a repo)
2. Create a `.gitignore` appropriate for the detected stack (Go, Python, Node, etc.)
3. `gh repo create <name> --private --source=. --remote=origin --push`
4. Confirm `git remote -v` shows `origin` before proceeding

Default visibility is **private**. Only use `--public` if the user explicitly requests it.

## Atomic Commit Workflow

Every commit must represent one coherent logical change. Never batch unrelated changes.

**Before committing:**
1. Invoke the `pre-commit` agent, passing the list of changed files and the proposed commit message.
2. The `pre-commit` agent will verify the build, run tests, and update any stale documentation.
3. Do not commit until the `pre-commit` agent returns `"pass": true`.

**Committing:**
- Stage selectively — use specific file paths or `git add -p`. Never `git add .` blindly.
- Use conventional commit message format: `type(scope): description`
  - Types: `feat`, `fix`, `chore`, `refactor`, `docs`, `test`, `build`
  - Examples:
    - `feat(auth): add JWT refresh endpoint`
    - `fix(parser): handle empty input`
    - `chore: add .gitignore`
    - `docs(readme): update setup instructions`
- `git commit -m "<message>"`

**After committing:**
- `git push` immediately. The remote must stay in sync with local at all times.
