# chant — Naive-User Re-Test Report (v0.2)

- **Date:** 2026-05-28
- **Evaluator role:** First-time user re-testing after a previous pass surfaced 5 real issues; the lead claims all fixed plus v0.2 features (enchantment metadata, cross-package registry) shipped.
- **Binary under test:** `/Users/fireharp/Prog/Harness/chant/bin/chant` (copied verbatim to `/tmp/chant-test-binary`, `chant version` → `chant dev`). Not rebuilt.
- **Scratch dirs (all under `/tmp/chant-naive-v0.2/`)**:
  - `repoA.pYJ1/` — primary capture+index repo
  - `repoB.LDOT/` — empty consumer repo for cross-package flow
  - `repoC/`, `repoD/` — fresh import-into-empty repos for second-look checks
  - `readme-walkthrough/` — verbatim execution of the README's `greet` walkthrough
  - `registry.json` — `CHANT_REGISTRY` env override (not `~/.chant/`)
- **Method:** (1) confirm each of the 6 v0.1 findings; (2) exercise the v0.2-only features (metadata via `capture`, cross-package `index`/`suggest --global`/`import`); (3) sanity-check `doctor`/`bench`/the three shipped demos; (4) read README + `docs/integration/bitgn.md` and judge whether a newcomer would actually get to "trusted cross-package reuse" unaided.

---

## TL;DR

- **All 6 v0.1 findings are fixed.** Verified one by one with real CLI output below.
- **v0.2 metadata + cross-package flow works end-to-end** and matches the README's "available now" claims line-for-line.
- **Cross-package flow clarity: 4/5.** The contract is right (foreign hit → `import` → local `verify`, only that establishes trust). One real UX wrinkle: an imported recipe inherits the origin's `metrics.runs` / success-rate, so `chant list` shows `1 run(s) 100% ok` *before* any local verify — readable as "already trusted in this repo," which contradicts the `import` command's own "NOT trusted" warning. See **NEW-1**.
- **Newcomer verdict: yes**, a docs-only newcomer can reach trusted cross-package reuse unaided. The README's `suggest --global` example is verbatim what the binary emits, and the BitGN integration doc is concrete enough to act on.

---

## Per-finding PASS/FAIL/PARTIAL table

| # | Finding (from v0.1 + v0.2 spec) | v0.1 status | This run | Evidence |
|---|---|---|---|---|
| 1 | Scenario A quickstart works as documented (uses `--entrypoint-src`) | FAIL (script missing in recipe dir) | **PASS — fixed** | §1.1 |
| 2 | Flag-order flexibility (`--json` after id works on `verify`/`run`/`explain`/`search`/`invalidate`) | FAIL (docs warned this was broken; was actually fine for some, but undocumented) | **PASS — fixed** | §1.2 |
| 3 | `explain --json` keys are snake_case (`id`, `when_to_use`, `what_to_do`) | docs were stale (tool was already right) | **PASS — fixed (docs + tool agree)** | §1.3 |
| 4 | `--json` error contract: stdout JSON, exit 1, `blocking_error:true`, `message`, `subcommand` | FAIL (`verify --json no-verifier` printed plain text on stderr) | **PASS — fixed** | §1.4 |
| 5 | `match_found` always present on every `--json` payload | PARTIAL (omitted on empty-library `suggest`) | **PASS — fixed** | §1.5 |
| 6 | `capture --json` without a verifier surfaces the warning in `message` + `suggested_commands` | FAIL (no warning under `--json`) | **PASS — fixed** | §1.6 |
| 7 | Enchantment metadata: `spell_hash`, `provenance.{origin,captured_at,author}`, `scope: project`, `portability.{determinism, input_contract.required_columns_any, requires.runtime}` populated by `capture` | n/a | **PASS** | §2.1 |
| 8 | Cross-package flow: `index` upserts; `suggest --global` returns foreign hits with `global:true`, `origin`, `scope`, `spell_hash`, `reuse_command: chant import …`; `import` stages locally; only local `verify` confers trust | n/a | **PASS** (with NEW-1 UX wrinkle) | §2.2 |
| 9 | `import --as <newid>` works; re-import without `--force` is refused with a helpful error; `import --force` overwrites | n/a | **PASS** | §2.3 |
| 10 | `index --no-registry` skips upsert; unwritable `CHANT_REGISTRY` degrades gracefully with `registry_warning`, exit 0 | n/a | **PASS** | §2.4 |
| 11 | `chant doctor` clean; `bench` retrieval 8/8 + e2e 8/8; the three demos each `verify` → trusted | n/a | **PASS** | §3 |
| 12 | README + BitGN integration story is coherent enough to adopt | n/a | **PASS** | §4 |

