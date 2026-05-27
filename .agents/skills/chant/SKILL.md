---
name: chant
description: Use before writing new code (check for a reusable verified recipe) and after solving a task (capture the procedure as a recipe). chant caches the tested method, not the answer.
---

# chant

chant is a recipe cache for agents. It stores successful work as **verified,
reusable procedures** and reuses them verifier-first.

## Before writing new code

```bash
chant suggest --task "<what you are about to do>" --files "<input files>" --json
```

Read the JSON: `match_found`, `hits[].id`, `hits[].verifier_exists`, `hits[].reuse_command`.
If a hit has a verifier, reuse it instead of writing new code:

```bash
chant verify <id> --input file=<path> --json    # runs + verifies; trust only if "trusted": true
```

A cache hit is a *candidate*, never trusted until the verifier passes.

## After solving a task

If you wrote code that solved a recurring task and it passes a test/verifier,
capture it so the next similar task reuses it:

```bash
chant capture --id <slug> --task "<task>" \
  --command "<command to reuse, may contain {{vars}}>" \
  --verifier "<verifier command, exit 0 == passed>" \
  --tags "<comma,tags>" --json
chant verify <slug>     # confirm the verifier passes
```

## Maintenance

- `chant list` / `chant search "<query>"` — browse the library.
- `chant invalidate <id>` — mark a recipe stale when it stops working.
- `chant status` / `chant doctor` — health of the recipe library.

Treat `.chant/` as local runtime state. Recipes under `recipes/` are committed.
