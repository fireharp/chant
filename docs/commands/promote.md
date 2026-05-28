# `chant promote`

> Recompute a recipe's scope from its current `verified_in` evidence. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdPromote`) +
> [`internal/recipe/scope.go`](../../internal/recipe/scope.go) (`ComputeScope`).

## What it does

`chant promote` reads a recipe's `verified_in` list + `domains` and recomputes
its `scope` per the spec's universality ladder (see
[`docs/specs/enchantment-metadata.md`](../specs/enchantment-metadata.md) §5):

| scope       | rule                                                                              |
|-------------|-----------------------------------------------------------------------------------|
| `project`   | default; verified in 0 or 1 distinct contexts (or no `domains` declared).         |
| `domain`    | verified in ≥2 distinct contexts AND ≥1 `domains` tag.                            |
| `universal` | verified in ≥3 distinct contexts AND `domains` spans ≥2 labels.                   |

Promotion is **earned, never declared**: the evidence comes from
`chant verify` passing in distinct contexts. `chant promote` is a *recompute*,
not a re-verify — it does not re-run the verifier. Use it after you've edited
`domains:` on a card, or to backfill `scope:` on recipes captured before
scope promotion landed.

Like the auto-promotion that runs on `chant verify`, `promote` never lowers
`scope`: an already-`universal` recipe keeps `universal` even if its
`verified_in` has been pruned. Explicit demotion is the job of
`chant invalidate` and a future `DemoteScope` path.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--json` | false | emit the JSON outcome contract. |

The recipe id is a positional argument; flags work in any position.

## Example — promote after editing domains

```bash
# Recipe card now declares domains: [csv, ecommerce] and has 3 verified_in.
chant promote csv-revenue-by-channel
# → csv-revenue-by-channel scope promoted: project → universal (recomputed from 3 context(s))
```

## Example — JSON

```bash
chant promote scopey --json
```

```json
{
  "subcommand": "promote",
  "recipe_id": "scopey",
  "version": 1,
  "exit_code": 0,
  "trusted": false,
  "scope_promotion": {
    "old": "project",
    "new": "domain",
    "contexts": 2
  },
  "scope": "domain",
  "old_scope": "project",
  "contexts_count": 2,
  "message": "scope promoted: project → domain (recomputed from 2 context(s))"
}
```

When the recipe's scope is already the highest its evidence can earn,
`scope_promotion` is omitted and `scope == old_scope`. Exit code is `0` in both
cases — `promote` is read-and-recompute, not a gate.

## Auto-promotion on verify

You usually don't need to run `chant promote` explicitly. `chant verify`
records the current context (env `CHANT_CONTEXT` overrides; otherwise the git
`remote.origin.url`, normalized to `host/owner/repo`, else the absolute repo
path) into `verified_in` on a passing verifier, and auto-promotes the scope.
`promote` is the manual rebuild for the rare cases auto-promotion can't reach:
domain edits, backfills, or imported recipes whose evidence carried over.

## Related

- [`chant verify`](verify.md) — the trust gate that earns evidence.
- [`chant invalidate`](invalidate.md) — the demotion path (marks the recipe
  stale; a passing re-verify re-blesses it).
- [Spec §5: Scope & the universality ladder](../specs/enchantment-metadata.md).
