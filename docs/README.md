# chant Command Reference

This folder is the long-form reference for **every command** chant ships.
Each page tells you the same things:

1. **What it does** — the job the command performs in the recipe lifecycle.
2. **Flags** — every flag the command accepts, with defaults.
3. **Example invocation with real output** — copy-pasted from the actual binary.
4. **The JSON shape** — the `--json` outcome contract, where applicable.

The goal is for a reader who's never seen the codebase to pick any command and
understand what it does, what it emits, and how it fits the verifier-first reuse
loop.

A chant recipe is also called an **enchantment** (synonym; the on-disk form is
`recipe.yaml`, the code type is `recipe.Recipe`).

---

## Layout

```
docs/
├── README.md            ← you are here
├── commands/            ← one page per CLI command
└── specs/               ← design specs (owned by the lead; see enchantment-metadata.md)
```

---

## The reuse lifecycle

chant's commands group into the agent hook surface, the library, and repo
management. The lifecycle is **verifier-first**:

```text
suggest → (reuse: verify) → accept     ← before writing code, find + trust a recipe
   │
   └── miss → solve → capture → verify  ← after solving, cache the method
```

A retrieved recipe is a *candidate*; a reuse result is `trusted` only after its
verifier passes.

### Lifecycle (the agent hook surface)

| Command | One-liner | Page |
|---------|-----------|------|
| `suggest` | Find a reusable recipe before writing new code (`--global` adds the registry). | [`commands/suggest.md`](commands/suggest.md) |
| `capture` | Distill solved work into a verified recipe. | [`commands/capture.md`](commands/capture.md) |
| `import` | Copy a foreign enchantment from the registry into `recipes/` (then verify). | [`commands/import.md`](commands/import.md) |
| `run` | Execute a recipe's procedure (never sets `trusted`). | [`commands/run.md`](commands/run.md) |
| `verify` | Run the procedure + verifier; only a pass is `trusted`. | [`commands/verify.md`](commands/verify.md) |

### Library

| Command | One-liner | Page |
|---------|-----------|------|
| `list` | List recipes with version + success rate. | [`commands/list.md`](commands/list.md) |
| `search` | Rank every recipe against a query. | [`commands/search.md`](commands/search.md) |
| `explain` | Print a recipe card. | [`commands/explain.md`](commands/explain.md) |
| `invalidate` | Mark a recipe stale. | [`commands/invalidate.md`](commands/invalidate.md) |

### Repo

| Command | One-liner | Page |
|---------|-----------|------|
| `init` | Scaffold `chant.yml`, `recipes/`, skill, gitignore. | [`commands/init.md`](commands/init.md) |
| `index` | Rebuild `.chant/index.json` and upsert into the per-machine registry (`--no-registry` to skip). | [`commands/index.md`](commands/index.md) |
| `status` | Rewrite `.chant/STATUS.md`. | [`commands/status.md`](commands/status.md) |
| `doctor` | Validate config + store. | [`commands/doctor.md`](commands/doctor.md) |
| `bench` | Run the validation suite. | [`commands/bench.md`](commands/bench.md) |

---

## The verifier-first trust gate

This is the principle every page returns to. A reuse result is "trusted" ONLY
after its verifier passes:

```text
retrieve → adapt → execute → verify → accept     (chant)
retrieve → trust                                  (the failure mode chant avoids)
```

In the JSON outcome contract this surfaces as a single boolean: `trusted`. It is
`true` **only** in a `verify` payload after a passing verifier. `suggest`,
`search`, `run`, and `import` always report `trusted: false` — they retrieve,
execute, or stage, but they do not bless. Reuse from another repo follows the
same rule: a foreign hit is `import`ed locally, then verified — never trusted on
import.

---

## JSON outcome contract

Every command accepts `--json` and emits a stable top-level vocabulary so
pre-commit hooks and agents can decide what to do next without parsing prose.
The full field table is in the [project README](../README.md#json-outcome-contract);
the source of truth is
[`internal/outcome/outcome.go`](../internal/outcome/outcome.go). Unset fields
are omitted so each subcommand's payload stays focused. Flags work in any
position — before or after a positional recipe id. Errors under `--json` are
part of the contract too: a failing command prints
`{"subcommand", "blocking_error": true, "message"}` to stdout and exits 1.

---

## Enchantment metadata & reuse

`chant capture` writes a fully backward-compatible metadata layer onto every
recipe card. **Available now**, emitted automatically: a content-addressed
`spell_hash` identity, `provenance` (`origin` / `captured_at` / `author`),
`scope: project` (the first rung of the universality ladder), and `portability`
(`determinism`, `input_contract.required_columns_any` from `--columns`,
`requires.runtime` from `--language`). Two capture flags drive it: `--author`
and `--columns`.

This metadata powers **cross-package reuse**, also **available now**: a
per-machine registry (`$CHANT_REGISTRY`, default `~/.chant/registry/index.json`)
that `chant index` upserts into, `chant suggest --global` for discovering foreign
enchantments, and `chant import` for copying one into the local library (then
verifying it). **Still planned:** scope promotion (`project → domain →
universal`) and typed relations reusing coherence's edge vocabulary surfaced as
graph edges. The canonical design is
[`specs/enchantment-metadata.md`](specs/enchantment-metadata.md).

---

## Bench

Every page's behavior is exercised by `chant bench`, which ships two suites:

- **retrieval** — synthetic recipe set + queries asserting which recipe ranks
  first (including true negatives: an unrelated query must NOT match).
- **e2e** — runs each recipe's procedure + verifier and asserts the
  verifier-first gate (trusted only after the verifier passes).

```bash
chant bench --suite=all
```

Exit code is `1` when any scenario fails — same CI muscle memory as
`coherence bench`. See [`commands/bench.md`](commands/bench.md).
