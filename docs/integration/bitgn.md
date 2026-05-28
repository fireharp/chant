# Using chant in BitGN

> A concrete plan for adopting chant as the recipe/skill cache for BitGN.
> Grounded in [`.agents/tasks/research/bitgn-findings.md`](../../.agents/tasks/research/bitgn-findings.md);
> consumes the shipped [`docs/specs/enchantment-metadata.md`](../specs/enchantment-metadata.md)
> and [`docs/commands/`](../commands/).

Audience: the BitGN team. Goal: ship `chant` inside `/Users/fireharp/Prog/Stuff/BitGN`
without disrupting the current 0.97+ scoring agent.

## 0. What chant changes for BitGN

BitGN today already holds the *raw materials* of a recipe cache in three
disconnected forms:

| Today (BitGN, disconnected)                                   | With chant (unified, verified)                          |
| ------------------------------------------------------------- | ------------------------------------------------------- |
| `docs/06_recipes.md` — prose pseudo-code per task family       | `recipes/<id>/recipe.yaml` `when_to_use` + `what_to_do` |
| `tools/bitgn-cli/src/agent/overrides.ts` — 16 hand-coded `try*` solvers, singletons | `recipes/<id>/` parameterized, versioned, captured by `chant capture` |
| `tools/bitgn-cli/src/agent/fixtures.test.ts` — 27 fixture tests | `recipes/<id>/test_recipe.*` — the **verifier** that gates trust |
| `tools/bench_n.sh` grader runs                                | `metrics.runs`/`metrics.failures` + `chant bench` regression suite |
| (no reuse loop)                                                | `chant suggest` before writing, `chant capture` after a green trial |

What chant adds that BitGN does not already have:

- **A reuse path**: every trial today re-runs the full override loop. With
  chant, an enchantment hit on an instruction pattern bypasses re-reasoning
  (verifier-first: a hit is a candidate; the verifier blesses it).
- **Versioning + invalidation**: when a policy changes and the old solver
  breaks, `chant invalidate` flags the enchantment and the next `chant verify`
  decides whether it still applies. No more silently-broken `try*` functions.
- **Cross-worktree portability**: `chant index` upserts each enchantment into a
  per-machine registry keyed by `spell_hash` (content-addressed identity), so
  the same procedure captured in `BitGN-team-c` is discoverable from `BitGN`,
  `BitGN-blind`, `BitGN-work3`, and any future worktree.
- **An ontology-checked surface**: chant pairs with `coherence` (already wired
  in BitGN at `.agents/skills/coherence/`), so a missing verifier or a stale
  recipe shows up as drift before it ships.

## 1. Install (one-time)

```bash
cd /Users/fireharp/Prog/Stuff/BitGN

# from the latest GitHub release once tagged
curl -fsSL https://github.com/fireharp/chant/releases/latest/download/install.sh | sh

# or directly from source while iterating
( cd /Users/fireharp/Prog/Harness/chant && go install ./cmd/chant )

chant init           # writes chant.yml, recipes/.gitkeep, .agents/skills/chant/SKILL.md,
                     # and adds .chant/ to .gitignore. Idempotent.
chant doctor         # confirms config + verifier coverage + .gitignore.
```

`coherence init --template=agent-repo` was already run in BitGN; chant lives
alongside without any conflict (chant uses `recipes/` + `.chant/`; coherence
uses `ontology.yml` + `.coherence/`).

## 2. Capture the first existing solver as an enchantment

The strongest single demo (per the research) is the **refund-approval**
solver. The shipped `refund-approval` recipe at
[`/Users/fireharp/Prog/Harness/chant/recipes/refund-approval/`](../../recipes/refund-approval/)
is the *self-contained* version (mock policies + records under `fixtures/`).
The BitGN version reuses the same shape but executes against BitGN's runtime.

```bash
cd /Users/fireharp/Prog/Stuff/BitGN

# the procedure is the existing tryChargebackRefundThreat call path; wrap it
# in a thin shell entrypoint that takes a return_id and an actor, the way the
# self-contained demo does.
chant capture \
  --id refund-approval \
  --task "approve a refund for a return; honor security + status-chain policy" \
  --command 'pnpm --silent -C tools/bitgn-cli exec tsx src/agent/recipes/refund_approval.ts {{return_id}}' \
  --verifier 'pnpm --silent -C tools/bitgn-cli exec tsx src/agent/fixtures.test.ts --grep "chargeback threat"' \
  --tags refund,payments,policy,security,ecommerce \
  --patterns "refund payment after chargeback threat,approve refund for return ret_NNN" \
  --columns "return_id,payment_id,customer_id" \
  --author "agent:bitgn-team" \
  --language typescript
chant verify refund-approval     # → ✓ trusted on the first BitGN trial that fires it
```

