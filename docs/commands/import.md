# `chant import`

> Copy a foreign enchantment from the per-machine registry into the local
> library, so you can verify and reuse it here. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdImport`).

## What it does

Looks up an enchantment in the per-machine registry by **id** or **spell_hash**,
copies its recipe directory into the local `recipes/`, and rebuilds the index.
Import is the bridge that makes a foreign hit from `chant suggest --global`
reusable in the current repo.

Import **stages, never blesses.** The copied recipe arrives `trusted: false`;
running its verifier *in this repo* is what establishes trust. This is the
cross-repo half of chant's verifier-first rule:

```text
suggest --global → import → verify → accept     (cross-repo reuse)
import → trust                                   (the failure mode chant avoids)
```

A foreign enchantment is the most suspect kind of hit — it was verified in
*another* repo, against *that* context. `import` brings the method over; the
local verifier decides whether it actually works here.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--as <newid>` | source id | import under a different local recipe id (lets you keep a local recipe of the same name). |
| `--force` | false | overwrite an existing local recipe of the same id. |
| `--json` | false | emit the JSON outcome contract. |

The id or spell_hash is the positional argument; flags work in any position.
Import refuses to overwrite an existing local recipe without `--force` (the error
message points you at `--force` or `--as`).

## Where it imports from

The registry lives at `$CHANT_REGISTRY`, defaulting to
`~/.chant/registry/index.json`. It is populated by `chant index` (which upserts
the local library on every run unless `--no-registry` is passed). If the
enchantment you ask for isn't in the registry, run `chant index` in its origin
repo first.

## Example — import by spell_hash (JSON)

```bash
chant import ff9a7d644ac15c3d --json
```

```json
{
  "subcommand": "import",
  "match_found": false,
  "recipe_id": "greet",
  "version": 1,
  "exit_code": 0,
  "trusted": false,
  "message": "imported \"greet\" from /abs/path/to/origin/repo — NOT trusted yet; run its verifier in this repo",
  "recommended_next_command": "chant verify greet"
}
```

Then verify it locally to establish trust:

```bash
chant verify greet --input name=world
# → ✓ greet verified — trusted
```

## Example — import under a new id (human)

```bash
chant import greet --as greet2
```

```text
imported "greet2" from /abs/path/to/origin/repo → /path/to/repo/recipes/greet2
⚠ foreign enchantment — NOT trusted. Run `chant verify greet2` to re-run its verifier here.
```

## Example — refuses to overwrite

```bash
chant import ff9a7d644ac15c3d        # a local "greet" already exists
```

```text
chant: local recipe "greet" already exists (use --force to overwrite, or --as <newid> to import under a different id)
```

Exit code 1. Re-run with `--force` to overwrite, or `--as <newid>` to import
alongside.

## JSON shape

`subcommand: "import"`, `match_found: false`, `recipe_id` (the local id after
`--as` is applied), `version`, `trusted: false`, `message` (names the origin and
that trust is not yet established), and `recommended_next_command`
(`chant verify <id>`). A failure — no such enchantment in the registry, or a
name collision without `--force` — returns the error contract:
`{"subcommand": "import", "blocking_error": true, "message"}` on stdout, exit 1.
See the [outcome contract](../README.md#json-outcome-contract).

## See also

- [`suggest`](suggest.md) — `--global` surfaces foreign enchantments whose
  `reuse_command` is a `chant import`.
- [`index`](index.md) — upserts the local library into the registry that
  `import` reads.
- [`verify`](verify.md) — the trust gate you run after importing.
