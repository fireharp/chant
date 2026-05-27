# chant scenario catalog

This catalog drives two kinds of testing:

- **Automated bench scenarios** — expressed in the shape of
  `internal/bench/bench.go` (`RetrievalScenario` for retrieval, the `RunE2E`
  run+verify gate for end-to-end) so a developer can wire each new one in.
- **Naive-user scenarios** — plain-language runbooks for a first-time human or
  agent using only `chant --help` + docs, handed to a guinea-pig / cli-user
  tester.

Terminology: a chant **recipe** is also called an **enchantment** — synonyms.
The on-disk form is `recipes/<id>/recipe.yaml`.

Verifier-first invariant under test everywhere: a retrieved enchantment is a
*candidate* (`trusted: false`); only a passing `chant verify` sets
`trusted: true`. `retrieve → adapt → execute → verify → accept`.

---

## 1. Automated bench scenarios

### 1a. Retrieval suite (`bench.RetrievalScenario` → `RunRetrieval`)

Each scenario builds a synthetic recipe set, runs `retrieve.Suggest`, and
asserts the top id and whether any match clears `retrieval.threshold` (default
`0.25`). `ExpectMatch=false` is a true-negative (must return zero matches).
Scores below use the default weights (lexical `0.5`, tags/signal `0.3`,
success `0.2`).

The three synthetic recipes referenced are the ones already in
`RetrievalSuite()`: `csv-revenue-by-channel` (csv file + column signals),
`refund-chargeback-threat` (no signals, lexical only), `normalize-orders-export`
(csv file signal, no column signals).

| id | suite | setup (synthetic recipes / query) | expected top | expect_match | what it proves |
| --- | --- | --- | --- | --- | --- |
| RET-001 | retrieval | full set; query "calculate revenue by channel from this CSV export", files `[orders_shopify.csv]`, columns `[utm_source, amount]` | `csv-revenue-by-channel` | true (≈0.88) | a similar task with matching file + column signals retrieves the right enchantment as a candidate |
| RET-002 | retrieval | full set; query "rotate the kubernetes TLS certificates in the staging cluster" | — | false | **true-negative**: an unrelated task returns no candidate (no wrong-answer amplification) |
| RET-003 | retrieval | full set; query "the customer wants a refund and is threatening a chargeback dispute" | `refund-chargeback-threat` | true (≈0.45) | a refund task routes to the refund enchantment on lexical signal alone (no file/column signals) |
| RET-004 | retrieval | full set; query "compute revenue by channel", files `[orders.csv]`, columns `[channel, price]` | `csv-revenue-by-channel` | true (≈0.98) | column + file signals disambiguate revenue from the look-alike normalize enchantment |
| **RET-005** (new) | retrieval | full set; query "normalize the messy column headers in this orders export", files `[orders.csv]` | `normalize-orders-export` | true | **column-adaptation across schemas**: a header-cleanup task routes to normalize, not revenue, even though both carry the `*.csv` file signal — lexical overlap on "normalize/headers" breaks the tie |
| **RET-006** (new) | retrieval | full set; query "analyze the ecommerce orders export", files `[orders.csv]` (no `--columns`) | `csv-revenue-by-channel` | true | **ambiguous-query tie-break**: when two csv enchantments both match the file glob and the query is generic, deterministic ranking (score, then ascending id) yields a stable, reproducible top hit; asserts the order does not flap |
| **RET-007** (new) | retrieval | **single stale recipe**: `csv-revenue-by-channel` with `Status: "stale"`; query "compute revenue by channel from csv", files `[orders.csv]`, columns `[channel, amount]` | `csv-revenue-by-channel` | true, but score halved | **verifier-first-stale (retrieval half)**: a stale enchantment is still retrievable but penalized (score × 0.5) and its `reasons` carry the stale warning — it is a candidate, never trusted, until re-verified (e2e half: E2E-005) |
| **RET-008** (new) | retrieval | full set; query "compute revenue by channel", columns `[foo, bar]` (none satisfy any alias group) | `csv-revenue-by-channel` | true (lexical only) | **column-signal precision**: columns that satisfy no alias group earn no column contribution; the hit still comes from lexical + file signals, proving the column gate is all-groups-must-be-covered, not any-overlap |

Notes for the developer wiring these in:

- RET-005/006/008 reuse the existing `RetrievalSuite()` recipe set; only the
  `Query` and `ExpectTop`/`ExpectMatch` change.
