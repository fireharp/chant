# chant — status

A recipe cache for coding agents: **cache the tested method, not the answer.**
chant turns successful agent work into reusable, versioned, **verified**
procedures ("enchantments"). Sibling to [coherence](https://github.com/fireharp/coherence)
(a drift detector): coherence catches *what broke*; chant captures *what worked
and is reusable*.

Repo: `github.com/fireharp/chant` · branch `master` · built via a coordinated
agent team. This file is the single-page current-state dashboard; per-iteration
detail lives in [`evidence/`](evidence/), the how-to in [`README.md`](README.md),
the metadata design in [`docs/specs/enchantment-metadata.md`](docs/specs/enchantment-metadata.md).

## The spine: verifier-first

A retrieved enchantment is a **candidate**, never trusted until its verifier
passes: `retrieve → adapt → execute → verify → accept`. `trusted` in the JSON
contract is true only after a passing verifier. This is the difference between a
useful cache and a wrong-answer amplifier.

## Architecture

```
cmd/chant/            CLI: init suggest capture run verify list search explain
                      invalidate index status doctor bench version (+ import, --global wip)
internal/
  recipe/             Recipe card model + enchantment metadata (spell_hash, provenance,
                      scope, portability, relations); load/save/fingerprints/metrics
  config/             chant.yml (recipes_dir + retrieval weights)
  store/              recipes/ (committed) vs .chant/ (gitignored runtime: index, runs)
  retrieve/           deterministic hybrid scorer = lexical + signals + verifier success
  runner/             {{var}} adapt + execute + the verifier-first trust gate
  glob/ outcome/ status/ bench/   matcher · JSON contract · STATUS · validation suite
  registry/           per-machine cross-package enchantment registry (wip, #9)
recipes/              shipped demo enchantments (committed library)
docs/ evidence/       command reference + specs + user stories · iteration ledger
```

## Shipped

| Area | State |
| --- | --- |
| Core CLI (14 commands) | ✅ |
| Verifier-first trust gate | ✅ (locked by tests) |
| Deterministic retrieval (lexical + signals + success-rate) | ✅ |
| Enchantment metadata (spell_hash, provenance, scope, portability) | ✅ capture populates them |
| Demos | ✅ `csv-revenue-by-channel` (schema-drift-robust), `refund-approval` (BitGN policy + security no-leak verifier) |
| Tests | ✅ `go test ./...` green across all packages |
| Bench | ✅ 15 scenarios (8 retrieval incl. score/stale assertions, 7 e2e incl. isolated negative gates) |
| Docs | ✅ README, AGENTS, 13 command pages, spec, 23 user stories, scenario catalog — reconciled to the binary |
| CI + release | ✅ gofmt/vet/test/bench gate; release-please cross-builds installable tarballs |
| JSON outcome contract | ✅ incl. blocking_error on error paths; always-present match_found |
| Cross-package registry | ✅ `index` upserts to a per-machine registry; `suggest --global` surfaces foreign enchantments by `spell_hash`; `import` copies one locally (verifier-first — import ≠ trust) |

## In progress / roadmap

- **Scope promotion** (spec §5): earn project → domain → universal from
  verified-in-N-contexts evidence; `chant promote`.
- **Typed relations** surfaced/queried (supersedes/mirrors/depends_on/implements).
- **Optional semantic retrieval** pass (gated like coherence's LLM pass).
- **BitGN integration**: capture the refund/count-rule/catalog solvers as
  reusable enchantments (see `.agents/tasks/research/bitgn-findings.md`).
- Make `enchantment` a coherence graph node kind once the registry lands.

## Validation snapshot

`go test ./...` → all green · `chant bench` → retrieval 8/8, e2e 7/7 ·
`chant doctor` → no blocking issues · `coherence review` → safe_to_commit,
drift telemetry-only. chant is itself a coherence-managed repo.
