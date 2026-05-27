# `chant verify`

> The trust gate. Run a recipe's procedure and verifier; a reuse result is
> "trusted" ONLY when the verifier passes. Source:
> [`cmd/chant/commands.go`](../../cmd/chant/commands.go) (`cmdVerify`) +
> [`internal/runner/runner.go`](../../internal/runner/runner.go) (`Verify`).

## What it does

This is the command that establishes trust. By default it first runs the
procedure (`what_to_do.command`), then runs the verifier
(`verification.command`), and treats the result as **trusted only when the
verifier exits 0** *and* every `verification.expected_artifacts` path exists.
The verification is recorded as the trust event in the recipe's `metrics`, and a
passing verifier on a stale recipe **re-blesses** it back to `active`.

`verify` is the only command that can emit `trusted: true`. A retrieved recipe
is a candidate; the verifier blesses it — `retrieve → adapt → execute → verify →
accept`, never `retrieve → trust`.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--input k=v` | none | repeatable; fills `{{k}}` placeholders in both the procedure and the verifier. |
| `--run` | `true` | run the procedure before verifying. Set `--run=false` to verify pre-existing artifacts only. |
| `--timeout` | `60s` | timeout for the procedure and verifier (Go duration). |
| `--json` | false | emit the JSON outcome contract. |

The recipe id is a positional argument; flags work in any position
(`chant verify <id> --json` and `chant verify --json <id>` are equivalent).

If the recipe has `examples`, the first example's `input` is used as the default
`{{input}}` and `{{input_file}}` when those inputs are not supplied. A recipe
with no verifier (no `verification.command` and no `expected_artifacts`) errors:
trust cannot be established.

## Example — pass (JSON)

```bash
chant verify csv-revenue-by-channel --json
```

```json
{
  "subcommand": "verify",
  "match_found": false,
  "recipe_id": "csv-revenue-by-channel",
  "version": 1,
  "executed": true,
  "exit_code": 0,
  "verifier_ran": true,
  "trusted": true,
  "runtime_ms": 126,
  "message": "verifier passed — result is trusted"
}
```

## Example — pass (human)

```text
{
  "direct": 25.5,
  "facebook": 200.0,
  "google": 150.0
}
OK: revenue-by-channel totals match expected ground truth
✓ csv-revenue-by-channel verified — trusted (114ms)
```

## Example — fail

When the verifier does not pass, the result is **not** trusted, the command
prints `✗ <id> NOT verified — do not trust this result.` and exits 1. The JSON
payload sets `trusted: false`, a `message` of `"verifier did NOT pass — result
is NOT trusted; repair or invalidate"`, and
`recommended_next_command: "chant invalidate <id>"`.

## JSON shape

`subcommand: "verify"`, `match_found` (always present, `false`), `recipe_id`,
`version`, `executed`, `verifier_ran: true`, `exit_code`, `trusted` (the
verdict), `runtime_ms`, `message`, and — on a verifier failure —
`recommended_next_command`. A verifier *failure* is a normal payload with
`trusted: false`, not an error. A command-level failure (unknown recipe, no
verifier configured) instead returns the error contract — `{"subcommand":
"verify", "blocking_error": true, "message"}` on stdout, exit 1. See the
[outcome contract](../README.md#json-outcome-contract).

## Trust semantics

| Condition | `trusted` |
|---|---|
| verifier command exits 0 **and** all expected artifacts exist | `true` |
| verifier command exits non-zero | `false` |
| an expected artifact is missing after the run | `false` |
| recipe has no verifier at all | error (cannot establish trust) |

A passing verify increments `metrics.runs` and stamps `last_success_at`; a
failing verify increments `metrics.failures` and stamps `last_failure_at`. This
track record feeds the `weight_success_rate` term in retrieval ranking.
