# BitGN findings — chant design input

Research date: 2026-05-27. Source: deep read of `/Users/fireharp/Prog/Stuff/BitGN`
and its worktrees. This file seeds chant's demo, benchmark, and schema design.

## Why BitGN matters for chant

BitGN resolves e-commerce customer-service trials by **writing adapted code
against current-state policies** — read the request, read today's policy docs,
write deterministic code that resolves the case, verify, and answer. It scores
on a 44-task grader (0.43 → 0.9756 over ~77 iterations).

BitGN already contains, in three separate and unconnected forms, exactly what
chant unifies into one verified object:

| BitGN artifact | What it is | chant unifies it as |
| --- | --- | --- |
| `docs/06_recipes.md` | **prose** pseudocode per task family — documents intent, no regression protection | recipe `when_to_use` + `what_to_do` |
| `tools/bitgn-cli/src/agent/overrides.ts` | 16 hand-coded `try*` deterministic solvers (~1720 lines), singletons, not parameterized or versioned | recipe `command` / executable procedure |
| `tools/bitgn-cli/src/agent/fixtures.test.ts` | 27 fixture tests (offline trigger + parse) | recipe `verification.command` |
| live grader + `tools/check_overfit.sh` | per-trial 0–1 score + anti-overfit gate | recipe `metrics` + evidence |

The gap: **no reuse path**. Every trial re-runs the full override loop; recipes
are prose that drifts from the code; fixtures are separate from the prose. chant
closes that: cache hit on an instruction pattern → adapt params → run → verify →
trust, with versioning and invalidation.

## Recurring procedures (recipe candidates)

1. **Catalog lookup with property matching** (t01–t08) — "Do you have <Brand>
   <Series> <Model> with <prop>=<val>?" → SQL lookup + property filter + cite
   exact SKU path. Verifier: answer contains `<YES>`/`<NO>`, refs stat-resolve.
2. **Count-rule policy application** (t09–t12) — "How many <kind> products …?"
   → find today's date-gated policy file, apply filter, format token
   (`<COUNT:%d>` vs bare `%d`). Verifier: exact format + matching rule file ref.
3. **Inventory availability by store** (t17–t20) — multi-table SQL join + city
   filter + sum. Verifier: single integer, refs include every store path.
4. **3DS payment recovery** (t27, t30–t31) — security gate (`/bin/id` ==
   basket owner) + 4-condition policy AND-check + conditional side-effect
   (`/bin/payments recover-3ds`). Verifier: cross-customer → `DENIED_SECURITY`.
5. **Discount eligibility + application** (t25–t28) — manager identity scope +
   subtotal-based max-% rule + `/bin/discount` side-effect.
6. **Refund approval / chargeback threat** (t43–t44) — status state-machine +
   ownership check + `/bin/payments refund`. **Strongest demo candidate.**
7. **Fraud impossible-travel detection** (t38–t40) — TSV parse + geo distance +
   velocity threshold. Known ceiling: 3 persistent false positives (0.927).
8. **Security injection detection** (t23–t24) — override-language regex →
   `DENIED_SECURITY`, must NOT cite foreign baskets even if named.

## "Adapted code against current-state policies", concretely

- Policies: `/docs/{security,checkout,discounts,returns,payments/3ds}.md` plus
  date-gated `/docs/{current-updates,policy-updates,ops-policy-notes,catalogue-addenda}/YYYY-MM-DD.md`.
- State records: `/proc/{baskets,payments,returns,customers,stores}/*.json`,
  catalog under `/proc/catalog/...`.
- Pipeline: read policy (`/bin/date`, `/bin/id`, tree `/docs`) → query state
  (SQL + `/proc`) → apply policy logic deterministically → emit answer (exact
  message format + refs) → verify (refs stat-resolve, grader tokens present,
  side-effects only when safe). Same task + different policy file = different
  code path. chant caches the *template* + the policy-read→code-structure map.

## Verifiers in BitGN (success signals chant can reuse)

- **Fixture suite** (offline): `cd tools/bitgn-cli && pnpm exec tsx src/agent/fixtures.test.ts` → "27 pass, 0 fail".
- **Augment unit tests**: `pnpm exec tsx src/agent/augment.test.ts` → 12 cases.
- **Anti-overfit**: `bash tools/check_overfit.sh` → "✅ CLEAN" (forbids hardcoded `basket_NNN`, `pay_NNN`, trial ids, token-spray).
- **Pre-flight**: `bash tools/ready_check.sh` → "✅ READY".
- **Grader bench**: `bash tools/bench_n.sh 3` (mean+stddev), `bash tools/compete.sh`, `bash tools/compare_runs.sh A B`.
- Known failure modes: fraud false positives (inherent), LLM JSON truncation,
  provider quota (429), world-state variance (take best-of-N).

## Strongest chant demo grounded in BitGN

**Refund approval / chargeback threat (t43–t44).** Agent solves once →
`tryChargebackRefundThreat` (regex + SQL + status check + side-effect). chant
captures recipe `refund-chargeback-threat-v1`:
- applicability: instruction regex `refund.*pay_\d{3}.*(chargeback|dispute|bank)`,
  required records `[payment, return]`, required policies `[returns.md, security.md]`.
- procedure: read records → security gate → status-chain AND-check →
  `/bin/payments refund <return_id>` → answer with refs.
- verifier: `fixtures.test.ts` "chargeback threat" case + grader t43/t44 == 1.0.

New trial "finalize refund for return ret_042" → chant matches `refund-approval`
family → adapts `ret_042` → executes deterministically → verifies → trusted.
Win: ~1.5s vs ~45s, $0 LLM, 100% vs ~95% reliability, and the 50-line
hand-coded `try*` becomes one parameterized recipe.

## Conventions chant should stay coherent with

- Layout: `tools/<name>/`, `docs/NN_*.md` numbered guides, `tools/*.sh` wrappers,
  `.agents/tasks/*.md`, `.agents/skills/<skill>/SKILL.md`, `ontology.yml`,
  `STATUS.md`, `.run-logs/` + `.run-stats/` (gitignored), `CHANGELOG.md`.
- coherence is already wired in BitGN (`.agents/skills/coherence/SKILL.md`,
  `init --template=agent-repo`, planned pre-commit `coherence review`).
- chant should ship a `.agents/skills/chant/SKILL.md` peer and a JSON outcome
  contract agents/hooks can consume, mirroring coherence.

## Takeaway

chant's MVP = recipe object (applicability + procedure + verifier + metrics +
invalidation) + a CLI with suggest/capture/run/verify/list/search and a
verifier-first trust gate. The BitGN refund recipe is the canonical "real"
demo; a self-contained CSV-revenue recipe is the zero-dependency demo.
