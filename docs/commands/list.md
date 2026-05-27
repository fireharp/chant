# `chant list`

> List the recipe library. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdList`) +
> [`internal/store/store.go`](../../internal/store/store.go).

## What it does

Rebuilds `.chant/index.json` from the recipes under `recipes/` and prints each
recipe with its version, run count, success rate, and a `(stale)` flag where
applicable. Listing always refreshes the index as a side effect.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--json` | false | emit the index JSON. |

## Example (human)

```bash
chant list
```

```text
1 recipe(s):
  csv-revenue-by-channel         v1  1 run(s) 100% ok
      Compute ecommerce revenue by channel from CSV-like exports, robust to column-name drift across Shopify/Stripe/custom exports.
```

On an empty library it prints `no recipes yet — capture one with `chant
capture`.`

## Example (JSON)

```bash
chant list --json
```

```json
{
  "generated_at": "2026-05-27T21:12:38Z",
  "count": 1,
  "recipes": [
    {
      "id": "csv-revenue-by-channel",
      "version": 1,
      "description": "Compute ecommerce revenue by channel from CSV-like exports, robust to column-name drift across Shopify/Stripe/custom exports.",
      "status": "active",
      "tags": [
        "csv",
        "ecommerce",
        "revenue",
        "analytics"
      ],
      "runs": 1,
      "failures": 0,
      "success_rate": 1
    }
  ]
}
```

## JSON shape

This command emits the **index** shape (not the standard outcome contract):
`generated_at`, `count`, and `recipes[]` where each entry carries `id`,
`version`, `description`, `status`, `tags[]`, `runs`, `failures`, and
`success_rate`. `success_rate` is `1` for a recipe with no recorded runs (the
benefit of the doubt for a freshly captured recipe).
