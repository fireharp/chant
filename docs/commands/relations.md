# `chant relations`

> List a recipe's typed lineage edges — supersession, mirroring, dependencies,
> implementation links. The third pillar of enchantment metadata (after
> `spell_hash` identity and the cross-package registry). Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdRelations`).

## What it does

Relations are the typed edges between enchantments and the things they relate
to: previous versions, equivalents in other languages, the data/config they
depend on, the user-stories or policies they fulfil. They are set at capture
time (six new flags on `chant capture` — see below) and surfaced by this
command.

The relation kinds, mirroring [coherence's edge
vocabulary](../specs/enchantment-metadata.md) (spec §3, §7):

| kind          | meaning                                                                    |
| ------------- | -------------------------------------------------------------------------- |
| `supersedes`  | this enchantment replaces those.                                            |
| `mirrors`     | same procedure, different language or package.                              |
| `generalizes` | this is a broader form of those.                                            |
| `specializes` | this is a narrower form of those.                                           |
| `depends_on`  | the data or config this enchantment requires (e.g. `data:orders-schema`). |
| `implements`  | the story/policy ids this enchantment fulfils (e.g. `US-014`).             |

## Flags

| Flag        | Default | Meaning                                                                            |
| ----------- | ------- | ---------------------------------------------------------------------------------- |
| `--inverse` | false   | flip direction: show which OTHER local recipes have an edge pointing at `<id>`.    |
| `--json`    | false   | emit the JSON outcome contract.                                                    |

Flags work in any position (e.g. `chant relations <id> --inverse --json`).
Unknown ids → the error contract (`blocking_error: true`, exit 1). Exit 0
always for known ids — `relations` is read-only.

## Examples

### Outgoing (default)

```bash
chant relations refund-approval-v2
```

```text
4 outgoing relation(s) from refund-approval-v2:
  supersedes   → refund-approval         [resolved]
  mirrors      → refund-approval-go      [dangling]
  depends_on   → data:returns-schema     [dangling]
  implements   → US-014                  [dangling]
```

`resolved` means the target id is a recipe in the local library; `dangling` is
a forward reference (typed-id, cross-package, etc.) that the local store does
not currently resolve. Both are valid — `doctor` surfaces the dangling count
as a `warn` so you can decide whether to import / capture / fix them.

### Outgoing as JSON

```bash
chant relations refund-approval-v2 --json
```

```json
{
  "subcommand": "relations",
  "recipe_id": "refund-approval-v2",
  "version": 2,
  "match_found": false,
  "trusted": false,
  "exit_code": 0,
  "outgoing": [
    { "kind": "supersedes", "target_id": "refund-approval", "resolved": true },
    { "kind": "mirrors",    "target_id": "refund-approval-go", "resolved": false },
    { "kind": "depends_on", "target_id": "data:returns-schema", "resolved": false },
    { "kind": "implements", "target_id": "US-014", "resolved": false }
  ]
}
```

### Inverse — "what relates to me?"

```bash
chant relations refund-approval --inverse
```

```text
1 incoming relation(s) to refund-approval:
  supersedes   ← refund-approval-v2
```

The same shape under `--json` populates the `incoming` field instead of
`outgoing`. Each incoming edge is `{kind, target_id, resolved}` where
`target_id` is the SOURCE recipe id (the one that declares the edge pointing
at us); `resolved` is always `true` because the inverse scan only walks
local recipes. `--inverse` is what you read when deciding whether removing a
recipe is safe — anything pointing AT it is about to dangle.

## Setting relations at capture time

`chant capture` accepts six new comma-separated flags. They populate the
recipe's `relations.<kind>` block in `recipe.yaml`:

| Flag             | Maps to                       |
| ---------------- | ----------------------------- |
| `--supersedes`   | `relations.supersedes: [...]`  |
| `--mirrors`      | `relations.mirrors: [...]`     |
| `--generalizes`  | `relations.generalizes: [...]` |
| `--specializes`  | `relations.specializes: [...]` |
| `--depends-on`   | `relations.depends_on: [...]`  |
| `--implements`   | `relations.implements: [...]`  |

Example:

```bash
chant capture --id refund-approval-v2 \
  --task "approve refunds — v2 with new policy" \
  --command "..." --verifier "..." \
  --supersedes refund-approval \
  --mirrors refund-approval-go \
  --depends-on "data:returns-schema" \
  --implements "US-014"
```

You can also edit the `relations:` block in `recipe.yaml` directly — these
flags are a convenience over the existing model.

## How it interacts with the rest of chant

- **Verifier-first**: `relations` is read-only and never sets `trusted: true`.
  An incoming/outgoing edge is information, not blessing.
- **Cross-package**: a target id can be a foreign-enchantment id (or
  `spell_hash`). `relations` doesn't try to resolve foreign refs — it reports
  them as `dangling`, and you `chant import` them when you want them locally.
- **Doctor**: `chant doctor` adds a `relations` check that warns when local
  recipes have dangling targets, listing up to a few examples. It is a
  `warn` (informational), not a `fail` — dangling is often deliberate.
- **Versioning parallel**: `supersedes` + `lineage_id` (capture-populated)
  together encode the chant equivalent of semver lineage, but for
  *procedures* rather than packages.

## JSON shape

`subcommand: "relations"`, `recipe_id`, `version`, `trusted: false`,
`match_found: false`, plus exactly one of:

- `outgoing: [{kind, target_id, resolved}, …]` (default)
- `incoming: [{kind, target_id, resolved}, …]` (with `--inverse`, where
  `target_id` is the source recipe id of the edge)

Empty list when the recipe declares no relations / nothing points at it. A
failure (unknown id) returns the standard error contract on stdout, exit 1.

## See also

- [`docs/specs/enchantment-metadata.md`](../specs/enchantment-metadata.md) §3 + §7 — the relation vocabulary + coherence mapping.
- [`capture`](capture.md) — set relations at capture time.
- [`doctor`](doctor.md) — surfaces dangling relation targets as `warn`.
- [`import`](import.md) — resolve a foreign target into the local library.
