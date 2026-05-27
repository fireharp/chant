# `chant capture`

> The after-success hook. When an agent solves a recurring task and it passes a
> test/verifier, capture the procedure as a recipe so the next similar task
> reuses it. Source: [`cmd/chant/commands.go`](../../cmd/chant/commands.go)
> (`cmdCapture`).

## What it does

Writes a new recipe card (`recipes/<id>/recipe.yaml`) from the supplied task,
command, verifier, and signals — optionally copying an entrypoint script into
the recipe directory — then rebuilds the index. The captured recipe starts at
`version: 1`, `status: active`, with zero recorded runs. Capture does **not**
establish trust; the recommended next step is `chant verify <id>` to run the
verifier and record the first trust event.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--id` | slug of `--task` | recipe id (slug). |
| `--task` | empty | task description; becomes the first `when_to_use.task_patterns` entry. Required unless `--id` is given. |
| `--description` | `--task` value | recipe description. |
| `--command` | *(required)* | the `what_to_do.command` (may contain `{{vars}}`). |
| `--language` | empty | informational language tag. |
| `--entrypoint` | empty | entrypoint filename inside the recipe dir. |
| `--entrypoint-src` | empty | copy this file into the recipe dir as the entrypoint. |
| `--verifier` | empty | verification command (exit 0 == verified). |
| `--expect-artifacts` | empty | comma-separated expected output artifacts. |
| `--tags` | empty | comma-separated tags. |
| `--patterns` | empty | comma-separated extra task patterns. |
| `--file-signals` | empty | comma-separated input file globs. |
| `--force` | false | overwrite an existing recipe. |
| `--json` | false | emit the JSON outcome contract. |

Capture errors if `--command` is empty, or if neither `--task` nor `--id` is
given, or if the recipe already exists without `--force`.

## Example (JSON)

```bash
chant capture --id hello-echo --task "print a greeting" \
  --command 'echo hello > out.txt' \
  --verifier 'test -f out.txt' --expect-artifacts out.txt \
  --tags demo --json
```

```json
{
  "subcommand": "capture",
  "recipe_id": "hello-echo",
  "version": 1,
  "exit_code": 0,
  "trusted": false,
  "captured": true,
  "message": "recipe captured — verify it to establish trust",
  "recommended_next_command": "chant verify hello-echo"
}
```

## Example (human)

```text
captured recipe "hello-echo" (v1) at /path/to/repo/recipes/hello-echo
→ run `chant verify hello-echo` to confirm the verifier passes.
```

If you capture **without** a verifier, the human output warns:

```text
⚠ no verifier set — add one so reuse can be trusted (a hit without a verifier is just a guess).
```

## JSON shape

`subcommand: "capture"`, `recipe_id`, `version: 1`, `captured: true`,
`trusted: false`, `message`, and `recommended_next_command` (`chant verify
<id>`). See the [outcome contract](../README.md#json-outcome-contract).

## Versioning note

`capture` will not overwrite an existing recipe without `--force`. To create a
new version of a recipe whose method changed, `chant invalidate` the old one (or
recapture with `--force` and bump the `version` in the card). The card's
`metrics` reset to zero on a fresh capture.