---

## 1. Confirm-the-fix evidence (v0.1 findings)

### 1.1 Scenario A quickstart — FIXED (was FAIL)

The exact command sequence from `docs/testing/scenarios.md` Scenario A step 3, run in a fresh `git init`ed scratch:

```text
$ printf '#!/bin/sh\necho "hello $1"\n' > hello.sh
$ chant capture --id say-hello --task "say hello to a name" \
    --command "sh hello.sh {{name}}" \
    --entrypoint-src hello.sh --entrypoint hello.sh \
    --verifier 'sh -c "test \"$(sh hello.sh World)\" = \"hello World\""'
captured recipe "say-hello" (v1) at /tmp/chant-naive-v0.2/repoA.pYJ1/recipes/say-hello
→ run `chant verify say-hello` to confirm the verifier passes.

$ ls recipes/say-hello/
hello.sh  recipe.yaml

$ chant verify say-hello --input name=World
✓ say-hello verified — trusted (8ms)
exit: 0
```

The script is copied into the recipe dir at capture time. **Fixed: yes.**

The README's literal "Capture your own recipe" walkthrough (`greet` + `hello.sh`) also runs clean end-to-end:

```text
$ chant capture --id greet --task "greet a name" \
    --command 'sh greet.sh {{name}}' --entrypoint-src hello.sh --entrypoint greet.sh \
    --verifier 'sh -c "test \"$(sh greet.sh world)\" = \"hello world\""'
captured recipe "greet" (v1) at .../readme-walkthrough/recipes/greet
$ chant verify greet --input name=world
✓ greet verified — trusted (8ms)
```

### 1.2 Flag-order flexibility — FIXED (was FAIL)

All five commands accept flags either before or after the positional id, with identical output. Sampled:

```text
$ chant verify say-hello --json --input name=World
{
  "subcommand": "verify",
  ...
  "trusted": true,
  ...
}
$ chant verify --json say-hello --input name=World
{
  "subcommand": "verify",
  ...
  "trusted": true,
  ...
}
```

Same for `run`, `explain`, `search "hello" --json` vs `search --json "hello"`, and `invalidate throwaway --json` vs `invalidate --json throwaway2` (each emits a well-formed JSON `invalidate` payload with `stale:true`). **Fixed: yes.**

### 1.3 `explain --json` is snake_case — FIXED (docs + tool now agree)

```text
$ chant explain say-hello --json
{
  "description": "say hello to a name",
  "id": "say-hello",
  "kind": "executable_recipe",
  "scope": "project",
  "spell_hash": "77800962ddb70f5b",
  "what_to_do": { "command": "sh hello.sh {{name}}", "entrypoint": "hello.sh" },
  "when_to_use": { "task_patterns": ["say hello to a name"] },
  ...
}
```

Zero capitalized Go field names. **Fixed: yes.**

### 1.4 `--json` error contract — FIXED (was FAIL)

```text
$ chant verify no-such-recipe --json
{
  "blocking_error": true,
  "message": "recipe \"no-such-recipe\" not found",
  "subcommand": "verify"
}
exit: 1
# stderr was empty (verified with `2>/tmp/err.txt; wc -c </tmp/err.txt` → 0)
```

Same shape (`blocking_error:true` + `message` + `subcommand`) for `run no-such-recipe --json`, `explain no-such-recipe --json`, `invalidate no-such-recipe --json`, and `import does-not-exist --json`. Stdout JSON, empty stderr, exit 1. **Fixed: yes.**

### 1.5 `match_found` always present — FIXED (was PARTIAL)

- `suggest` no-match: `"match_found": false` present.
- `suggest` match: `"match_found": true` present.
- `capture --json`: `"match_found": false` present.
- `run --json`: `"match_found": false` present.
- `verify --json`: `"match_found": false` present.
- `invalidate --json`: `"match_found": false` present.

