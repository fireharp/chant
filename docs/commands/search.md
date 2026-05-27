# `chant search`

> Rank every recipe against a query. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdSearch`) +
> [`internal/retrieve/retrieve.go`](../../internal/retrieve/retrieve.go).

## What it does

Ranks the whole library against a free-text query and prints the scored results.
Unlike `suggest`, `search` does **not** apply the retrieval `threshold` — it
shows every recipe with its blended score and the per-term breakdown (lexical,
signal, success rate). Use it to inspect ranking and debug why a recipe does or
doesn't surface for a query.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--json` | false | emit hits as the JSON outcome contract. |

The query is the positional argument(s). **Put `--json` before the query**
(`chant search --json "revenue by channel"`) — Go's `flag` package stops parsing
at the first non-flag argument, so a trailing `--json` is absorbed into the query
text and ignored.

## Example (human)

```bash
chant search "revenue by channel"
```

```text
ranked recipes for "revenue by channel":
  0.70  csv-revenue-by-channel         (lex 1.00, signal 0.00, ok 1.00)
```

The breakdown columns are the three scorer terms: `lex` (lexical overlap),
`signal` (file/column structural match — `0.00` here because `search` passes no
files/columns), and `ok` (verifier success rate).

## Example (JSON)

```bash
chant search --json "revenue by channel"
```

```json
{
  "subcommand": "search",
  "hits": [
    {
      "id": "csv-revenue-by-channel",
      "version": 1,
      "description": "Compute ecommerce revenue by channel from CSV-like exports, robust to column-name drift across Shopify/Stripe/custom exports.",
      "confidence": 0.7,
      "status": "active",
      "verifier_exists": true,
      "reasons": [
        "task text overlaps recipe description/patterns"
      ],
      "reuse_command": "chant verify csv-revenue-by-channel   # run + verify before trusting"
    }
  ],
  "exit_code": 0,
  "trusted": false
}
```

## JSON shape

`subcommand: "search"`, `hits[]` (every recipe, ranked, same `Hit` shape as
`suggest`), and `trusted: false`. There is no `match_found` field — `search`
does not gate on the threshold. See the
[outcome contract](../README.md#json-outcome-contract).

## `search` vs `suggest`

| | `search` | `suggest` |
|---|---|---|
| threshold filtering | no — ranks everything | yes — only hits ≥ threshold |
| takes files/columns | no | yes (`--files`, `--columns`) |
| intended use | inspecting / debugging ranking | the agent pre-write hook |
| `match_found` field | absent | present |
