# Iteration 3 — v0.2 ship: cross-package registry + BitGN integration guide

Date: 2026-05-28 · Lead coordinating an agent team.

## Arc

Closed out the "cross-package enchantment reuse" arc and shipped the artifacts
BitGN needs to actually adopt chant. Four pushed increments this iteration:

1. `a67eec7` **registry** (registry-dev): per-machine `~/.chant/registry/index.json`,
   `chant index` upserts, `chant suggest --global` returns foreign hits annotated
   with `global`/`origin`/`scope`/`spell_hash`, `chant import <id|spell_hash>`
   stages a foreign enchantment locally. Verifier-first preserved across
   packages: `import` sets `trusted: false` and points at `chant verify`.
   New `internal/registry` package with its own tests + a CLI-level test that
   exercises index → suggest --global → import across two temp repos.
2. `2693304` **registry docs** (docs-writer, resumed): registry / `--global` /
   `import` move from planned → available-now in README/AGENTS/docs/README;
   new `docs/commands/import.md`; `suggest.md` gains `--global`; `index.md`
   updated for the new outcome-style `--json` shape
   (`{subcommand,count,index_path,registry_upserted,registry_warning}`).
3. `1efb8a2` **BitGN integration guide**: a concrete 9-section adoption plan at
   `docs/integration/bitgn.md`. Install → capture `refund-approval` reusing the
   existing fixture as verifier → wire `before_plan` suggest/verify and
   `after_success` capture into the trial loop → an 8-row migration map of the
   `try*` solvers → enchantment ids → existing fixtures as verifiers → 5-week
   rollout with a `CHANT_REUSE=1` env-var rollback.
4. (in flight: count-rule demo) — third self-contained demo grounded in BitGN
   t09–t12, adds the date-gated-policy-reading capability neither existing
   demo has.

## Validation

- `go test ./...` green across all 9 packages.
- `chant bench` → retrieval 8/8, e2e 7/7 (will become 8/8 once count-rule lands).
- Full 15-command capstone smoke test green in a temp repo: init →
  capture(+metadata) → list → search → explain(snake_case) → suggest → run
  (adapt) → verify(trusted) → index(registry upsert) → invalidate(stale) →
  re-verify(re-bless) → status → doctor → bench.
- Cross-repo loop validated end-to-end: capture in repo A → `chant index` →
  `chant suggest --global` from repo B finds the foreign enchantment with the
  correct `import` reuse_command → `chant import` stages locally with
  `trusted: false` → local `chant verify` re-runs the verifier and only THEN
  trusts. Verifier-first held across the package boundary.

## Coordination

- Five subagents touched the tree this iteration with strict file-ownership
  partitioning: registry-dev (internal/registry + cmd wiring), docs-writer
  twice (markdown), naive-user (read-only + reports/), bench-dev
  (internal/bench), demo-dev (recipes/count-rule-policy/ in flight).
- Continuous commit-and-push throughout (user preference): every increment was
  committed by explicit paths to avoid sweeping up another agent's in-progress
  work.

## Open blocker (one user action)

`release-please` workflow is failing on master: "GitHub Actions is not
permitted to create or approve pull requests" (repo setting). Fix:
Settings → Actions → General → Workflow permissions →
"Allow GitHub Actions to create and approve pull requests" → Save.
After the toggle, re-running the failed job opens the release PR for v0.1.0
(or v0.2.0 — release-please decides from conventional-commit accumulation),
and merging it triggers the cross-built `chant_{os}_{arch}` tarballs the
README's `install.sh` resolves. Captured as task #16.

## Next

1. Integrate the count-rule demo (#17) → bench grows to e2e 8/8.
2. Pick one of the post-MVP items: scope promotion (spec §5),
   typed-relations surfacing, or wiring chant into BitGN as a real adoption
   PR (cross-repo, requires explicit go-ahead per the working-style memory).
3. Once release-please is unblocked: tag v0.2.0 and validate `install.sh`
   end-to-end.