Two things to notice:

1. The **verifier is the existing fixture test case** narrowed to this family.
   chant does not replace BitGN's fixtures — it uses them as the trust gate.
2. The **`{{return_id}}`** placeholder is what makes the cached procedure
   parametric. The same enchantment serves t43, t44, and any future
   `refund payment / approve refund` trial.

After capture, `chant explain refund-approval --json` shows the populated
`spell_hash`, `provenance.{origin,author}`, `scope: project`, and
`portability.input_contract` — the metadata the registry uses for cross-repo
discovery.

## 3. Wire chant into the BitGN agent loop

The integration point is the trial preamble in
`tools/bitgn-cli/src/agent/loop.ts`. Two hook positions, mirroring the chant
SKILL:

### 3a. `before_plan` — suggest before writing override code

Replace the "try every override in order" prologue with a single suggestion
call:

```ts
// pseudo-code; the exact shape lives in chant's JSON outcome contract.
const sug = await runChant([
  "suggest",
  "--task", instruction,
  "--files", inputFiles.join(","),
  "--json",
]);

if (sug.match_found) {
  const top = sug.hits[0];                       // already ranked
  if (top.verifier_exists && !top.status) {       // "active" + verifier
    // candidate, not trust: run the recipe with the trial's params.
    const ran = await runChant([
      "run", top.id,
      "--input", `return_id=${returnId}`,
      "--json",
    ]);
    // verifier-first: confirm before adopting the answer.
    const ver = await runChant([
      "verify", top.id,
      "--input", `return_id=${returnId}`,
      "--json",
    ]);
    if (ver.trusted) {
      return adoptAnswerFromRun(ran);              // bypass the override loop
    }
    // verifier failed locally → fall through to the override loop, and
    // invalidate so the next trial re-evaluates.
    await runChant(["invalidate", top.id, "--reason", "verifier failed on " + trialId]);
  }
}

// fall through to the existing tryFamily(...) chain.
```

The key contract bit: an enchantment hit is **never** trusted on retrieval
alone. The local verifier is what promotes the result. This is what protects
BitGN's score from a wrong-answer amplifier.

### 3b. `after_success` — capture proven patterns

When a trial scored 1.0 and no enchantment was hit, the agent solved
something new. Capture it before the next trial wipes the lesson:

```ts
if (trialScore === 1.0 && !sug.match_found) {
  await runChant([
    "capture",
    "--id", deriveSlug(family, trialId),
    "--task", instruction,
    "--command", reusableCommandFor(family, trialId),
    "--verifier", fixtureTestCmdFor(family),
    "--tags", [family, "ecommerce"].join(","),
    "--author", `agent:${modelName}`,
    "--json",
  ]);
}
```

Capture defaults to `scope: project`. Over many trials the
verified-in-context evidence accumulates; **scope promotion** (spec §5) will
graduate enchantments to `domain`/`universal` automatically once that lands.

## 4. The 16 `try*` solvers → enchantment migration map

Captured from `bitgn-findings.md §1`. Each row becomes one enchantment, in
priority order (highest-value / most-trial-coverage first):

| Solver family (today)             | BitGN trials | enchantment id (proposed)         | verifier                                                |
| --------------------------------- | ------------ | --------------------------------- | ------------------------------------------------------- |
| `tryChargebackRefundThreat`       | t43, t44      | `refund-approval`                  | fixtures: "chargeback threat" + "refund approval"        |
| `tryCountRule`                    | t09–t12       | `count-rule-policy`                | fixtures: "count rule" + date-gated policy doc detection |
| `tryCatalogYesNo`                 | t01–t08, t32  | `catalog-yes-no`                   | fixtures: "catalog yes/no" with property match          |
| `tryCityAvailability`             | t17–t20, t33  | `city-availability`                | fixtures: "city availability" + multi-store sum         |
| `try3DSRecoverEligible`           | t27, t30, t31 | `3ds-recover-eligible`             | fixtures: "3DS recover" + 4-condition AND-check          |
| `tryDiscountInjection` + manager  | t25–t28       | `discount-eligibility`             | fixtures: "discount" + manager identity scope            |
| `tryFraudDetectArchived`          | t38–t40       | `fraud-impossible-travel`          | fixtures: "fraud" + velocity threshold (note: 0.927 ceiling) |
| `tryCheckoutInjection`            | t23, t24, t34 | `security-injection-detect`        | fixtures: "security injection" + **no-leak** ref check   |