No `--json` payload I produced omitted the field. **Fixed: yes.**

Minor semantic note (not a regression): `search --json` emits hits with `confidence:0.7` yet `match_found:false`, because `match_found` is gated on the suggest threshold and `search` deliberately ranks everything. This matches the README's stated contract ("a candidate above the retrieval threshold exists") but is mildly surprising for `search` specifically. Flagging in NEW-3 below.

### 1.6 `capture --json` no-verifier warning — FIXED (was FAIL)

```text
$ chant capture --id noverify --task "test no verifier" --command "echo hi" --json
{
  "subcommand": "capture",
  "match_found": false,
  "recipe_id": "noverify",
  "version": 1,
  "exit_code": 0,
  "trusted": false,
  "captured": true,
  "message": "captured WITHOUT a verifier — reuse cannot be trusted until you add one",
  "suggested_commands": [
    "chant capture --id noverify --force --verifier \"<cmd>\" ..."
  ],
  "recommended_next_command": "chant verify noverify"
}
exit: 0
```

Warning is in `message`, the repair command is in `suggested_commands`. Exit 0 (a successful capture with a gap, not an error). **Fixed: yes.**

---

## 2. v0.2 feature evidence

### 2.1 Enchantment metadata via `capture` — PASS

```text
$ chant capture --id meta-test --task "compute revenue by channel" \
    --columns "channel,amount" --author "agent:me" \
    --command "python3 run.py {{input}}" --language python \
    --verifier "test -f revenue.json"
captured recipe "meta-test" (v1) at .../repoA.pYJ1/recipes/meta-test
```

`recipes/meta-test/recipe.yaml` (relevant tail):

```yaml
spell_hash: 380bf424e3f04aaf
provenance:
    origin: /tmp/chant-naive-v0.2/repoA.pYJ1
    captured_at: "2026-05-28T10:06:00Z"
    author: agent:me
scope: project
portability:
    determinism: deterministic
    input_contract:
        required_columns_any:
            - - channel
              - amount
    requires:
        runtime: python
```

Every field claimed in the README's "Available now" table is present: `spell_hash`, `provenance.origin`, `provenance.captured_at`, `provenance.author` (correctly carrying `--author agent:me`), `scope: project`, `portability.determinism`, `portability.input_contract.required_columns_any` (from `--columns`), `portability.requires.runtime` (from `--language`). **PASS, no caveats.**

### 2.2 Cross-package discovery — PASS (with NEW-1 wrinkle)

In repo A: capture a `greet-foreign` recipe, then `chant index`:

```text
$ chant index
indexed 7 recipe(s) → .../repoA.pYJ1/.chant/index.json
upserted 7 enchantment(s) into the registry → /tmp/chant-naive-v0.2/registry.json
```

Switch to repo B (empty, freshly `chant init`ed). `chant list` says `no recipes yet`. Then:

```text
$ chant suggest --task "greet a customer by name with a templated message" --global --json
{
  "subcommand": "suggest",
  "match_found": true,
  "hits": [
    {
      "id": "greet-foreign",
      "version": 1,
      "description": "greet a customer by name with a templated message",
      "confidence": 0.7,
      "verifier_exists": true,
      "reasons": [
        "foreign enchantment from registry — import then verify before trusting"
      ],
      "reuse_command": "chant import ff9a7d644ac15c3d   # copy locally, then `chant verify` before trusting",
      "global": true,
      "origin": "/tmp/chant-naive-v0.2/repoA.pYJ1",
      "scope": "project",
      "spell_hash": "ff9a7d644ac15c3d"
    },
    ...
  ],
  "exit_code": 0,
  "trusted": false,
  "recommended_next_command": "chant import ff9a7d644ac15c3d   # copy locally, then `chant verify` before trusting"
}
```

Foreign hits carry every documented field: `global:true`, `origin`, `scope`, `spell_hash`, plus a `reuse_command` that is `chant import …` (not `chant verify`). The top-level `recommended_next_command` mirrors it. This is the README §"Cross-package reuse" example with my hash substituted.

Without `--global`, the same query in repo B returns `match_found:false` — registry is opt-in per call. Good.

Now the trust-establishment gate:

