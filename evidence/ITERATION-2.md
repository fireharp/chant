# Iteration 2 — team integration, metadata, hardening, release pipeline

Date: 2026-05-27 · Lead coordinating a background team.

## Arc

Built a working v0.1 baseline first, then ran a team against it and integrated.
Three increments shipped to `origin/master`:

1. `f75c18c` **enchantment metadata** (dev-metadata, worktree→reviewed): optional,
   backward-compatible fields on Recipe — `spell_hash` (content-addressed
   portable identity), `provenance`, `scope`, `portability`, `relations`;
   `chant capture` populates them; `ComputeSpellHash()` per spec §4; 8 tests.
2. `c8d93e3` **contract fixes** (from the naive-user findings): `--json` error
   paths emit `{blocking_error, message}` + exit 1; `match_found` always present;
   `capture --json` surfaces the no-verifier warning. +2 regression tests.
3. `96aab25` **CI + release-please + Makefile**: gofmt/vet/test/`chant bench`
   gate; cross-built `chant_{os}_{arch}` release tarballs so the documented
   `install.sh` resolves; coherence drift dogfood job on PRs.

## Bugs found & fixed (team review loop working as intended)

dev-tester and the naive-user independently surfaced the same three CLI bugs;
all fixed in v0.1.x before they reached docs/users:
- `verify/run --json` exited 0 on failure → now exit 1 in every mode.
- flags after a positional id were dropped (`verify <id> --json`) → `parseFlags`
  allows interspersed flags.
- `explain --json` emitted Go-cased keys → now snake_case via a yaml round-trip.

The naive-user (first-time-user) verdict was **Partially**: engine strong
(verifier-first gate, CSV schema-drift handling), but a quickstart that failed
because a referenced script wasn't copied into the recipe dir. Verified the
correct incantation uses `--entrypoint-src` to copy the script in; handed the
fix to docs reconciliation.

## In flight at time of writing

- docs-writer (resumed): reconcile README/AGENTS/docs/commands + scenarios.md to
  current behavior — remove stale flag-order warnings, fix the quickstart,
  document the now-real metadata + JSON-error contract (#12).
- bench-dev: isolated negative-gate e2e scenarios (verify-fail, artifact-gate,
  missing-input) + score/stale-penalty retrieval assertions, wiring qa-suite's
  RET-005..008 / E2E-001..005 (#13).

Both leave changes uncommitted for lead review; integrate → commit v0.2.

## Coordination notes

- File-ownership partitioning kept parallel agents conflict-free: docs=markdown,
  bench-dev=internal/bench, dev-metadata=recipe/commands. Worktree isolation
  leaked to the main tree once (dev-metadata) but was benign — reviewed +
  committed from the main tree.
- Devops files were committed by explicit path (never `git add -A`) to avoid
  sweeping up agents' in-progress work.

## Next

1. Integrate docs (#12) + bench (#13); reset demo metrics; commit v0.2.
2. Cross-package registry + `suggest --global` + `import` (#9) — the payoff of
   the spell_hash identity foundation (spec §6).
3. GitHub Pages docs site (docs/index.html) for parity with coherence.
