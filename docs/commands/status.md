# `chant status`

> Rewrite the library status snapshot. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdStatus`) +
> [`internal/status/status.go`](../../internal/status/status.go).

## What it does

Computes a snapshot of the recipe library — counts, run totals, and a per-recipe
table — and writes it to `.chant/STATUS.md` (a human-readable Markdown file,
mirroring coherence's `STATUS.md` convention). With `--json` it also emits the
structured report.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--json` | false | emit the structured report as JSON. |

## Example (human)

```bash
chant status
```

```text
wrote /path/to/repo/.chant/STATUS.md (1 recipe(s), 1 active, 0 stale)
```

The written `STATUS.md` contains a table:

```markdown
# chant status

- recipes: **1** (1 active, 0 stale)
- recorded runs: 5 (0 failures)

| recipe | v | status | runs | fails | success | verifier |
| --- | --- | --- | --- | --- | --- | --- |
| `csv-revenue-by-channel` | 1 | active | 5 | 0 | 100% | yes |

> A recipe with a verifier can be *trusted* on reuse only after `chant verify`
> passes. Retrieval ranks candidates; the verifier blesses them.
```

## Example (JSON)

```bash
chant status --json
```

```json
{
  "generated_at": "2026-05-27T21:12:56Z",
  "recipe_count": 1,
  "active_count": 1,
  "stale_count": 0,
  "total_runs": 5,
  "total_failures": 0,
  "recipes": [
    {
      "id": "csv-revenue-by-channel",
      "version": 1,
      "status": "active",
      "runs": 5,
      "failures": 0,
      "success_rate": 1,
      "has_verifier": true
    }
  ]
}
```

## JSON shape

The status report shape: `generated_at`, `recipe_count`, `active_count`,
`stale_count`, `total_runs`, `total_failures`, and `recipes[]` where each
`RecipeStat` carries `id`, `version`, `status`, `runs`, `failures`,
`success_rate`, and `has_verifier`.

## Use in a pre-commit gate

chant pairs with coherence rather than shipping its own hook. To surface library
health on every commit, add a `chant status` line to your existing
`.githooks/pre-commit` — it rewrites `STATUS.md` so the snapshot stays current.