- RET-007 needs a one-recipe set with `Status: "stale"`. The current
  `RetrievalScenario` struct does not assert score magnitude — to assert the
  ×0.5 penalty you either (a) add an `ExpectScoreBelow`/`ExpectStalePenalty`
  field, or (b) include a fresh duplicate and assert the active one ranks above
  the stale one. **GAP for the lead: `RetrievalScenario` has no field to assert
  a score/penalty or to assert hit `reasons`; only top-id + match-or-not.**

### 1b. End-to-end suite (`RunE2E` run+verify gate)

`RunE2E` loads every recipe in the store, and for each that has a verifier AND
an example, runs `runner.Run` then `runner.Verify` and asserts the trust gate.
Recipes without a verifier or example are reported as skipped passes.

| id | suite | setup | expected | what it proves |
| --- | --- | --- | --- | --- |
| E2E-csv-revenue-by-channel | e2e | the shipped `recipes/csv-revenue-by-channel/` with example `examples/orders_shopify.csv` | run `run.py` → verify `test_recipe.py` → `verifier passed → trusted`, artifact `revenue_by_channel.json` exists | the happy-path trust gate: a real procedure + verifier promotes a reuse result to trusted |
| **E2E-001** (new) | e2e | a fixture enchantment whose verifier command exits non-zero (e.g. `command: "false"` or a deliberately failing test) | `runner.Verify` returns `trusted=false`; scenario asserts NOT trusted | **verify-fails-not-trusted**: a failing verifier must keep the result untrusted (the wrong-answer-amplifier guard) |
| **E2E-002** (new) | e2e | a fixture enchantment that runs fine but declares an `expected_artifacts: [missing.json]` it never produces | `runner.Verify` returns `trusted=false` (artifact-missing branch) | **artifact-gate**: trust requires every declared artifact to exist, not just exit 0 |
| **E2E-003** (new) | e2e | a fixture enchantment with `command: "python3 run.py {{input}}"` invoked with NO `input` and no `examples` | `runner.Run` returns an error `missing inputs: input`; scenario asserts fail-fast (no execution, no trust) | **missing-input {{var}} fail-fast**: an unresolved placeholder must abort before running a half-formed command |
| **E2E-004** (new) | e2e | **capture-then-reuse**: programmatically `capture` a tiny enchantment (`--command "echo hi > out.txt"`, `--verifier "test -f out.txt"`), then `suggest` for its task, then `run` + `verify` | suggest returns it as top hit (`trusted:false`); verify yields `trusted:true`; metrics.runs increments | **capture → suggest → verify end-to-end**: the full lifecycle round-trips through the store and the trust gate |
| **E2E-005** (new) | e2e | **verifier-first-stale (e2e half)**: take a verified enchantment, `invalidate` it (`status:stale`), then `verify` it again | after invalidate: `status:stale`, suggest hit penalized (RET-007); after a passing re-verify: `trusted:true` AND `status` flips back to `active` | **stale must not be trusted until re-verified, and a pass re-blesses it** (the invalidate → re-verify → re-bless loop) |

Notes for the developer wiring these in:

- `RunE2E` currently iterates the *live store* and only distinguishes
  pass/skip/fail by presence of verifier+example. E2E-001/002/003 need fixture
  enchantments that are intentionally broken; running them against the live
  store would make `chant bench` red. **Recommended: add a dedicated
  `E2EScenario` struct (fixture recipe + expected trusted bool + expected run
  error) and a `RunE2EScenarios()` over an isolated temp store**, mirroring how
  `RetrievalSuite()` builds synthetic recipes in-memory. This keeps the live
  `chant bench` green while exercising the negative gates.
- E2E-004 and E2E-005 need a temp store (capture/invalidate mutate disk); run
  them against a scratch dir, not the repo's `recipes/`.

---

## 2. Naive-user scenarios

Audience: a first-time human or agent who has only `chant --help` and the docs.
Each step lists the **action**, **what the user should see**, and
**PASS / FAIL / CONFUSED** criteria. CONFUSED = the tool ran but the next step
was not obvious from the output.

Setup for all scenarios: a scratch directory the tester can write to, with the
`chant` binary on PATH (built via `go build -o bin/chant ./cmd/chant`).