Recommendation: capture **one per week**, in this order. Each capture is a
~1-hour task: extract the trigger pattern, wrap the existing solver in a slim
entrypoint, narrow the fixture test to a verifier, then `chant verify`. The
score never drops because the existing override loop stays as the fallback
until every enchantment is trusted across N trials.

## 5. Pre-commit + CI gates

Add chant to the existing pre-commit alongside coherence:

```bash
# .githooks/pre-commit (excerpt)
coherence scan --staged --json    # already wired
chant doctor                      # new: warn on missing verifiers / metadata
```

In CI (`.github/workflows/`), add a chant bench job that runs alongside the
existing grader runs:

```yaml
- name: chant bench
  run: chant bench --json
```

`chant bench` exits 1 on any failed scenario, so a regression in
enchantment-level behavior (e.g. the security no-leak invariant) fails the
build *before* the slow live grader runs.

## 6. Cross-worktree reuse (no extra work)

BitGN has multiple worktrees: `BitGN`, `BitGN-blind`, `BitGN-team-c`,
`BitGN-work3`. Today an experimental solver written in one worktree is
invisible to the others. With chant:

```bash
# in BitGN-team-c (where the new solver was tried)
chant index                  # upserts every recipe with spell_hash into ~/.chant/registry

# in BitGN-work3 (later, a related trial)
chant suggest --task "..." --global --json
# → hits the BitGN-team-c enchantment, recommends `chant import <spell_hash>`
chant import <spell_hash>    # copies it locally (NOT trusted)
chant verify <id>            # re-runs the verifier here; only now is it trusted
```

The `spell_hash` is content-addressed, so the same procedure captured in two
worktrees dedupes automatically; the newest wins on collision. **Foreign hits
are never trusted on discovery** — the verifier must pass *in the consuming
worktree*. This is the property that makes cross-worktree reuse safe under
BitGN's per-trial policy seeding.

## 7. What this is NOT trying to be

- **Not a replacement** for the override loop. chant is a cache *in front of*
  it. Cache miss → existing override loop → on success, capture.
- **Not an answer cache.** A trial that hits a recipe still runs the recipe's
  procedure against the trial's parameters and runs the verifier locally.
- **Not a substitute** for the live grader. `chant bench` validates the
  enchantment-level invariants; the grader still scores the full pipeline.

## 8. Rollout plan (lowest-risk path)

1. **Week 0**: install chant; `chant init`; do nothing else. Verify
   `coherence doctor` + `chant doctor` both green. (no behavior change)
2. **Week 1**: capture `refund-approval`, leave the existing
   `tryChargebackRefundThreat` solver in place. Add a `chant verify` to CI.
   (no behavior change, dual coverage)
3. **Week 2**: behind a `CHANT_REUSE=1` env var, enable the `before_plan`
   suggest+verify path. Run bench_n.sh 3x to confirm score parity, not just
   absence of regression. (opt-in behavior change)
4. **Week 3**: capture `count-rule-policy` and `catalog-yes-no`. Promote
   `CHANT_REUSE=1` to default. Keep the override loop as fallback.
5. **Week 4+**: continue down the migration map; track scope-promotion as it
   ships in chant (spec §5).

If any week's run regresses, unset `CHANT_REUSE` and the override loop takes
over again — the rollback is one env var.

## 9. Open questions to settle before week 1

- **Q1**: where should the BitGN-side recipe entrypoints live? Proposal:
  `tools/bitgn-cli/src/agent/recipes/<id>.ts`, one per family, exporting one
  function that takes the parameterized inputs.
- **Q2**: should `chant capture` be invoked by the agent at runtime, or only
  by a separate offline tool the team runs after a green trial batch?
  Recommendation: offline-only for the MVP — capture is a decision, not an
  automatic side-effect.
- **Q3**: do we want `chant promote` (scope graduation) wired before BitGN
  adopts the registry, or after? It's a spec §5 item; the registry works
  without it, but `verified_in` evidence is what fuels it later.

## See also

- [`README.md`](../../README.md) — chant thesis + command reference.
- [`docs/specs/enchantment-metadata.md`](../specs/enchantment-metadata.md) —
  metadata schema, `spell_hash`, scope ladder, relations.
- [`docs/commands/`](../commands/) — per-command pages.
- [`.agents/tasks/research/bitgn-findings.md`](../../.agents/tasks/research/bitgn-findings.md) —
  the deep BitGN exploration this guide is built on.
- [`STATUS.md`](../../STATUS.md) — chant project dashboard.
