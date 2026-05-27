# `chant init`

> Scaffold a fresh chant repo. Source:
> [`cmd/chant/init.go`](../../cmd/chant/init.go).

## What it does

Bootstraps chant in the current repo. It:

- writes `chant.yml` (the committed config with default retrieval weights),
- creates `recipes/.gitkeep` (the committed recipe library),
- installs the agent skill at `.agents/skills/chant/SKILL.md`,
- appends `.chant/` to `.gitignore` (without clobbering existing entries),
- creates the `.chant/` runtime state directory.

It is **idempotent**: existing files are skipped without `--force`. After init,
run `chant doctor` to verify.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--force` | false | overwrite existing files instead of skipping them. |
| `--json` | false | emit a JSON summary of created/skipped paths. |

## Example (human)

```bash
chant init
```

```text
chant init in /path/to/repo
  created  chant.yml
  created  recipes/.gitkeep
  created  .agents/skills/chant/SKILL.md
  created  .gitignore (+.chant/)

Next:
  $ chant capture --id <slug> --task "..." --command "..." --verifier "..."
  $ chant verify <slug>
  $ chant suggest --task "..." --json    # wire into your agent's pre-write hook

Pre-commit: chant pairs with coherence. If you want a chant gate too, add
'chant status' or a 'chant bench' line to .githooks/pre-commit.
```

On a repo already initialized, those lines read `skipped  <path>` instead.

## Example (JSON)

```bash
chant init --json
```

```json
{
  "created": [
    "chant.yml",
    "recipes/.gitkeep",
    ".agents/skills/chant/SKILL.md",
    ".gitignore (+.chant/)"
  ],
  "root": "/path/to/repo",
  "skipped": null
}
```

## JSON shape

This command emits its own summary shape (not the standard outcome contract):
`created[]`, `skipped[]`, and `root`. Each path that already existed appears in
`skipped` (with a parenthetical note for `.gitignore`); each newly written path
appears in `created`.

## Repo-root discovery

`init` writes relative to the repo root, found by walking up from the working
directory for a `.git` directory or an existing `chant.yml`. In a non-git
scratch directory with no `chant.yml`, the working directory is used as the
root.

## Pre-commit

chant does not install its own hook. It pairs with coherence: if you want a
chant gate too, add a `chant status` or `chant bench` line to your existing
`.githooks/pre-commit`.
