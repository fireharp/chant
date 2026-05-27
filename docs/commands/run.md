# `chant run`

> Execute a recipe's procedure. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdRun`) +
> [`internal/runner/runner.go`](../../internal/runner/runner.go).

## What it does

Adapts the recipe's `what_to_do.command` by substituting `{{var}}` placeholders
from `--input k=v` values, then runs it in the recipe's own directory via
`sh -c`. Inputs are also exposed as `CHANT_INPUT_<KEY>` environment variables.
The run is logged under `.chant/runs/<timestamp>/run.json`.

`run` **never sets `trusted`.** Running a procedure is not reuse — it produces
output but makes no trust claim. The payload always reports `trusted: false` and
recommends `chant verify <id>` as the next step. If a `{{var}}` has no matching
input the command fails fast rather than running half-formed.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--input k=v` | none | repeatable; fills `{{k}}` placeholders and sets `CHANT_INPUT_K`. |
| `--timeout` | `60s` | command timeout (Go duration). |
| `--json` | false | emit the JSON outcome contract. |

The recipe id is a positional argument; flags work in any position
(`chant run <id> --json` and `chant run --json <id>` are equivalent).

## Example (human)

```bash
chant run --input input=examples/orders_shopify.csv csv-revenue-by-channel
```

```text
{
  "direct": 25.5,
  "facebook": 200.0,
  "google": 150.0
}

[ran csv-revenue-by-channel in 52ms, exit 0] — run `chant verify csv-revenue-by-channel` to establish trust.
```

## Example (JSON)

```bash
chant run csv-revenue-by-channel --input input=examples/orders_shopify.csv --json
```

```json
{
  "subcommand": "run",
  "match_found": false,
  "recipe_id": "csv-revenue-by-channel",
  "version": 1,
  "executed": true,
  "exit_code": 0,
  "trusted": false,
  "runtime_ms": 70,
  "recommended_next_command": "chant verify csv-revenue-by-channel"
}
```

In human mode, recipe stdout is printed to stdout and stderr to stderr; a
non-zero exit code makes `chant run` exit 1.

## JSON shape

`subcommand: "run"`, `match_found` (always present, `false`), `recipe_id`,
`version`, `executed: true`, `exit_code`, `trusted: false`, `runtime_ms`, and
`recommended_next_command` (`chant verify <id>`). A command-level failure
(unknown recipe, unresolved `{{var}}`) returns the error contract instead:
`{"subcommand": "run", "blocking_error": true, "message"}` on stdout, exit 1.
See the [outcome contract](../README.md#json-outcome-contract).

## When to use `run` vs `verify`

Use `verify` for reuse — it runs the procedure *and* the verifier and is the
only command that can report `trusted: true`. Use `run` when you want the raw
output without the verifier (debugging, inspecting a procedure), accepting that
the result is unverified.
