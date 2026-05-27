# `chant bench`

> Run the validation suites. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdBench`) +
> [`internal/bench/bench.go`](../../internal/bench/bench.go).

## What it does

Runs chant's shipped validation suites and exits `1` on any scenario failure —
the same CI muscle memory as `coherence bench`. Two suites prove the core
thesis:

- **`retrieval`** — a synthetic recipe set + queries, asserting which recipe
  ranks first and whether it clears the match threshold. Includes a **true
  negative**: an unrelated query must NOT match.
- **`e2e`** — runs each library recipe's procedure + verifier and asserts the
  **verifier-first trust gate** (a recipe counts as a pass only when its
  verifier establishes trust). Recipes without a verifier or without an example
  are skipped (reported as a pass with a skip note) so the suite stays green on
  a fresh library.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--suite` | `all` | `retrieval`, `e2e`, or `all`. |
| `--json` | false | emit the suite summaries as JSON. |

## Example (human)

```bash
chant bench
```

```text
== suite: retrieval (4/4 passed) ==
  [PASS] RET-001  hit on similar revenue task — top=csv-revenue-by-channel @ 0.88
  [PASS] RET-002  no false hit on unrelated task — correctly returned no match
  [PASS] RET-003  refund task routes to refund recipe — top=refund-chargeback-threat @ 0.45
  [PASS] RET-004  column signals disambiguate revenue vs normalize — top=csv-revenue-by-channel @ 0.98

== suite: e2e (1/1 passed) ==
  [PASS] E2E-csv-revenue-by-channel run+verify csv-revenue-by-channel — verifier passed → trusted

bench: all scenarios passed.
```

The retrieval scenarios run over a built-in synthetic recipe set
(`csv-revenue-by-channel`, `refund-chargeback-threat`, `normalize-orders-export`),
so the retrieval suite is independent of the on-disk library. The e2e suite runs
over the **actual** recipes under `recipes/`.

## Example (JSON)

```bash
chant bench --json
```

```json
{
  "failed": 0,
  "summaries": [
    {
      "suite": "retrieval",
      "total": 4,
      "passed": 4,
      "failed": 0,
      "results": [
        {"id": "RET-001", "name": "hit on similar revenue task", "suite": "retrieval", "pass": true, "detail": "top=csv-revenue-by-channel @ 0.88"},
        {"id": "RET-002", "name": "no false hit on unrelated task", "suite": "retrieval", "pass": true, "detail": "correctly returned no match"},
        {"id": "RET-003", "name": "refund task routes to refund recipe", "suite": "retrieval", "pass": true, "detail": "top=refund-chargeback-threat @ 0.45"},
        {"id": "RET-004", "name": "column signals disambiguate revenue vs normalize", "suite": "retrieval", "pass": true, "detail": "top=csv-revenue-by-channel @ 0.98"}
      ]
    },
    {
      "suite": "e2e",
      "total": 1,
      "passed": 1,
      "failed": 0,
      "results": [
        {"id": "E2E-csv-revenue-by-channel", "name": "run+verify csv-revenue-by-channel", "suite": "e2e", "pass": true, "detail": "verifier passed → trusted"}
      ]
    }
  ]
}
```

## JSON shape

A top-level `{summaries[], failed}`. Each `Summary` carries `suite`, `total`,
`passed`, `failed`, and `results[]`; each `Result` carries `id`, `name`,
`suite`, `pass`, and a `detail` string. `failed` is the total failed across all
suites; a non-zero value makes `chant bench` exit `1`.

## What the scenarios assert

| Scenario | Assertion |
|---|---|
| `RET-001` | a paraphrased revenue task with matching files+columns ranks `csv-revenue-by-channel` first. |
| `RET-002` | an unrelated task (k8s TLS rotation) returns **no** match — the threshold guards against false positives. |
| `RET-003` | a chargeback/refund task routes to `refund-chargeback-threat`. |
| `RET-004` | column signals disambiguate revenue from a normalize recipe. |
| `E2E-<id>` | each recipe with a verifier + example runs and verifies, and is trusted only on a passing verifier. |