> Cross-cutting gotcha for the tester (real CLI behavior): for `run`, `verify`,
> `explain`, `invalidate`, and `search`, **flags such as `--json` / `--input`
> must come BEFORE the recipe id** (Go's flag parser stops at the first
> positional argument). `chant verify --json my-recipe` works; `chant verify
> my-recipe --json` silently ignores `--json`. Note any place this trips you up.

### Scenario A — From zero to a reused enchantment

**Goal:** init a repo, capture a tiny enchantment, find it via suggest, and
verify it to establish trust.

1. **Action:** run `chant --help`.
   **Should see:** a usage block listing `suggest`, `capture`, `run`, `verify`,
   `list`, `search`, `explain`, `invalidate`, `init`, `index`, `status`,
   `doctor`, `bench`.
   **PASS:** the lifecycle (suggest/capture/run/verify) is clearly the headline.
   **FAIL:** command errors or prints nothing. **CONFUSED:** can't tell where to
   start.

2. **Action:** run `chant init`.
   **Should see:** `chant init in <dir>` and `created chant.yml`,
   `recipes/.gitkeep`, the chant skill, and a `.gitignore` entry for `.chant/`.
   **PASS:** files created and a "Next:" hint shown. **FAIL:** non-zero exit.
   **CONFUSED:** unclear what was created or what to do next.

3. **Action:** create a trivial script the enchantment will run, e.g. a file
   `hello.sh` containing `echo "hello $1" && echo ok > out.txt`. Then capture:
   `chant capture --id say-hello --task "say hello to a name" --command "sh hello.sh {{name}}" --verifier "test -f out.txt"`.
   **Should see:** `captured recipe "say-hello" (v1) at recipes/say-hello` and a
   hint to run `chant verify say-hello`.
   **PASS:** capture succeeds and points to verify. **FAIL:** capture errors.
   **CONFUSED:** unsure whether the enchantment is trusted yet (it should not be).

4. **Action:** run `chant suggest --task "say hello to a customer" --json`.
   **Should see:** JSON with `"match_found": true` and a hit `"id": "say-hello"`
   carrying `"verifier_exists": true`, `"trusted"` absent/false, and a
   `reuse_command`.
   **PASS:** the just-captured enchantment is surfaced as a candidate, not
   trusted. **FAIL:** no match / wrong id. **CONFUSED:** can't tell the hit is
   "candidate, not trusted."

5. **Action:** run `chant verify --input name=World say-hello`.
   **Should see:** `✓ say-hello verified — trusted` (and exit 0).
   **PASS:** verify runs the procedure + verifier and reports trusted; a follow-up
   `chant list` shows `1 run(s) 100% ok`. **FAIL:** verify errors or reports not
   trusted despite the verifier passing. **CONFUSED:** unclear that trust was
   just established here and not at capture/suggest.

### Scenario B — An enchantment that should NOT be trusted

**Goal:** confirm the verifier-first guard: a hit whose verifier fails is never
trusted.

1. **Action:** capture an enchantment with a verifier that always fails:
   `chant capture --id always-fails --task "demo a failing verifier" --command "echo ran" --verifier "false"`.
   **Should see:** capture succeeds; hint to verify.
   **PASS:** created. **FAIL:** capture errors. **CONFUSED:** thinks it is
   already usable.

2. **Action:** run `chant suggest --task "demo a failing verifier" --json`.
   **Should see:** a hit for `always-fails` with `verifier_exists: true` and no
   `trusted: true`.
   **PASS:** surfaced as a candidate. **FAIL:** not surfaced. **CONFUSED:**
   assumes a hit means it works.

3. **Action:** run `chant verify --json always-fails`.
   **Should see:** JSON with `"trusted": false` and `message` "verifier did NOT
   pass — result is NOT trusted; repair or invalidate"; process exits non-zero;
   `recommended_next_command` is `chant invalidate always-fails`.
   **PASS:** the result is explicitly NOT trusted and the tool says what to do.
   **FAIL:** reports trusted, or exits 0 as if fine. **CONFUSED:** can't tell the
   result is untrustworthy.

4. **Action:** run `chant invalidate --reason "verifier fails" always-fails`,
   then `chant list`.
   **Should see:** the enchantment marked `(stale)` in the listing.
   **PASS:** stale flag visible; tester understands re-verify would re-bless it.
   **FAIL:** still shown active. **CONFUSED:** unclear what stale means or how to
   recover.

### Scenario C — Browse and understand the library

