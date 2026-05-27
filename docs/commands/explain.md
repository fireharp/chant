# `chant explain`

> Print a recipe card. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdExplain`).

## What it does

Loads a single recipe by id and prints its card: description, when-to-use
patterns and tags, the procedure command, the verifier command, and the run
metrics. With `--json` it emits the full recipe struct.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--json` | false | emit the full recipe struct as JSON. |

The recipe id is the positional argument; flags work in any position
(`chant explain <id> --json` and `chant explain --json <id>` are equivalent).

## Example (human)

```bash
chant explain csv-revenue-by-channel
```

```text
# csv-revenue-by-channel (v1) — active

Compute ecommerce revenue by channel from CSV-like exports, robust to column-name drift across Shopify/Stripe/custom exports.

when to use:
  - compute revenue by channel from csv
  - analyze ecommerce orders export
  - revenue breakdown by marketing channel
tags: csv, ecommerce, revenue, analytics

what to do:
  python3 run.py {{input}}

verify with:
  python3 test_recipe.py

metrics: 1 run(s), 0 failure(s), 100% success
```

If the recipe has no verifier, the `verify with:` block is replaced by
`⚠ no verifier — reuse cannot be trusted.`

## Example (JSON)

```bash
chant explain --json csv-revenue-by-channel
```

```json
{
  "dependencies": {
    "runtime": "python: >=3.8"
  },
  "description": "Compute ecommerce revenue by channel from CSV-like exports, robust to column-name drift across Shopify/Stripe/custom exports.",
  "examples": [
    {
      "input": "examples/orders_shopify.csv",
      "output": "revenue_by_channel.json"
    }
  ],
  "id": "csv-revenue-by-channel",
  "invalidation": {
    "if_columns_missing": true,
    "if_tests_fail": true
  },
  "kind": "executable_recipe",
  "metrics": {
    "last_success_at": "2026-05-27T21:34:59Z",
    "runs": 1
  },
  "status": "active",
  "verification": {
    "command": "python3 test_recipe.py",
    "expected_artifacts": ["revenue_by_channel.json"]
  },
  "version": 1,
  "what_to_do": {
    "command": "python3 run.py {{input}}",
    "entrypoint": "run.py",
    "language": "python"
  },
  "when_to_use": {
    "input_signals": {
      "columns_any": [
        ["channel", "source", "utm_source"],
        ["revenue", "amount", "price", "total"]
      ],
      "files": ["*.csv"]
    },
    "tags": ["csv", "ecommerce", "revenue", "analytics"],
    "task_patterns": [
      "compute revenue by channel from csv",
      "analyze ecommerce orders export",
      "revenue breakdown by marketing channel"
    ]
  }
}
```

## JSON shape

`explain --json` emits the recipe card in its on-disk `snake_case` form — it
round-trips through the same representation as `recipe.yaml`, so the JSON keys
match the card's YAML keys (`id`, `when_to_use`, `what_to_do`, `verification`,
…). Empty/zero fields are omitted (e.g. a recipe with no recorded failures has
no `failures` key, and an empty `fingerprints` block is dropped). For the
indexed summary instead of the full card, use `chant list --json`.
