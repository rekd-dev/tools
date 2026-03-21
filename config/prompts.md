# Reusable Prompt Templates

## Code Review
```
Review the following code for:
- Correctness and edge cases
- Security vulnerabilities (OWASP top 10)
- Unnecessary complexity or over-engineering
- Deviations from the surrounding code style

Be direct. List issues by severity (critical / warning / suggestion).
```

## Bug Investigation
```
I'm seeing the following behavior: [DESCRIBE SYMPTOM]
Expected: [DESCRIBE EXPECTED]
Relevant file(s): [FILE PATHS]

Investigate root cause. Do not suggest workarounds — find the actual bug.
```

## Feature Implementation
```
Implement [FEATURE] in [FILE/MODULE].

Constraints:
- Match existing code style and patterns
- No new dependencies unless necessary
- No extra configurability beyond what is needed
- Update tests if a test file exists
```

## Explain This Code
```
Explain what this code does, focusing on:
1. What problem it solves
2. How it works at a high level
3. Any non-obvious decisions or gotchas

Assume I understand [LANGUAGE] but am unfamiliar with this specific module.
```

## Migration / Upgrade
```
Migrate [THING] from [OLD VERSION/PATTERN] to [NEW VERSION/PATTERN].

- Preserve all existing behavior
- Note any breaking changes explicitly
- Do not refactor unrelated code
- Update tests to match
```

## Architecture Decision
```
I need to decide between [OPTION A] and [OPTION B] for [CONTEXT].

Evaluate both options on:
- Simplicity
- Maintainability
- Performance implications
- Risk / reversibility

Give a clear recommendation with reasoning.
```

## Incident / Debug Runbook
```
We have an incident: [DESCRIBE ISSUE]
Current impact: [WHO/WHAT IS AFFECTED]
What we've tried: [STEPS TAKEN]

Suggest next diagnostic steps. Prioritize fast, safe, non-destructive checks first.
```

## Analysis Pipeline

Run the full repository analysis pipeline against a path. The pipeline is two-phase with a required human decision gate between them.

**Phase 1 — Automated analysis (run sequentially, pass all output forward):**
1. `repo-scanner` — scan `[REPO_PATH]` for modules, entry points, dependencies
2. `flow-mapper` — map execution and data flow using repo-scanner output
3. `feature-synthesizer` — identify features from modules and flows
4. `gap-detector` — detect gaps and dead code from all prior output

**Human gate:**
5. `interrogation` — generate decisions for blocking gaps; present questions to user and wait for answers before continuing

**Phase 2 — Resolution (run after answers are received):**
6. `decision-recorder` — convert user answers into structured decisions
7. `planner` — generate implementation plan from features, gaps, and decisions

**Invoke with:**
"Run the analysis pipeline on [REPO_PATH]"

Each agent outputs JSON. Pass the full accumulated output of all prior agents into each subsequent agent.
Do not proceed past step 5 until the user has answered the interrogation questions.