**Goal:** a tester can survey what is cached and read a single enchantment card.

1. **Action:** run `chant list`.
   **Should see:** each enchantment with version, run count, success %, and a
   `(stale)` tag where applicable; or `no recipes yet` on an empty library.
   **PASS:** the inventory is legible at a glance. **FAIL:** errors. **CONFUSED:**
   can't tell which are trustworthy.

2. **Action:** run `chant search "hello"`.
   **Should see:** a ranked list with score components `(lex .., signal ..,
   ok ..)`.
   **PASS:** results ranked and the score breakdown is shown. **FAIL:** errors or
   empty when a relevant enchantment exists. **CONFUSED:** doesn't understand the
   score columns (acceptable to note as a docs gap).

3. **Action:** run `chant explain say-hello`.
   **Should see:** the card: id/version/status, description, when-to-use
   patterns, the `what to do` command, the verifier command, and a metrics line.
   **PASS:** the tester understands when to use it and how it is verified.
   **FAIL:** errors or `recipe not found`. **CONFUSED:** can't find the verifier
   or applicability info.
   **GOTCHA to note:** `chant explain --json say-hello` prints the raw recipe
   struct with capitalized Go field names (`ID`, `WhatToDo`), unlike the
   snake_case JSON from other commands — flag if it confuses you.

4. **Action:** run `chant doctor` then `chant status`.
   **Should see:** doctor prints `[ok]`/`[warn]` checks ending in
   `doctor: no blocking issues.`; status writes `.chant/STATUS.md` and reports
   counts.
   **PASS:** the tester can assess library health and trust coverage. **FAIL:**
   doctor exits non-zero on a healthy fresh repo. **CONFUSED:** unclear what a
   `warn` (e.g. a missing verifier) implies for trust.

### Scenario D — Reuse the shipped CSV revenue enchantment (Demo 1)

**Goal:** experience the canonical demo without writing any code.

1. **Action:** from the chant repo root (which ships
   `recipes/csv-revenue-by-channel/`), run
   `chant suggest --task "compute revenue by channel" --files "orders.csv" --columns "utm_source,amount" --json`.
   **Should see:** top hit `csv-revenue-by-channel`, `verifier_exists: true`,
   `reasons` mentioning file and column signals.
   **PASS:** the enchantment is found across a Shopify-flavored schema. **FAIL:**
   no/wrong hit. **CONFUSED:** unclear why it matched.

2. **Action:** repeat with a Stripe-flavored schema:
   `--columns "source,price"`.
   **Should see:** still `csv-revenue-by-channel` on top (column aliases absorb
   the drift).
   **PASS:** schema drift handled. **FAIL:** drops the match. **CONFUSED:**
   surprised the same recipe matches different headers.

3. **Action:** run `chant verify --json csv-revenue-by-channel`.
   **Should see:** the verifier output (`OK: revenue-by-channel totals match…`)
   and an outcome with `"trusted": true`; `revenue_by_channel.json` exists in the
   recipe dir.
   **PASS:** reuse is trusted only after the verifier passes. **FAIL:** not
   trusted despite a passing verifier. **CONFUSED:** unclear that this step is
   what conferred trust.

### Scenario E — Bench as a confidence check

**Goal:** a tester confirms the suite is green and understands what it proves.

1. **Action:** run `chant bench`.
   **Should see:** `== suite: retrieval (N/N passed) ==` and
   `== suite: e2e (M/M passed) ==`, ending in `bench: all scenarios passed.`
   **PASS:** all scenarios pass and the names convey what each proves (a hit, a
   true-negative, the trust gate). **FAIL:** any scenario fails on a clean
   checkout. **CONFUSED:** can't tell what the suite is asserting.

2. **Action:** run `chant bench --suite=retrieval --json` and read the
   `summaries[].results[]` entries.
   **Should see:** structured pass/fail with `detail` strings.
   **PASS:** machine-readable and matches the human run. **FAIL:** JSON malformed
   or disagrees with the human output. **CONFUSED:** unclear how to map a result
   id to a scenario.

---

## Scenario count

- Automated retrieval: **8** (RET-001..RET-004 existing, RET-005..RET-008 new)
- Automated e2e: **6** (E2E-csv-revenue-by-channel existing,
  E2E-001..E2E-005 new)
- Naive-user: **5** flows (A–E), **20** numbered steps total.

**Total automated scenarios: 14** (4 existing + 10 new).
