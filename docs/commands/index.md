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

It then **upserts the local library into the per-machine registry** — the shared
index that powers cross-repo discovery (`chant suggest --global` and
`chant import`). The registry lives at `$CHANT_REGISTRY`, defaulting to
`~/.chant/registry/index.json`. Each enchantment is keyed by its `spell_hash`,
so re-indexing updates the existing entry rather than duplicating it. Pass
`--no-registry` to skip the upsert. The registry write degrades gracefully: if
it isn't writable, indexing still succeeds and the failure is reported in
`registry_warning` rather than erroring.

Most commands that mutate the library (`capture`, `verify`, `invalidate`,
`list`) refresh the local index automatically; run `index` explicitly after
editing `recipe.yaml` files by hand, and to publish the library to the registry.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--no-registry` | false | skip upserting the library into the per-machine registry. |
| `--json` | false | emit the JSON outcome contract to stdout. |

## Example (human)

```bash
chant index
```

```text
indexed 1 recipe(s) → /path/to/repo/.chant/index.json
upserted 1 enchantment(s) into the registry → /home/you/.chant/registry/index.json
```

With `--no-registry` the upsert line is omitted:

```bash
chant index --no-registry
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
  "subcommand": "index",
  "count": 1,
  "index_path": "/path/to/repo/.chant/index.json",
  "registry_upserted": 1,
  "registry_warning": ""
}
```

If the registry isn't writable, indexing still succeeds and the reason is
surfaced (and `registry_upserted` stays `0`):

```json
{
  "subcommand": "index",
  "count": 1,
  "index_path": "/path/to/repo/.chant/index.json",
  "registry_upserted": 0,
  "registry_warning": "mkdir /this: read-only file system"
}
```

## JSON shape

`subcommand: "index"`, `count` (recipes indexed), `index_path` (the
`.chant/index.json` written), `registry_upserted` (how many enchantments were
written to the registry — `0` with `--no-registry` or on a registry error), and
`registry_warning` (empty on success; the failure reason otherwise). See the
[outcome contract](../README.md#json-outcome-contract).

> The flattened recipe summary (`generated_at`, `count`, `recipes[]`) that this
> command writes to `.chant/index.json` is emitted to stdout by
> [`chant list --json`](list.md), not by `chant index --json`.