```text
$ chant import ff9a7d644ac15c3d
imported "greet-foreign" from /tmp/chant-naive-v0.2/repoA.pYJ1 → .../repoB.LDOT/recipes/greet-foreign
⚠ foreign enchantment — NOT trusted. Run `chant verify greet-foreign` to re-run its verifier here.

$ chant verify greet-foreign --input name=world
✓ greet-foreign verified — trusted (7ms)
exit: 0
```

Verifier-first across packages works. The `import` warning explicitly says "NOT trusted" and points at `chant verify`. The recipe dir was copied with its `recipe.yaml` (preserving `spell_hash`, `provenance.origin`, etc.) and the entrypoint script. **PASS — the v0.2 showcase flow is real.**

`import` by **id** also works (`chant import greet-foreign` from a third fresh repo also resolved correctly). `import --json` on success emits a proper outcome (`subcommand:"import"`, `recipe_id`, `message: imported "..." from ... — NOT trusted yet; run its verifier in this repo`, `recommended_next_command: chant verify <id>`).

### 2.3 `import --as` and `--force` — PASS

```text
$ chant import ff9a7d644ac15c3d --as greet-renamed
imported "greet-renamed" from .../repoA.pYJ1 → .../repoB.LDOT/recipes/greet-renamed
$ grep '^id:' recipes/greet-renamed/recipe.yaml
id: greet-renamed

$ chant import ff9a7d644ac15c3d              # re-import w/o --force
chant: local recipe "greet-foreign" already exists (use --force to overwrite, or --as <newid> to import under a different id)
exit: 1

$ chant import ff9a7d644ac15c3d --json
{
  "blocking_error": true,
  "message": "local recipe \"greet-foreign\" already exists (use --force to overwrite, or --as <newid> to import under a different id)",
  "subcommand": "import"
}
exit: 1

$ chant import ff9a7d644ac15c3d --force
imported "greet-foreign" from .../repoA.pYJ1 → .../repoB.LDOT/recipes/greet-foreign
exit: 0
```

The collision message names *both* recovery flags. JSON error contract holds. `--force` overwrites cleanly. **PASS.**

### 2.4 `index --no-registry` and unwritable registry — PASS

```text
$ chant index --no-registry --json
{
  "count": 7,
  "index_path": ".../repoA.pYJ1/.chant/index.json",
  "registry_upserted": 0,
  "registry_warning": "",
  "subcommand": "index"
}
# verified: registry.json mtime unchanged across two --no-registry runs.

$ CHANT_REGISTRY=/dev/null/x.json chant index --json
{
  "count": 7,
  "index_path": ".../repoA.pYJ1/.chant/index.json",
  "registry_upserted": 0,
  "registry_warning": "open /dev/null/x.json: not a directory",
  "subcommand": "index"
}
exit: 0

$ CHANT_REGISTRY=/dev/null/x.json chant index
indexed 7 recipe(s) → .../repoA.pYJ1/.chant/index.json
chant: registry not updated — open /dev/null/x.json: not a directory
exit: 0
```

`--no-registry` short-circuits cleanly (count=0, no I/O). Unwritable registry path emits the warning in JSON and in the human stderr line and **still exits 0** — exactly the graceful degradation the README promises. **PASS.**

Nit: under `--no-registry`, `registry_warning: ""` is emitted as an empty string rather than omitted. Cosmetic. (NEW-2.)

---

## 3. Sanity checks

### 3.1 Doctor

In a populated scratch (8 recipes, 1 without a verifier):

```text
$ chant doctor
[ok] config: chant.yml present
[ok] recipes-dir: recipes/ present
[warn] verifiers: 1/7 recipe(s) have no verifier — reuse can't be trusted
[ok] gitignore: .chant/ is gitignored
doctor: no blocking issues.
exit: 0
```

In the source repo: all four checks `[ok]`, exit 0. `doctor --json` returns a structured `checks[]` with `name/status/detail` plus `ok:true`. Clean.

### 3.2 Bench

