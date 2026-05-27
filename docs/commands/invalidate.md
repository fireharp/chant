# `chant invalidate`

> Mark a recipe stale. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdInvalidate`).

## What it does

Flags a recipe `status: stale` and rebuilds the index. A stale recipe is **still
retrievable** — `suggest` and `search` still surface it — but it is scored at
half weight so an active alternative ranks above it, and its hits carry the
reason `"recipe is stale — re-verify before trusting"`. A passing `chant verify`
re-blesses a stale recipe back to `active`.

Use `invalidate` when a recipe stops working: its inputs drifted, a dependency
changed, or its verifier started failing. It is the demotion half of the
lifecycle; `verify` is the (re-)promotion half.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--reason` | empty | a note appended to the status message explaining why. |
| `--json` | false | emit the JSON outcome contract. |

The recipe id is the positional argument. **Put flags before the id**
(`chant invalidate --json <id>`) — Go's `flag` package stops parsing at the
first non-flag argument.

## Example (JSON)

```bash
chant invalidate --json hello-echo
```

```json
{
  "subcommand": "invalidate",
  "recipe_id": "hello-echo",
  "exit_code": 0,
  "trusted": false,
  "stale": true,
  "message": "recipe marked stale — a passing `chant verify` will re-bless it"
}
```

## Example (human)

```bash
chant invalidate hello-echo --reason "demo"
```

```text
marked hello-echo stale. recipe marked stale — a passing `chant verify` will re-bless it (demo)
```

## JSON shape

`subcommand: "invalidate"`, `recipe_id`, `stale: true`, `trusted: false`, and
`message` (with the `--reason` appended in parentheses when given). See the
[outcome contract](../README.md#json-outcome-contract).

## Lifecycle

```text
active ──invalidate──▶ stale ──verify (pass)──▶ active
```

Invalidation is a soft demotion, not a delete. The recipe card stays on disk and
in the index; only its `status` changes. To remove a recipe entirely, delete its
directory under `recipes/` and re-run `chant index`.
