# `chant capture`

> The after-success hook. When an agent solves a recurring task and it passes a
> test/verifier, capture the procedure as a recipe so the next similar task
> reuses it. Source: [`cmd/chant/commands.go`](../../cmd/chant/commands.go)
> (`cmdCapture`).

## What it does

Writes a new recipe card (`recipes/<id>/recipe.yaml`) from the supplied task,
command, verifier, and signals — optionally copying an entrypoint script into
the recipe directory — then rebuilds the index. The captured recipe starts at
`version: 1`, `status: active`, with zero recorded runs, and `chant capture`
also writes the [enchantment metadata](#enchantment-metadata) layer
(`spell_hash`, `provenance`, `scope`, `portability`). Capture does **not**
establish trust; the recommended next step is `chant verify <id>` to run the
verifier and record the first trust event.

> **A recipe runs inside its own directory** (`recipes/<id>/`). Any file the
> `command` or `verifier` references must live there, or `chant verify` fails
> because the file isn't found. Use `--entrypoint-src <path>` to copy a script
> into the recipe dir at capture time.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--id` | slug of `--task` | recipe id (slug). |
| `--task` | empty | task description; becomes the first `when_to_use.task_patterns` entry. Required unless `--id` is given. |
| `--description` | `--task` value | recipe description. |
| `--command` | *(required)* | the `what_to_do.command` (may contain `{{vars}}`). |
| `--language` | empty | informational language tag; also populates `portability.requires.runtime`. |
| `--entrypoint` | empty | entrypoint filename inside the recipe dir. |
| `--entrypoint-src` | empty | copy this file into the recipe dir as the entrypoint. |
| `--verifier` | empty | verification command (exit 0 == verified). |
| `--expect-artifacts` | empty | comma-separated expected output artifacts. |
| `--columns` | empty | comma-separated logical columns the inputs must satisfy; populates `portability.input_contract.required_columns_any`. |
| `--supersedes`, `--mirrors`, `--generalizes`, `--specializes`, `--depends-on`, `--implements` | empty | comma-separated target ids/refs for each typed [relations](relations.md) kind (spec §3 + §7). |
| `--author` | `agent:capture` | provenance author written to `provenance.author`. |
| `--tags` | empty | comma-separated tags. |
| `--patterns` | empty | comma-separated extra task patterns. |
| `--file-signals` | empty | comma-separated input file globs. |
| `--force` | false | overwrite an existing recipe. |
| `--json` | false | emit the JSON outcome contract. |

Capture errors if `--command` is empty, or if neither `--task` nor `--id` is
given, or if the recipe already exists without `--force`. Flags work in any
position.

## Example — verified end to end

This walkthrough is verified working. `--entrypoint-src hello.sh` copies the
script into `recipes/greet/` as `greet.sh` so the in-dir command and verifier
can find it:

```bash
printf '#!/bin/sh\necho "hello $1"\n' > hello.sh

chant capture --id greet --task "greet a name" \
  --command 'sh greet.sh {{name}}' --entrypoint-src hello.sh --entrypoint greet.sh \
  --verifier 'sh -c "test \"$(sh greet.sh world)\" = \"hello world\""'

chant verify greet --input name=world
# → ✓ greet verified — trusted
```

```text
captured recipe "greet" (v1) at /path/to/repo/recipes/greet
→ run `chant verify greet` to confirm the verifier passes.
```

## Example (JSON)

```bash
chant capture --id greet --task "greet a name" \
  --command 'sh greet.sh {{name}}' --entrypoint-src hello.sh --entrypoint greet.sh \
  --verifier 'sh -c "test \"$(sh greet.sh world)\" = \"hello world\""' \
  --author "agent:claude" --json
```

```json
{
  "subcommand": "capture",
  "match_found": false,
  "recipe_id": "greet",
  "version": 1,
  "exit_code": 0,
  "trusted": false,
  "captured": true,
  "message": "recipe captured — verify it to establish trust",
  "recommended_next_command": "chant verify greet"
}
```

## Example — capture without a verifier

A capture without a verifier still succeeds (exit 0) but flags the gap. The
human output warns:

```text
captured recipe "nv" (v1) at /path/to/repo/recipes/nv
⚠ no verifier set — add one so reuse can be trusted (a hit without a verifier is just a guess).
```

Under `--json` the gap is carried in `message` and `suggested_commands` rather
than printed as prose:

```json
{
  "subcommand": "capture",
  "match_found": false,
  "recipe_id": "nv",
  "version": 1,
  "captured": true,
  "trusted": false,
  "message": "captured WITHOUT a verifier — reuse cannot be trusted until you add one",
  "suggested_commands": [
    "chant capture --id nv --force --verifier \"<cmd>\" ..."
  ],
  "recommended_next_command": "chant verify nv"
}
```

## Enchantment metadata

Every capture writes a fully backward-compatible metadata layer onto the card:

```yaml
spell_hash: ff9a7d644ac15c3d    # content-addressed identity of the procedure
provenance:
    origin: /abs/path/to/repo   # repo root at capture time
    captured_at: "2026-05-27T21:35:06Z"
    author: agent:claude        # from --author (defaults to agent:capture)
scope: project                  # first rung of the universality ladder
portability:
    determinism: deterministic
    input_contract:
        required_columns_any:   # from --columns
            - [channel, amount]
    requires:
        runtime: python         # from --language
```

`--author` sets `provenance.author`; `--columns a,b` populates
`portability.input_contract.required_columns_any`; `--language` populates
`portability.requires.runtime`.

### Typed relations (since v0.3)

Six additional flags populate `relations.*` on the captured card and let
[`chant relations`](relations.md) query the resulting lineage edges:

| Flag             | Populates                       | Use for                                          |
| ---------------- | ------------------------------- | ------------------------------------------------ |
| `--supersedes`   | `relations.supersedes`           | this enchantment replaces those (version lineage).|
| `--mirrors`      | `relations.mirrors`              | same procedure, another language/package.         |
| `--generalizes`  | `relations.generalizes`          | this is a broader form of those.                  |
| `--specializes`  | `relations.specializes`          | this is a narrower form of those.                 |
| `--depends-on`   | `relations.depends_on`           | data/config the procedure requires.               |
| `--implements`   | `relations.implements`           | user-story / policy ids this fulfils.             |

Each accepts a comma-separated list. Targets do not need to exist in the
local library (cross-package + forward references are fine — `chant doctor`
surfaces them as warnings).

`scope` and `verified_in` populate at first capture as `project` with no
contexts; both grow as `chant verify` passes in new repos. The cross-package
registry, scope promotion, and typed relations described in
[`docs/specs/enchantment-metadata.md`](../specs/enchantment-metadata.md) are
**shipped** (see [registry](../README.md#cross-package-reuse-the-registry),
[`promote`](promote.md), [`relations`](relations.md)).

## JSON shape

`subcommand: "capture"`, `match_found: false`, `recipe_id`, `version: 1`,
`captured: true`, `trusted: false`, `message`, `recommended_next_command`
(`chant verify <id>`), and — when no verifier was set — `suggested_commands`.
See the [outcome contract](../README.md#json-outcome-contract).

## Versioning note

`capture` will not overwrite an existing recipe without `--force`. To create a
new version of a recipe whose method changed, `chant invalidate` the old one (or
recapture with `--force` and bump the `version` in the card). The card's
`metrics` reset to zero on a fresh capture.