```text
$ chant bench
== suite: retrieval (8/8 passed) ==
  [PASS] RET-001 hit on similar revenue task — top=csv-revenue-by-channel @ 0.88
  [PASS] RET-002 no false hit on unrelated task — correctly returned no match
  [PASS] RET-003 refund task routes to refund recipe — top=refund-chargeback-threat @ 0.45
  [PASS] RET-004 column signals disambiguate revenue vs normalize — top=csv-revenue-by-channel @ 0.98
  [PASS] RET-005 column-adaptation routes header cleanup to normalize — top=normalize-orders-export @ 0.69
  [PASS] RET-006 ambiguous query breaks tie deterministically — top=csv-revenue-by-channel @ 0.83
  [PASS] RET-007 stale recipe retrievable but penalized ×0.5 — stale top=... @ 0.49 (active would be 0.98, ×0.5 penalty applied)
  [PASS] RET-008 column-signal precision: unsatisfied aliases add nothing — top=csv-revenue-by-channel @ 0.68

== suite: e2e (8/8 passed) ==
  [PASS] E2E-count-rule-policy run+verify count-rule-policy — verifier passed → trusted
  [PASS] E2E-csv-revenue-by-channel run+verify csv-revenue-by-channel — verifier passed → trusted
  [PASS] E2E-refund-approval run+verify refund-approval — verifier passed → trusted
  [PASS] E2E-001 failing verifier is not trusted
  [PASS] E2E-002 passing command but missing artifact is not trusted
  [PASS] E2E-003 missing {{input}} fails fast before running
  [PASS] E2E-004 capture → reuse → verify happy path is trusted
  [PASS] E2E-005 invalidate → re-verify re-blesses

bench: all scenarios passed.
exit: 0
```

8/8 retrieval + 8/8 e2e. Names are self-documenting. **PASS.**

### 3.3 Three shipped demos

Each verifies → trusted in the source repo:

```text
$ chant verify csv-revenue-by-channel        # → ✓ trusted (62ms)
$ chant verify refund-approval               # → ✓ trusted (109ms)
$ chant verify count-rule-policy             # → ✓ trusted (179ms)
```

All exit 0, all emit the expected verifier line ("OK: revenue-by-channel totals match…", "OK: all 3 refund-policy cases pass (incl. security no-leak)", "OK: all 3 count-rule cases pass (exact-format + ref invariants)"). **PASS.**

---

## 4. Adopt-from-zero impression (docs only)

### README

The v0.2 README is materially better than what I saw at v0.1:

- The flag-ordering "gotcha" is no longer plastered across the doc — there is exactly one parenthetical line ("flags work in any position"), where it belongs.
- The **`suggest --global` example block is verbatim what the binary emits** with my data substituted. That's the single highest-value docs change for a newcomer.
- The new **"Enchantment metadata & reuse"** section neatly separates "Available now" (with a field table) from "Still planned" (scope promotion + typed relations) — I knew exactly what to expect from `capture` without surprises.
- The cross-package §"Cross-package reuse" explicitly states the verifier-first invariant *across* repos: "Import stages; the verifier still blesses." That phrasing made the contract click for me in one read.

What I'd still tighten (not blockers):

- The 30-second demo opens with `go build`, then a separate `Install` section appears below. A first-time reader top-to-bottom infers "must build". Suggest reordering Install above the demo, or prefixing the demo with `# already-built binary in ./bin/chant — or install via curl above`.
- The metadata example shows `provenance.author: agent:claude` but the default (with no `--author`) is `agent:capture`. The table covers this, but the example block could note "(set with `--author`)" inline so readers don't think `agent:claude` is auto-detected.

### `docs/integration/bitgn.md`

This is the kind of integration doc I want for every tool I evaluate. It is concrete (named files, named overrides, real `chant capture` flags filled in), it has an honest table of what chant *adds* over BitGN's existing setup, it includes a rollback (week 2: gate behind `CHANT_REUSE=1`), and it answers the obvious "is this a replacement for our fixtures?" question with "no, the fixtures *become* the verifier." I could hand this to an engineer and they could land week-0 today.

Would a newcomer reading just README + `docs/integration/bitgn.md` adopt this for a real project? **Yes, with a small read of `docs/commands/capture.md` (for `--columns`/`--language`/`--author`) and `docs/commands/index.md` (for `CHANT_REGISTRY`).** Both exist.

### Cross-package flow clarity rating

**4 out of 5.** Loses one point only for **NEW-1** below — the imported-recipe metric inheritance.

---

## 5. NEW issues introduced in v0.2

