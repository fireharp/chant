# Iteration 1 — chant MVP vertical slice

Date: 2026-05-27 · Lead + team (background agents).

## Goal

Stand up `chant` as a real Go CLI: the recipe-cache complement to `coherence`.
chant turns successful agent work into reusable, **versioned, verified**
procedures ("enchantments"). Cache the tested *method*, not the answer.

## What shipped this iteration

- **Bootstrap (coherence-managed):** ran `coherence init --template=agent-repo`
  so chant is itself drift-tracked. `coherence doctor` is green.
- **Go module + CLI** (`cmd/chant`): `init, suggest, capture, run, verify,
  list, search, explain, invalidate, index, status, doctor, bench, version`.
- **Core packages** (`internal/`): `recipe` (card model, load/save,
  fingerprints, metrics), `config` (chant.yml), `store` (recipes/ committed +
  .chant/ runtime + index + run logs), `retrieve` (deterministic hybrid scorer
  = lexical + structural signals + verifier success rate), `glob`, `runner`
  (the **verifier-first trust gate**), `outcome` (JSON contract), `status`,
  `bench`.
- **Demo enchantment** `csv-revenue-by-channel`: zero-dependency Python-stdlib
  procedure robust to column drift (Shopify/Stripe/custom), with a
  self-contained verifier and 3 example CSVs.
- **Validation:** `chant bench` → retrieval 4/4, e2e 1/1. End-to-end proven:
  `suggest` hits at 0.88 confidence with reasons; `verify` runs procedure +
  verifier → **trusted**, records metrics.

## Key design decisions

- **Verifier-first is the spine.** A retrieved enchantment is a *candidate*;
  `trusted` in the JSON contract is true only after a passing verifier. `run`
  alone never sets trust. This is what separates a useful cache from a
  wrong-answer amplifier.
- **Committed vs runtime split** mirrors coherence: `recipes/` is the shared,
  versioned library; `.chant/` is gitignored runtime (index + run logs).
- **Deterministic retrieval** (no embeddings/LLM) so `suggest` is reproducible
  and testable; an optional semantic pass can layer later, gated like
  coherence's LLM pass.

## Research grounding

- `.agents/tasks/research/bitgn-findings.md` — BitGN already has prose recipes
  (`docs/06_recipes.md`), 16 hand-coded `try*` solvers, and 27 fixture tests,
  but **no reuse path**. chant unifies these into one verified, versioned,
  parameterized object. Canonical real demo: the refund/chargeback recipe.

## In flight (team, background)

- **dev-tester** — Go unit + integration tests → `go test ./...` green.
- **docs-writer** — README/AGENTS/docs site coherent with coherence.
- **qa-suite** — user stories (docs/user-stories/US-###) + scenario catalog.
- After docs: **naive-user** usability pass on CLI + docs clarity.

## New direction (this iteration, spec'd not yet coded)

`docs/specs/enchantment-metadata.md` — metadata for reuse: portable identity
(`spell_hash`), provenance, a **scope/universality ladder**
(project→domain→universal, *earned* via verified-in-N-contexts), typed
relations (supersedes/mirrors/depends_on/implements, reusing coherence's edge
vocabulary), and cross-package discovery (`suggest --global`, `import`). All
additive/backward-compatible. Tasks #8–#9 track implementation.

## Next iteration

1. Integrate team output; `go test ./...` + `chant bench` + `coherence review`
   green; reset demo metrics; commit v0.1 baseline.
2. Implement additive enchantment metadata (#8).
3. Add the BitGN refund enchantment as a second demo.
4. Cross-package registry (#9).
