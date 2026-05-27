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

The recipe id is the positional argument. **Put `--json` before the id**
(`chant explain --json <id>`) — Go's `flag` package stops parsing at the first
non-flag argument.

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
  "ID": "csv-revenue-by-channel",
  "Version": 1,
  "Kind": "executable_recipe",
  "Description": "Compute ecommerce revenue by channel from CSV-like exports, robust to column-name drift across Shopify/Stripe/custom exports.",
  "WhenToUse": {
    "TaskPatterns": [
      "compute revenue by channel from csv",
      "analyze ecommerce orders export",
      "revenue breakdown by marketing channel"
    ],
    "Tags": ["csv", "ecommerce", "revenue", "analytics"],
    "InputSignals": {
      "Files": ["*.csv"],
      "ColumnsAny": [
        ["channel", "source", "utm_source"],
        ["revenue", "amount", "price", "total"]
      ]
    }
  },
  "WhatToDo": {
    "Entrypoint": "run.py",
    "Command": "python3 run.py {{input}}",
    "Language": "python"
  },
  "Verification": {
    "Command": "python3 test_recipe.py",
    "ExpectedArtifacts": ["revenue_by_channel.json"]
  },
  "Invalidation": {"IfTestsFail": true, "IfColumnsMissing": true, "IfDependencyChange": false},
  "Dependencies": {"Runtime": "python: >=3.8", "Packages": null},
  "Fingerprints": {"RecipeCodeHash": "", "VerifierHash": "", "SchemaFingerprint": ""},
  "Examples": [{"Input": "examples/orders_shopify.csv", "Output": "revenue_by_channel.json"}],
  "Metrics": {"Runs": 4, "Failures": 0, "LastSuccessAt": "2026-05-27T21:12:50Z", "LastFailureAt": ""},
  "Status": "active"
}
```

## JSON shape

> **Note:** `explain --json` serializes the raw Go `recipe.Recipe` struct, whose
> fields carry only `yaml` tags. JSON marshalling therefore uses the Go field
> names (`ID`, `WhenToUse`, `WhatToDo`, …) rather than the `snake_case` keys you
> see in `recipe.yaml`. This is the one command whose JSON does **not** follow
> the `snake_case` outcome-contract convention. To read the card in its on-disk
> form, open the `recipe.yaml` directly or use `chant list --json` for the
> indexed summary.