### NEW-1 — Imported recipes inherit origin metrics, contradicting the "NOT trusted" warning

**Severity:** medium UX bug. Misleading but not a correctness violation.

**Reproduction:**

```text
$ chant import ff9a7d644ac15c3d           # into a brand-new repo, no prior verifies here
imported "greet-foreign" from .../repoA.pYJ1 → .../repoC/recipes/greet-foreign
⚠ foreign enchantment — NOT trusted. Run `chant verify greet-foreign` to re-run its verifier here.

$ chant list                              # BEFORE any local verify
1 recipe(s):
  greet-foreign                  v1  1 run(s) 100% ok
      greet a customer by name with a templated message
```

`recipe.yaml` post-import contains the upstream `metrics.runs: 1, last_success_at: ...` block. So a user who imports and then reads `chant list` sees `1 run(s) 100% ok` *before* any local verify — directly contradicting `chant import`'s own "NOT trusted" warning and the verifier-first thesis.

**Suggested fix (any one of):**
1. **Reset `metrics` on import** (`runs:0, failures:0, last_success_at: ""`). Cleanest. Origin metrics live in the registry entry / provenance, not the local card.
2. **Mark the local card as `status: candidate` or `pending` until first local verify** flips it to `active`. Surfaces in `chant list` like `(stale)` does.
3. At minimum, tag the list line: `greet-foreign v1 (imported, unverified here)` until the first local verify, then drop the tag.

Option 1 is my recommendation: it keeps `status: active` semantics intact (active means "not stale"), and the "is it trusted *here*?" question is what `verify` + the trust gate already answer separately.

### NEW-2 — `registry_warning: ""` emitted on `index --no-registry --json`

**Severity:** cosmetic.

```json
{
  "count": 7,
  "registry_upserted": 0,
  "registry_warning": "",
  "subcommand": "index"
}
```

Empty string rather than omitted (the rest of the outcome contract uses `omitempty`). Trivial — either omit it when empty, or document it as "always present, empty means clean."

### NEW-3 — `search --json` always returns `match_found:false` even with above-threshold hits

**Severity:** low; consistent with the contract as worded, but surprising.

```text
$ chant search "greet a name to say hello" --json | grep -E '(match_found|confidence)'
  "match_found": false,
      "confidence": 0.45,
```

The README defines `match_found` as "a candidate above the retrieval threshold exists" and intentionally distinguishes `search` (ranks everything) from `suggest` (gated). So this is by design. But for `search`, where the agent loop documentation says "read `match_found` → pick `hits[0]`," it's confusing: an agent that copies the agent-loop pattern verbatim onto `search` will incorrectly skip real hits.

**Suggested fix:** either (a) document explicitly in the `match_found` table row that "for `search`, `match_found` is always `false`; consult `hits[]` directly," or (b) reuse the suggest threshold inside `search` so the field is meaningful there too. (a) is the cheaper, contract-compatible option.

---

## 6. Bottom line

| Question | Answer |
|---|---|
| Are all 6 v0.1 findings fixed? | **Yes — all 6.** Evidence per finding in §1. |
| Does the v0.2 metadata-via-capture work as documented? | Yes. Every "Available now" README claim is populated. |
| Does the v0.2 cross-package flow work end-to-end? | Yes. `index` → `suggest --global` → `import` → local `verify` does what the README promises. Trust is established only by the local `verify`, as designed. |
| Cross-package flow clarity (1–5)? | **4/5.** Loses one point to NEW-1 (imported metric inheritance). |
| New issues introduced in v0.2? | **3.** NEW-1 (medium, list shows "100% ok" for unverified imports), NEW-2 (cosmetic empty-string field), NEW-3 (low, `search --json match_found` semantics). |
| **Would a newcomer reading the README + `docs/integration/bitgn.md` reach trusted cross-package reuse unaided?** | **Yes.** The README's `--global` JSON example matches the binary verbatim, the `import → verify` two-step is explicit, and the BitGN integration doc is concrete enough to act on without asking a human. |

Net: v0.2 lands the previous fixes and the promised new features cleanly. The one thing I'd want shipped before BitGN week-1 is **NEW-1** — without it, "is the imported recipe trusted yet?" is harder than it should be to read from `chant list` alone.
