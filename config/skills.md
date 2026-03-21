# Shared Skills

## Repo Analysis
**Trigger:** User asks to understand, audit, or map a codebase.

1. Glob for entry points (`main.*`, `index.*`, `app.*`, `server.*`).
2. Grep for key patterns (exports, routes, DB calls, env vars).
3. Read CLAUDE.md, README, and package manifests first.
4. Summarize architecture: layers, data flow, external dependencies.
5. Flag anything unusual (dead code, circular deps, security smells).

## Summarization
**Trigger:** User asks for a summary of a file, PR, diff, or conversation.

- Lead with a one-sentence TL;DR.
- Follow with bullet points for key changes/decisions.
- Note any risks, open questions, or follow-ups.
- Keep it under 200 words unless depth is requested.

## Dependency Audit
**Trigger:** User asks about packages, versions, or supply-chain concerns.

1. Read `package.json` / `requirements.txt` / `go.mod` / `Cargo.toml`.
2. Identify outdated, deprecated, or CVE-flagged packages.
3. Flag unused dependencies.
4. Suggest minimal upgrade path.

## Refactor Review
**Trigger:** User asks to review or improve existing code.

- Identify duplication, dead code, and over-abstraction.
- Suggest consolidation only where it reduces complexity.
- Never refactor beyond what was asked.
- Preserve existing behavior — note any behavior changes explicitly.

## Test Coverage Check
**Trigger:** User asks about test coverage or wants tests written.

1. Locate test files (Glob `**/*.test.*`, `**/*_test.*`, `**/tests/**`).
2. Identify untested public interfaces and critical paths.
3. Write tests that hit real behavior — avoid mocking internals.
4. Match the existing test style and framework.

## Git History Analysis
**Trigger:** User asks why something was done, who changed it, or when.

- Use `git log --follow -p <file>` for file history.
- Use `git blame` for line-level attribution.
- Cross-reference commit messages for intent.
- Surface relevant issues or PR references from commit bodies.

## Analysis Pipeline
**Trigger:** User says "run the analysis pipeline", "analyze this repo end to end", or "scan and plan [repo/path]".

1. Confirm the repository path with the user if not provided.
2. Run Phase 1 sequentially using the `repo-scanner`, `flow-mapper`, `feature-synthesizer`, and `gap-detector` agents — pass accumulated output forward at each step.
3. Run the `interrogation` agent and present the questions to the user. **Stop and wait for answers.**
4. Once answers are received, run `decision-recorder` then `planner`.
5. Present the final planner output (tasks, execution order, milestones) to the user as the result.

Do not skip the human gate. Do not proceed to Phase 2 without explicit user answers to interrogation questions.

## YouTube Transcript
**Trigger:** User shares a YouTube URL and asks for a transcript, summary, or analysis of the video.

1. Extract the video ID from the URL (the `v=` query param, or the last path segment for `youtu.be` links).
2. Run the following Python snippet via Bash to fetch the transcript:
   ```bash
   python -c "
   from youtube_transcript_api import YouTubeTranscriptApi
   api = YouTubeTranscriptApi()
   t = api.fetch('VIDEO_ID')
   print('\n'.join([s.text for s in t]))
   "
   ```
3. If `youtube-transcript-api` is not installed, install it first: `pip install youtube-transcript-api -q`
4. If no transcript is available (private video, no captions), inform the user and suggest they provide a manual transcript or use YouTube's own caption download.
5. Once the transcript is retrieved, proceed with whatever the user asked (summary, concept extraction, Q&A, etc.).
