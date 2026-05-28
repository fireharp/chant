# `chant suggest`

> The pre-write hook. Before an agent writes new code, ask whether a recipe
> already solves the task. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdSuggest`) +
> [`internal/retrieve/retrieve.go`](../../internal/retrieve/retrieve.go).

## What it does

Ranks the recipe library against a natural-language task (plus optional input
file names and column names) using the deterministic hybrid scorer, and reports
every candidate at or above the configured retrieval `threshold`. Each hit
carries a `verifier_exists` flag and a `reuse_command` so the agent can reuse
the recipe verifier-first. A hit is a **candidate**, never trusted —
`suggest` always reports `trusted: false`.

With `--global`, `suggest` also searches the per-machine registry and returns
**foreign** enchantments — ones born in other repos (see
[Foreign hits](#example--foreign-hit-with---global-json) below).

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--task` | *(required)* | natural-language task description. |
| `--files` | empty | comma-separated input file names/paths, matched against each recipe's `input_signals.files` globs. |
| `--columns` | empty | comma-separated available column names, matched against each recipe's `input_signals.columns_any` alias groups. |
| `--global` | false | also search the per-machine registry (`$CHANT_REGISTRY`, default `~/.chant/registry/index.json`) for foreign enchantments. |
| `--json` | false | emit the JSON outcome contract. |

## Example — hit (JSON)

```bash
chant suggest --task "compute revenue by channel" \
  --files orders.csv --columns utm_source,amount --json
```

```json
{
  "subcommand": "suggest",
  "match_found": true,
  "hits": [
    {
      "id": "csv-revenue-by-channel",
      "version": 1,
      "description": "Compute ecommerce revenue by channel from CSV-like exports, robust to column-name drift across Shopify/Stripe/custom exports.",
      "confidence": 1,
      "status": "active",
      "verifier_exists": true,
      "reasons": [
        "task text overlaps recipe description/patterns",
        "input files match recipe file signal",
        "input columns satisfy recipe column aliases"
      ],
      "reuse_command": "chant verify csv-revenue-by-channel   # run + verify before trusting"
    }
  ],
  "exit_code": 0,
  "trusted": false,
  "recommended_next_command": "chant verify csv-revenue-by-channel   # run + verify before trusting"
}
```

## Example — hit (human)

```bash
chant suggest --task "compute revenue by channel" --files orders.csv --columns utm_source,amount
```

```text
1 recipe candidate(s) for "compute revenue by channel":
  • csv-revenue-by-channel       v1  confidence 1.00  [verifier available]
      reuse: chant verify csv-revenue-by-channel   # run + verify before trusting
```

## Example — miss (human)

```bash
chant suggest --task "rotate kubernetes TLS certificates"
```

```text
no recipe matched "rotate kubernetes TLS certificates" above threshold 0.25
→ solve the task, then capture it with `chant capture`.
```

On a miss the JSON payload sets `match_found: false` and
`recommended_next_command: "no recipe matched — solve the task, then `chant
capture` it"`.

## Example — foreign hit with `--global` (JSON)

When no local recipe matches but the registry holds one captured in another repo
(the `greet` enchantment here), `--global` surfaces it as a **foreign** hit:

```bash
chant suggest --task "greet a customer by name" --global --json
```

```json
{
  "subcommand": "suggest",
  "match_found": true,
  "hits": [
    {
      "id": "greet",
      "version": 1,
      "description": "greet a name politely",
      "confidence": 0.53,
      "verifier_exists": true,
      "reasons": ["foreign enchantment from registry — import then verify before trusting"],
      "reuse_command": "chant import ff9a7d644ac15c3d   # copy locally, then `chant verify` before trusting",
      "global": true,
      "origin": "/abs/path/to/origin/repo",
      "scope": "project",
      "spell_hash": "ff9a7d644ac15c3d"
    }
  ],
  "exit_code": 0,
  "trusted": false,
  "recommended_next_command": "chant import ff9a7d644ac15c3d   # copy locally, then `chant verify` before trusting"
}
```

A foreign hit carries `global: true`, plus `origin`, `scope`, and `spell_hash`,
and its `reuse_command` is a `chant import` (not `chant verify`) — a foreign
enchantment must be imported locally before its verifier can run here. The human
form annotates the hit `(foreign: <origin>, scope <scope>)`. `trusted` stays
`false`.

## JSON shape

`subcommand: "suggest"`, `match_found` (bool), `hits[]` (ranked candidates, each
with `id`, `version`, `description`, `confidence`, `status`, `verifier_exists`,
`reasons[]`, `reuse_command`; foreign hits add `global`, `origin`, `scope`,
`spell_hash`), `trusted: false`, and `recommended_next_command`. See the
[outcome contract](../README.md#json-outcome-contract).

## How ranking works

```text
score = weight_lexical      * lexical(task, description + patterns + tags)
      + weight_tags         * signal_match(files/columns vs input_signals)
      + weight_success_rate * verifier_success_rate
```

Weights and `threshold` come from `chant.yml` (defaults `0.5 / 0.3 / 0.2`,
threshold `0.25`). Stale recipes are included but scored at half weight so an
active alternative ranks first. `suggest` filters to hits ≥ threshold; use
`chant search` to rank everything regardless of threshold.

## Agent loop

Read `match_found` → take `hits[0]` → if `verifier_exists`, run its
`reuse_command` → trust the result only when the `verify` payload reports
`"trusted": true`. On a miss, solve the task and `chant capture` it.

For a foreign hit (`global: true`), the `reuse_command` is a `chant import` —
run it to copy the enchantment locally, **then** `chant verify` the imported
recipe. Import stages; the local verifier blesses.
