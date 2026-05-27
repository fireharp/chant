# `chant index`

> Rebuild the retrieval index. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdIndex`) +
> [`internal/store/store.go`](../../internal/store/store.go) (`WriteIndex`).

## What it does

Walks the recipe library under `recipes/`, loads every `recipe.yaml`, and
rewrites `.chant/index.json` — a flattened, fast-to-read summary of every recipe
(id, version, description, status, tags, run/failure counts, success rate). The
index is the data `chant list` prints and a convenient artifact for external
tooling.

Most commands that mutate the library (`capture`, `verify`, `invalidate`,
`list`) refresh the index automatically; run `index` explicitly after editing
`recipe.yaml` files by hand.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--json` | false | emit the index JSON to stdout. |

## Example (human)

```bash
chant index
```

```text
indexed 1 recipe(s) → /path/to/repo/.chant/index.json
```

## Example (JSON)

```bash
chant index --json
```

```json
{
  "generated_at": "2026-05-27T21:13:02Z",
  "count": 1,
  "recipes": [
    {
      "id": "csv-revenue-by-channel",
      "version": 1,
      "description": "Compute ecommerce revenue by channel from CSV-like exports, robust to column-name drift across Shopify/Stripe/custom exports.",
      "status": "active",
      "tags": ["csv", "ecommerce", "revenue", "analytics"],
      "runs": 1,
      "failures": 0,
      "success_rate": 1
    }
  ]
}
```

## JSON shape

The index shape: `generated_at`, `count`, and `recipes[]` where each entry is an
`IndexEntry` (`id`, `version`, `description`, `status`, `tags[]`, `runs`,
`failures`, `success_rate`). This is the same shape `chant list --json` emits.
The file is written to `.chant/index.json` (gitignored runtime state).
