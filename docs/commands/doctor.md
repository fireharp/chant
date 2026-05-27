# `chant doctor`

> Validate config + store. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdDoctor`).

## What it checks

A quick environment check after `init` or before adopting chant in a repo. Not
part of any reuse pipeline — diagnostic only.

1. **`config`** — `chant.yml` is present at the repo root. `warn` if absent
   (chant still works with defaults; run `chant init`).
2. **`recipes-dir`** — the configured `recipes_dir` exists. `warn` if missing.
3. **`recipes`** / **`verifiers`** — every recipe loads, and how many have a
   verifier. `warn` when any recipe has no verifier (its reuse can't be
   trusted); `fail` if a recipe fails to load.
4. **`gitignore`** — `.gitignore` excludes `.chant/`. `warn` if not (the runtime
   index, run logs, and `STATUS.md` shouldn't be committed).

Exit code is `1` only when a check is `fail`. `warn` issues are reported but do
not block.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--json` | false | emit the checks as JSON. |

## Example (human)

```bash
chant doctor
```

```text
[warn] config: chant.yml absent — using defaults (run `chant init`)
[ok] recipes-dir: recipes/ present
[ok] verifiers: all 1 recipe(s) have a verifier
[warn] gitignore: .chant/ not gitignored — add it
doctor: no blocking issues.
```

(The `config` and `gitignore` warnings above are real output from this repo,
which has no `chant.yml` and gitignores `.coherence/` but not `.chant/`. Running
`chant init` clears both.)

## Example (JSON)

```bash
chant doctor --json
```

```json
{
  "checks": [
    {"name": "config",      "status": "warn", "detail": "chant.yml absent — using defaults (run `chant init`)"},
    {"name": "recipes-dir", "status": "ok",   "detail": "recipes/ present"},
    {"name": "verifiers",   "status": "ok",   "detail": "all 1 recipe(s) have a verifier"},
    {"name": "gitignore",   "status": "warn", "detail": ".chant/ not gitignored — add it"}
  ],
  "ok": true
}
```

## Output shape

`checks[]` (each with `name`, `status`, `detail`) plus a top-level `ok` boolean.

| Status | Meaning |
|---|---|
| `ok` | check passed. |
| `warn` | something is off but chant still works; surface the fix, don't block. |
| `fail` | something blocks correct operation. |

`ok` is `true` iff no check is `fail`. Exit code: `0` on `ok: true`, `1` on
`ok: false`.
