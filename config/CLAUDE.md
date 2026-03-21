# Claude Code — Global Config

This directory contains shared configuration loaded across all projects under `/path/to/tools/`.

## Files

| File                                                       | Purpose                                                                     |
| ---------------------------------------------------------- | --------------------------------------------------------------------------- |
| [`system.md`](./system.md)                                 | Global agent behavior, persona, tone, and decision-making rules             |
| [`skills.md`](./skills.md)                                 | Shared skill definitions with triggers (repo analysis, summarization, etc.) |
| [`prompts.md`](./prompts.md)                               | Reusable prompt templates for common tasks                                  |
| [`indexing.md`](./indexing.md)                             | Rules for what to index, search priority, and naming conventions            |
| [`agent-build-guidelines.md`](./agent-build-guidelines.md) | Rules for coding and developing the indexing pipeline                       |

## How This Works

Claude Code automatically reads `CLAUDE.md` files at startup. To make these global configs apply to a project:

1. Add an `@import` reference in that project's own `CLAUDE.md`:
   ```
   @/path/to/tools/config/system.md
   @/path/to/tools/config/skills.md
   @/path/to/tools/config/prompts.md
   @/path/to/tools/config/indexing.md
   @/path/to/tools/config/agent-build-guidelines.md
   ```
2. Or place a `CLAUDE.md` in a parent directory — Claude Code walks up the directory tree and loads all `CLAUDE.md` files it finds.

## Overrides

Project-level `CLAUDE.md` files take precedence over this global config. Any rule here can be overridden locally by restating it in the project's own `CLAUDE.md`.

## Maintenance

- Edit files here when a rule or skill should apply globally.
- Add project-specific rules only to that project's `CLAUDE.md`.
- Keep this index up to date if new config files are added.
