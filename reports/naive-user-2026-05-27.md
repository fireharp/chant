# chant — Naive-User Evaluation Report

- **Date:** 2026-05-27
- **Evaluator role:** First-time user, docs-only onboarding, then hands-on.
- **Binary under test:** `/Users/fireharp/Prog/Harness/chant/bin/chant` (copied to `/tmp/chant-bin`, `chant version` → `chant dev`). Not rebuilt.
- **Scratch dirs:** `/tmp/chant-test.qv5mVB` (git-init'd, primary), `/tmp/chant-empty.0Kte0M` (empty-library probes).
- **Method:** (1) learn from `README.md` + `chant help` + `docs/commands/*` only; (2) run naive-user Scenarios A–E from `docs/testing/scenarios.md`; (3) adopt-from-zero; (4) probe the `--json` contract and flag ordering; (5) try to break it.

---

## 0. Two-minute onboarding impression (docs-only)

**Could I understand what it's for in under 2 minutes? Yes.** The tagline ("cache the tested method, not the answer") plus the `retrieve → adapt → execute → verify → accept` diagram and the 30-second two-command demo (`suggest` → `verify`) make the value proposition land fast. The verifier-first trust gate is the clearest idea in the docs.

**What was confusing / friction in the first 2 minutes:**
- The **flag-ordering warning** appears 3+ times (README lines 119–123 and 272–274, plus every command doc). Repeating a "gotcha" this loudly signals an unintuitive API and primed me to expect breakage. (It turns out the binary no longer has this problem — see Issue #1.)
- "recipe" vs "enchantment" synonym is explained twice; harmless but adds noise for a newcomer.
- The README's `30-second demo` opens with `go build -o bin/chant ./cmd/chant`, implying you must build before anything — there's an `Install` section below it, but a newcomer reading top-to-bottom assumes a build step is mandatory.

---

## 1. Per-scenario results (A–E)

| Step | Command (exact) | Expected (per docs) | Actual | Verdict |
|---|---|---|---|---|
| A1 | `chant --help` | usage block, lifecycle headlined | usage block, lifecycle is the first group; exit 0 | **PASS** |
| A2 | `chant init` | creates chant.yml, recipes/.gitkeep, skill, .gitignore (+.chant/), "Next:" hint | all 4 created, "Next:" hint shown, exit 0 | **PASS** |
| A3 | `chant capture --id say-hello --task "say hello to a name" --command "sh hello.sh {{name}}" --verifier "test -f out.txt"` | captured v1, hint to verify | captured v1, hint shown | **PASS (capture)** but see A5 |
| A4 | `chant suggest --task "say hello to a customer" --json` | `match_found:true`, hit `say-hello`, `verifier_exists:true`, `trusted:false`, reuse_command | exactly that (confidence 0.53) | **PASS** |
| A5 | `chant verify --input name=World say-hello` | `✓ say-hello verified — trusted`, exit 0; `list` → `1 run(s) 100% ok` | **`✗ say-hello NOT verified`, exit 1** (script not in recipe dir). After re-capturing with `--entrypoint-src hello.sh`: `✓ verified — trusted`, exit 0, list `1 run(s) 100% ok` | **FAIL as written / PASS after fix** — see Issue #2 |
| B1 | `chant capture --id always-fails ... --verifier "false"` | captured, hint to verify | captured, exit 0 | **PASS** |
| B2 | `chant suggest --task "demo a failing verifier" --json` | hit `always-fails`, `verifier_exists:true`, not trusted | exactly that (confidence 0.70) | **PASS** |
| B3 | `chant verify --json always-fails` | `trusted:false`, message "verifier did NOT pass…", exit non-zero, `recommended_next_command: chant invalidate always-fails` | exactly that, exit 1 | **PASS** |
| B4 | `chant invalidate --reason "verifier fails" always-fails` then `chant list` | listed as `(stale)` | `marked always-fails stale...`; list shows `always-fails ... 0% ok (stale)` | **PASS** |
| C1 | `chant list` | per-recipe version/runs/success%/(stale) | legible, stale flag shown | **PASS** |
| C2 | `chant search "hello"` | ranked list with `(lex .., signal .., ok ..)` | `0.70 say-hello (lex 1.00, signal 0.00, ok 1.00)` + `0.00 always-fails ...` | **PASS** |
| C3 | `chant explain say-hello` | card: id/version/status, description, when-to-use, what-to-do, verifier, metrics | all present and legible | **PASS** |
| C3-gotcha | `chant explain --json say-hello` | docs say **capitalized Go field names** (`ID`, `WhatToDo`) | **snake_case** (`id`, `what_to_do`, `when_to_use`) — docs are stale | **PASS (tool) / doc mismatch** — Issue #3 |
| C4 | `chant doctor` then `chant status` | doctor `[ok]/[warn]` ending "no blocking issues."; status writes STATUS.md + counts | doctor all `[ok]`, "no blocking issues."; status wrote STATUS.md (2 recipes, 1 active, 1 stale) | **PASS** |
| D1 | `chant suggest --task "compute revenue by channel" --files "orders.csv" --columns "utm_source,amount" --json` | top hit `csv-revenue-by-channel`, `verifier_exists:true`, file+column reasons | exactly that, confidence 1, all 3 reasons | **PASS** |
| D2 | same with `--columns "source,price"` (Stripe drift) | still `csv-revenue-by-channel` on top | top id `csv-revenue-by-channel`, confidence 1 | **PASS** |
| D3 | `chant verify --json csv-revenue-by-channel` | `trusted:true`, artifact exists | `trusted:true`, exit 0, `revenue_by_channel.json` created | **PASS** |
| E1 | `chant bench` | retrieval N/N + e2e M/M, ending "bench: all scenarios passed." | retrieval 4/4, e2e 3/3, "bench: all scenarios passed.", exit 0 | **PASS** |
| E2 | `chant bench --suite=retrieval --json` | structured `summaries[].results[]` with `detail` | well-formed JSON matching human output | **PASS** |

**Scenario tally:** 18 of 20 steps PASS outright; A5 FAILs as literally written in the doc (recoverable); C3-gotcha is a PASS-but-docs-wrong. The verifier-first thesis (B3, D3, artifact-gate probe) is rock-solid.

---

## 2. Adopt-from-zero (docs only)

Using only README + `chant help` + `docs/commands/`, I ran `init` → captured my own `say-hello` recipe → `suggest` → `verify` → `list`/`explain`/`search`. **The docs got me ~90% there.** The one place I had to guess: the capture docs list `--entrypoint-src` as a flag but **none of the quickstart paths tell you that a recipe whose `--command` references a local script will not find that script unless you copy it in** with `--entrypoint-src`. I only discovered this because verify failed (Issue #2). Everything else was directly answerable from the docs.

---

## 3. JSON contract & flag-ordering probe

- **Flag ordering — the big one.** README + every command doc insist flags after the positional id are "silently ignored." **This is false in the current binary.** Both orderings produce identical correct output:
  - `chant verify --json csv-revenue-by-channel` and `chant verify csv-revenue-by-channel --json` → both emit JSON.
  - Same for `run`, `explain` (`explain say-hello --json` → JSON), `search` (`search "hello" --json` → JSON), `invalidate` (`invalidate say-hello --json` → JSON).
- **snake_case consistency.** Verified across `suggest`, `search`, `verify`, `run`, `invalidate`, `list`, `status`, `bench`, **and `explain --json`** — all keys are snake_case. The documented "explain is the one capitalized exception" no longer holds. `grep '"[A-Z]'` on `explain --json csv-revenue-by-channel` matched only a value, zero keys.
- **Field self-explanatory?** Mostly yes. `trusted`, `verifier_exists`, `match_found`, `recommended_next_command`, `reuse_command` are clear. The `hits[]` `confidence` rounds to 2 decimals as documented.
- **Omitted-vs-empty inconsistency.** On an empty library, `suggest --json` **omits `match_found` entirely** (rather than `false`) and `search --json` omits `hits` (rather than `[]`). The contract table says `suggest` emits `match_found`; an agent must treat "absent" as false. Minor but a real edge for the documented "agent reads three fields" loop.

---

## 4. Break-it / confusion probes

| Probe | Result | Verdict |
|---|---|---|
| Capture **without** verifier (human) | prints `⚠ no verifier set — add one so reuse can be trusted...` | clear |
| Capture **without** verifier (`--json`) | **no warning field at all**; `message` = "recipe captured — verify it to establish trust" | **confusing for agents** — Issue #4 |
| `verify` a no-verifier recipe | `chant: recipe "no-verifier" has no verifier ... — cannot establish trust`, exit 1 | clear |
| `verify --json` a no-verifier recipe | **error printed as plain text on stderr, NOT JSON**, exit 1, empty stdout | **breaks --json contract** — Issue #5 |
| Verifier fails (`--verifier false`) | `trusted:false`, exit 1, recommends invalidate | PASS (core gate) |
| Verifier exits 0 but expected artifact missing | `trusted:false`, exit 1 | PASS (artifact gate works) — but `message` mislabels it "verifier did NOT pass" (Issue #6) |
| Missing `{{var}}` input (`verify --json say-hello` w/ no `--input`, no example) | `chant: recipe "say-hello" is missing inputs: name`, exit 1, plain-text stderr | fail-fast works; same `--json`-bypass as Issue #5 |
| Empty library: `search` / `search --json` / `list` / `suggest --json` | graceful: `no recipes to search.` / minimal JSON / `no recipes yet...` / miss payload; all exit 0 | PASS |

---

## 5. Ranked clarity / usability issues (most-confusing first)

### Issue #1 — Docs loudly warn about a flag-ordering gotcha the tool no longer has (HIGH, doc fix)
README (lines 119–123, 272–274) and all of `verify.md`, `run.md`, `explain.md`, `search.md`, `invalidate.md`, and `scenarios.md` warn: *"Put flags before the id; a trailing `--json` is silently ignored."* The current binary parses flags in **either** position. So the docs train newcomers to fear a non-problem, and the most-emphasized "gotcha" in the entire doc set is now misinformation.
**Fix:** Remove every "flags must come before the id" warning, or replace with a one-liner "flags work in any position." This single change removes the most repeated friction in onboarding.

### Issue #2 — The headline quickstart (Scenario A) FAILS as written: missing `--entrypoint-src` step (HIGH, doc fix)
`docs/testing/scenarios.md` A3 tells the user to create `hello.sh` in the working dir and capture with `--command "sh hello.sh {{name}}"`, then promises A5 `chant verify` shows `✓ verified — trusted`. It does not — verify runs in `recipes/say-hello/` where `hello.sh` does not exist, so it prints `✗ NOT verified`, exit 1. The fix is `--entrypoint-src hello.sh` (which copies the script into the recipe dir), but the scenario never mentions it. A newcomer following the canonical "from zero to a reused enchantment" flow hits a hard FAIL on the very step that's supposed to demonstrate success.
**Fix:** Add `--entrypoint-src hello.sh` to A3's capture command (and to `capture.md`'s narrative for script-based recipes), or make the example a self-contained inline command (e.g. `--command 'echo "hello {{name}}" && echo ok > out.txt'`) that needs no external file.

### Issue #3 — `explain --json` docs describe capitalized Go field names; tool emits snake_case (MEDIUM, doc fix)
`docs/commands/explain.md` has a prominent "Note" and a full JSON example using `ID`, `WhenToUse`, `WhatToDo`, etc., calling it "the one command whose JSON does not follow snake_case." `scenarios.md` C3 repeats this as a GOTCHA. The actual output is fully snake_case and consistent with every other command. The docs undersell a real improvement and could make an agent author write fragile capitalized-key parsing.
**Fix:** Update `explain.md` example + note and `scenarios.md` C3 to show snake_case; delete the "one exception" caveat. The contract is now uniformly snake_case — say so.

### Issue #4 — `capture --json` without a verifier gives no machine-readable "untrustable" signal (MEDIUM, CLI fix)
Human mode warns `⚠ no verifier set...`, but the `--json` payload has no `warning`/`has_verifier:false`/`blocking` field — just `captured:true` + "verify it to establish trust." An agent capturing via JSON cannot tell it just wrote a recipe that `verify` will refuse to bless. This contradicts the verifier-first ethos at exactly the hook (capture) where it matters.
**Fix:** Add a field to capture's JSON when no verifier is set, e.g. `"verifier_exists": false` and/or a `"message": "captured WITHOUT a verifier — reuse can never be trusted"`. Cheap, contract-consistent.

### Issue #5 — Errors bypass `--json` (HIGH for agents, CLI fix)
Every error path (`missing inputs: X`, `has no verifier ... cannot establish trust`, presumably others) prints a `chant: ...` line to **stderr in plain prose** and leaves stdout empty, even when `--json` was requested. The README sells "every command accepts --json … agents decide what to do next without parsing prose" and the contract reserves `blocking_error`/`suggested_commands` for exactly this — but they're never emitted. An agent in `--json` mode gets exit 1 + empty stdout and must fall back to scraping stderr prose.
**Fix:** When `--json` is set, emit a JSON error payload on stdout: `{"subcommand":..., "exit_code":1, "trusted":false, "blocking_error":true, "message":"recipe X is missing inputs: name", "recommended_next_command":"..."}`. This is the contract's stated purpose.

### Issue #6 — "verifier did NOT pass" message fires even when the verifier passed and only the artifact gate failed (LOW, CLI/message fix)
Captured a recipe with `--verifier "true" --expect-artifacts never_created.json`. The verifier command exits 0; the artifact is missing; result is correctly `trusted:false`. But `message` reads "verifier did NOT pass — result is NOT trusted." A user would debug their verifier command when the real problem is a missing/misnamed artifact.
**Fix:** Distinguish the two branches in the message, e.g. "verifier passed but expected artifact 'never_created.json' is missing — result is NOT trusted."

### Issue #7 — `verify` (and `run`) human mode does not print the procedure's stdout that the docs show (LOW, doc-or-CLI fix)
`verify.md`'s human example shows the procedure output (`{ "direct": 25.5, ...}` and `OK: ...`). In practice `chant verify say-hello` printed only `✓ ... verified — trusted (3ms)` with no procedure/verifier output; `chant run` does print stdout. A newcomer expecting to see what the recipe produced sees nothing from verify.
**Fix:** Either stream verifier/procedure stdout in human verify (matching the doc example) or update `verify.md` to clarify that human verify is quiet by default.

### Issue #8 — Human-mode failing `verify` doesn't tell the user what to do next (LOW, CLI fix)
`chant verify always-fails` (human) prints `✗ always-fails NOT verified — do not trust this result.` — but unlike the JSON payload it omits the `recommended_next_command: chant invalidate ...`. The human user is told what NOT to do but not the recovery step.
**Fix:** Append the next-step hint to human failure output (e.g. "→ run `chant invalidate always-fails` or repair the recipe").

---

## 6. Doc-vs-behavior mismatches (summary)

1. **Flag ordering** — docs say flags-after-id are ignored; binary accepts both orders. (Issue #1)
2. **`explain --json`** — docs say capitalized Go field names; binary emits snake_case. (Issue #3)
3. **Scenario A quickstart** — promises `verify` PASS, but the documented steps produce a FAIL (missing `--entrypoint-src`). (Issue #2)
4. **`capture --json` no-verifier warning** — present in human docs/output, absent from JSON. (Issue #4)
5. **`--json` error payloads** — README promises prose-free machine output; errors are prose-on-stderr. (Issue #5)
6. **Empty-library `suggest --json`** — contract table lists `match_found` for suggest; it's omitted when empty. (Section 3)

(Note: RET-005..008 and E2E-001..005 in `scenarios.md` are explicitly marked "(new)" / not-yet-wired; `chant bench` runs only RET-001..004 + the live e2e set. This matches the doc's own "GAP for the lead" notes — recorded as expected, not a mismatch.)

---

## 7. Verdict: would a newcomer succeed unaided?

**Partially.** The core product works exactly as advertised — the verifier-first gate, suggest→verify lifecycle, the shipped CSV demo (incl. schema-drift), bench, doctor, and the JSON contract are all solid and genuinely impressive. A newcomer running the README's 30-second demo (the shipped CSV recipe) succeeds cleanly. But a newcomer following the **"capture your own recipe" quickstart (Scenario A)** hits a hard FAIL on the verify step because the docs omit `--entrypoint-src`, and they'll waste time fearing the (now non-existent) flag-ordering gotcha. Agents wiring against `--json` will trip over errors that bypass JSON.

**Top 3 to fix:**
1. **Fix the Scenario A quickstart** (add `--entrypoint-src`, or use a self-contained inline command) so the first "capture your own" flow actually reaches `✓ verified — trusted`. (Issue #2)
2. **Delete the flag-ordering warnings** everywhere — they describe a bug the binary no longer has and are the most-repeated friction in the docs. (Issue #1)
3. **Make `--json` honor the contract on error paths** (emit a JSON `blocking_error` payload instead of prose-on-stderr), and add a no-verifier signal to `capture --json`. (Issues #5, #4)
