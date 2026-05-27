# chant

A recipe cache for coding agents: **cache the tested method, not the answer.**

chant turns successful agent work into reusable, **versioned, verified**
procedures. It is the complement to
[coherence](https://github.com/fireharp/coherence): **coherence catches what
broke (drift); chant captures what worked and is reusable (recipes).** Both are
harnesses for agent-edited repos.

> **Memoization for agentic procedures, not answers.**

chant does not cache LLM responses, plans, or final outputs. It caches the
*tested way* of solving a recurring task — an executable procedure with
applicability conditions, a verifier, and a reuse track record. The next time a
similar task appears, an agent retrieves the recipe, adapts it to the new
inputs, runs it, and **only trusts the result if the verifier passes.**

A chant recipe is also called an **enchantment** — chant *casts* enchantments.
"Enchantment" is the product/narrative noun; the on-disk form stays
`recipe.yaml` under `recipes/` because it reads plainly. The two words are
synonyms throughout chant (the code type is `recipe.Recipe`); this README uses
"recipe" for the concrete file and "enchantment" where the flavor helps.

> **Algorithm reference**: [`docs/`](docs/README.md) has a long-form page for
> every command, with flags, the JSON outcome contract, and real CLI output.
> Start there if you want the precise contract for a single command.

## The verifier-first trust gate

This is the central principle, and it is what separates a useful cache from a
wrong-answer amplifier:

```text
retrieve → adapt → execute → verify → accept     (chant)
retrieve → trust                                  (the failure mode chant avoids)
```

A retrieved recipe is a **candidate**. A reuse result is **trusted** only after
its verifier passes. A cache hit means *this method might apply* — never *this
answer is correct*. Every JSON payload carries a `trusted` boolean that is
`true` **only** after a passing verifier; retrieval (`suggest`, `search`) and
bare execution (`run`) never set it.

## How is this different?

| Tool / category | Positioning | chant differentiation |
| --- | --- | --- |
| LLM semantic / prompt caches ([GPTCache](https://github.com/zilliztech/gptcache), LangChain cache, OpenAI prompt caching) | Cache the model's request/response tokens for cost and latency. | Caches the executable *method*, not the model IO. The cached object runs and is re-verified against new inputs. |
| Agentic plan caching ([test-time plan caching](https://arxiv.org/html/2506.14852v1), [orra plan caching](https://github.com/orra-dev/orra)) | Store structured plan templates from completed runs and adapt them for similar requests. | Caches a *verified executable* procedure, not a natural-language plan. A plan can be plausible and wrong; a chant recipe is gated on a passing verifier. |
| Agent skill libraries ([Voyager](https://github.com/MineDojo/Voyager), [SoK: Agentic Skills](https://arxiv.org/html/2602.20867v1)) | Store and retrieve executable code skills with self-verification. | Adds explicit **applicability conditions**, **versioning + invalidation**, and a **verifier-first trust contract** as first-class recipe fields, surfaced over a stable JSON outcome contract for hooks. |
| Incremental / self-adjusting computation ([Salsa](https://github.com/salsa-rs/salsa), [Adapton](https://github.com/Adapton/adapton.rust), [Mandala](https://amakelov.github.io/blog/pl/)) | Memoize a computation graph and recompute only affected parts when inputs change. | Not a runtime memoizer. chant is hook-based and language-agnostic: it caches the *recipe* an agent reuses across tasks, not a function's per-input result. |

## 30-second demo

The repo ships one zero-dependency recipe, `csv-revenue-by-channel`, that
computes ecommerce revenue by marketing channel from a CSV, robust to
column-name drift across Shopify / Stripe / custom exports.

```bash
# build the CLI
go build -o bin/chant ./cmd/chant

# 1. Before writing new code: does a recipe already solve this?
./bin/chant suggest --task "compute revenue by channel" \
  --files orders.csv --columns utm_source,amount --json
```

```json
{
  "subcommand": "suggest",
  "match_found": true,
  "hits": [
    {
      "id": "csv-revenue-by-channel",
      "version": 1,
      "description": "Compute ecommerce revenue by channel from CSV-like exports, robust to column-name drift across Shopify/Stripe/custom exports.",
      "confidence": 1,
      "status": "active",
      "verifier_exists": true,
      "reasons": [
        "task text overlaps recipe description/patterns",
        "input files match recipe file signal",
        "input columns satisfy recipe column aliases"
      ],
      "reuse_command": "chant verify csv-revenue-by-channel   # run + verify before trusting"
    }
  ],
  "exit_code": 0,
  "trusted": false,
  "recommended_next_command": "chant verify csv-revenue-by-channel   # run + verify before trusting"
}
```

`match_found` is `true`, a verifier exists, and `trusted` is `false` — a
candidate, not a blessing. Follow the `reuse_command`:

```bash
# 2. Reuse verifier-first: run the procedure, then run the verifier.
./bin/chant verify --json csv-revenue-by-channel
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

`trusted: true` only now, after the verifier ran and passed. That is the whole
thesis in two commands. (`chant verify <id> --json` and `chant verify --json
<id>` are equivalent — flags work in any position.)

## Capture your own recipe

After an agent solves a recurring task, capture the procedure so the next
similar task reuses it. A recipe runs **inside its own directory**
(`recipes/<id>/`), so any script the `command` or `verifier` references must
live there — `--entrypoint-src <path>` copies a file into the recipe dir at
capture time. This end-to-end walkthrough is verified working:

```bash
# 1. The procedure the agent just got working.
printf '#!/bin/sh\necho "hello $1"\n' > hello.sh

# 2. Capture it. --entrypoint-src copies hello.sh into recipes/greet/ as greet.sh,
#    so the in-dir command and verifier can find it.
./bin/chant capture --id greet --task "greet a name" \
  --command 'sh greet.sh {{name}}' --entrypoint-src hello.sh --entrypoint greet.sh \
  --verifier 'sh -c "test \"$(sh greet.sh world)\" = \"hello world\""'

# 3. Verify to establish the first trust event.
./bin/chant verify greet --input name=world
# → ✓ greet verified — trusted
```

If you capture without copying the referenced script in (or without a
verifier), `chant verify` will fail because the script isn't in the recipe dir —
copy it with `--entrypoint-src`.

## Requirements

- Go 1.26+ (to build)
- Git (for repo-root discovery; chant also works in a non-git scratch directory)
- A shell (`sh`) to run recipe procedures and verifiers
- Whatever runtime a given recipe declares (the shipped CSV recipe needs
  `python3`)

## Install

```bash
# latest release binary; writes ~/.local/bin/chant
curl -fsSL https://github.com/fireharp/chant/releases/latest/download/install.sh | sh

# fallback: install from the latest tagged source
go install github.com/fireharp/chant/cmd/chant@latest

# local development build from a clone
go build -o bin/chant ./cmd/chant
```

## The recipe card

A recipe (enchantment) is a directory under `recipes/<id>/` containing a
`recipe.yaml` card plus any procedure and verifier files. The card is the cached
object — the applicability gate, the executable procedure, the verifier, and the
track record, in one versioned file. The shipped card:

```yaml
id: csv-revenue-by-channel
version: 1
kind: executable_recipe
description: Compute ecommerce revenue by channel from CSV-like exports, robust to column-name drift across Shopify/Stripe/custom exports.
when_to_use:
    task_patterns:
        - compute revenue by channel from csv
        - analyze ecommerce orders export
        - revenue breakdown by marketing channel
    tags:
        - csv
        - ecommerce
        - revenue
        - analytics
    input_signals:
        files:
            - '*.csv'
        columns_any:
            - [channel, source, utm_source]
            - [revenue, amount, price, total]
what_to_do:
    entrypoint: run.py
    command: python3 run.py {{input}}
    language: python
verification:
    command: python3 test_recipe.py
    expected_artifacts:
        - revenue_by_channel.json
invalidation:
    if_tests_fail: true
    if_columns_missing: true
dependencies:
    runtime: 'python: >=3.8'
examples:
    - input: examples/orders_shopify.csv
      output: revenue_by_channel.json
metrics:
    runs: 0
    failures: 0
status: active
```

The field groups mirror the reuse lifecycle. Source of truth:
[`internal/recipe/recipe.go`](internal/recipe/recipe.go).

| Field group | Field | Meaning |
| --- | --- | --- |
| top-level | `id` | recipe slug; the directory name and the handle every command takes. |
| | `version` | integer recipe version. Bumped by capturing a fresh recipe after invalidation. |
| | `kind` | `executable_recipe` (default), `patch_recipe`, or `workflow_recipe`. |
| | `description` | one-line summary; the primary lexical-match corpus. |
| | `status` | `active` or `stale`. Stale recipes stay retrievable but are penalized in ranking. |
| `when_to_use` | `task_patterns` | natural-language descriptions of tasks this recipe solves; matched lexically against a query. |
| | `tags` | free-form labels added to the lexical corpus. |
| | `input_signals.files` | globs the input file set should match (e.g. `*.csv`). |
| | `input_signals.columns_any` | a list of alias groups; each inner group is a set of acceptable names for one logical column. A query's columns "cover" the recipe when every group has at least one member present. |
| `what_to_do` | `entrypoint` | the script file inside the recipe dir (e.g. `run.py`). |
| | `command` | the templated shell command. `{{var}}` placeholders are filled from `--input k=v` at run time. |
| | `language` | informational runtime tag (`python`, `go`, `node`, `bash`, …). |
| `verification` | `command` | the verifier. **Exit 0 == passed.** |
| | `expected_artifacts` | paths (relative to the recipe dir) that must exist after a successful run for the result to be trusted. |
| `invalidation` | `if_tests_fail` / `if_columns_missing` / `if_dependency_changed` | declarative hints for when the recipe should be marked stale. |
| `dependencies` | `runtime` / `packages` | the runtime the recipe was verified against. |
| `fingerprints` | `recipe_code_hash` / `verifier_hash` / `schema_fingerprint` | content hashes used to detect drift in the recipe itself (computed by `chant capture`). |
| `examples` | `input` / `output` | recorded input/output pairs. The first example's `input` is the default `{{input}}` / `{{input_file}}` for `chant verify`. |
| `metrics` | `runs` / `failures` / `last_success_at` / `last_failure_at` | the reuse track record. Feeds the `weight_success_rate` term in ranking. A recipe with zero runs is given a `1.0` success rate so a freshly captured recipe is not penalized. |

A recipe **without** a verifier (no `verification.command` and no
`expected_artifacts`) can still be retrieved, but its reuse can never be
trusted — `chant verify` errors out, and `chant doctor` warns. A hit without a
verifier is just a guess.

### Enchantment metadata & reuse

`chant capture` writes an additional, fully backward-compatible metadata layer
onto every recipe card. **Available now**, emitted automatically at capture:

```yaml
spell_hash: ff9a7d644ac15c3d    # content-addressed identity of the procedure
provenance:
    origin: /abs/path/to/repo   # where the enchantment was captured
    captured_at: "2026-05-27T21:35:06Z"
    author: agent:claude        # set with --author (defaults to agent:capture)
scope: project                  # the universality ladder starts at project
portability:
    determinism: deterministic
    input_contract:
        required_columns_any:   # populated from --columns
            - [channel, amount]
    requires:
        runtime: python         # populated from --language
```

| Field | Source | Meaning |
| --- | --- | --- |
| `spell_hash` | computed at capture | content-addressed identity of the procedure, so the same method is recognizable even with a different id. |
| `provenance.origin` | repo root at capture | where the enchantment was born. |
| `provenance.captured_at` | capture time | UTC timestamp. |
| `provenance.author` | `--author` (default `agent:capture`) | who/what captured it. |
| `scope` | always `project` at capture | the universality ladder's first rung. |
| `portability.determinism` | `deterministic` | how reusable the procedure is across contexts. |
| `portability.input_contract.required_columns_any` | `--columns` | the logical columns the inputs must satisfy. |
| `portability.requires.runtime` | `--language` | the runtime the procedure needs. |

Two `chant capture` flags drive this metadata: `--author <id>` sets
`provenance.author`, and `--columns a,b` populates
`portability.input_contract.required_columns_any` (it also feeds the recipe's
column-alias signal). The canonical design — including the parts still
**planned** — is [`docs/specs/enchantment-metadata.md`](docs/specs/enchantment-metadata.md).

**Still planned:** the cross-package **registry**, `chant suggest --global` and
`chant import` for discovering foreign enchantments, **scope promotion**
(`project → domain → universal`, earned by a verifier passing in new contexts),
and **typed relations** reusing coherence's edge vocabulary (`supersedes`,
`mirrors`, `depends_on`, `implements`, …). Every metadata field is optional
(`omitempty`), so a hand-written `recipe.yaml` with no metadata stays valid.

## Command reference

```bash
# Lifecycle (the agent hook surface)
chant suggest --task "..." [--files a,b] [--columns a,b] [--json]   # find a reusable recipe before writing code
chant capture --id <id> --task "..." --command "..." [--verifier "..."] [--entrypoint-src path] [--columns a,b] [--author id] [--json]   # distill solved work into a recipe
chant run <id> [--input k=v ...] [--timeout 60s] [--json]          # execute a recipe (never sets trusted)
chant verify <id> [--input k=v ...] [--run=false] [--json]         # run + verify; only a pass is "trusted"

# Library
chant list [--json]                 # list recipes
chant search "<query>" [--json]     # rank recipes against a query
chant explain <id> [--json]         # print a recipe card
chant invalidate <id> [--reason ...] [--json]   # mark a recipe stale

# Repo
chant init [--force] [--json]       # scaffold chant.yml, recipes/, skill, gitignore
chant index [--json]                # rebuild .chant/index.json
chant status [--json]               # rewrite .chant/STATUS.md
chant doctor [--json]               # validate config + store
chant bench [--suite=retrieval|e2e|all] [--json]   # run the validation suite
chant version
chant help
```

The committed library lives under `recipes/`. Runtime state (the index, run
logs, `STATUS.md`) lives under `.chant/`, which is gitignored. One page per
command is in [`docs/commands/`](docs/README.md). Flags may appear in any
position — before or after a positional recipe id both work.

## JSON outcome contract

Every command accepts `--json` and emits a stable top-level vocabulary so
pre-commit hooks and agents can decide what to do next without parsing prose.
This mirrors coherence's outcome contract so the two harnesses feel consistent.
Source: [`internal/outcome/outcome.go`](internal/outcome/outcome.go). Unset
fields are omitted, so each subcommand's payload stays focused.

| Field | Type | Emitted by | Meaning |
| --- | --- | --- | --- |
| `subcommand` | string | all | which command produced this payload (`suggest`, `capture`, `run`, `verify`, `invalidate`, `search`). |
| `match_found` | bool | all | a candidate above the retrieval threshold exists. Always present (`false` too) so a consumer never has to distinguish "no match" from "field absent". |
| `hits` | `[]Hit` | `suggest`, `search` | ranked recipe candidates (see below). |
| `recipe_id` | string | `run`, `verify`, `capture`, `invalidate` | the recipe acted on. |
| `version` | int | `run`, `verify`, `capture` | the recipe version. |
| `executed` | bool | `run`, `verify` | the procedure was run. |
| `exit_code` | int | `run`, `verify` | the procedure / verifier exit code. Always present. |
| `verifier_ran` | bool | `verify` | the verifier command was executed. |
| `trusted` | bool | all | **the verifier-first verdict.** `true` ONLY after a passing verifier. Always present; `false` everywhere else. |
| `runtime_ms` | int | `run`, `verify` | wall-clock duration of the execution. |
| `captured` | bool | `capture` | a new recipe was written. |
| `stale` | bool | `invalidate` | the recipe was marked stale. |
| `message` | string | most | a human-readable status line (and the error text when `blocking_error` is set). |
| `recommended_next_command` | string | most | the exact command to run next (verifier-first). |
| `blocking_error` | bool | any error | `true` when the command failed under `--json` (see error contract below). |
| `suggested_commands` | `[]string` | `capture` | shell commands the result recommends — e.g. how to add a missing verifier after a capture without one. |
| `llm_calls_avoided` | int | reserved | a reuse-win estimate carried on the recipe. |

Each `Hit` in `hits[]`:

| Field | Type | Meaning |
| --- | --- | --- |
| `id` | string | recipe id. |
| `version` | int | recipe version. |
| `description` | string | recipe description. |
| `confidence` | float | the retrieval score, rounded to 2 decimals. |
| `status` | string | `active` or `stale`. |
| `verifier_exists` | bool | the recipe has a verifier, so reuse *can* be trusted. |
| `reasons` | `[]string` | why this recipe matched (lexical / file-signal / column-alias / staleness). |
| `reuse_command` | string | the exact verifier-first command to reuse this recipe. |

The recommended agent loop reads three fields and never parses prose:
`match_found` → pick `hits[0]` → run its `reuse_command` → trust iff
`trusted: true`.

### Error contract

Under `--json`, errors are part of the contract too. A command that fails prints
a JSON object to **stdout** (not prose on stderr) and exits 1:

```json
{
  "subcommand": "verify",
  "blocking_error": true,
  "message": "recipe \"nonexistent\" not found"
}
```

So a `--json` consumer always gets parseable JSON — success or failure — and can
gate on `blocking_error`. A capture that succeeds but has no verifier is **not**
an error (it exits 0); it instead reports the gap in `message` and
`suggested_commands`:

```json
{
  "subcommand": "capture",
  "match_found": false,
  "recipe_id": "noverify",
  "version": 1,
  "captured": true,
  "trusted": false,
  "message": "captured WITHOUT a verifier — reuse cannot be trusted until you add one",
  "suggested_commands": [
    "chant capture --id noverify --force --verifier \"<cmd>\" ..."
  ],
  "recommended_next_command": "chant verify noverify"
}
```

## chant.yml configuration

`chant.yml` lives at the repo root and is committed. A missing file is not an
error — chant works with zero config and built-in defaults. Source:
[`internal/config/config.go`](internal/config/config.go).

```yaml
version: 1

recipes_dir: recipes

retrieval:
  # Hybrid scorer weights. A hit above threshold is a *candidate*; only a
  # passing verifier makes a reuse result trusted.
  weight_lexical: 0.5        # task-text overlap with description/patterns
  weight_tags: 0.3           # structural file/column signal match
  weight_success_rate: 0.2   # verifier track record
  threshold: 0.25            # minimum score for 'chant suggest' to report a match
```

| Key | Default | Meaning |
| --- | --- | --- |
| `recipes_dir` | `recipes` | committed recipe library, relative to repo root. |
| `retrieval.weight_lexical` | `0.5` | weight on token overlap between the query and the recipe's description + task patterns + tags. |
| `retrieval.weight_tags` | `0.3` | weight on the structural signal match (file globs + column aliases). |
| `retrieval.weight_success_rate` | `0.2` | weight on the recipe's verifier track record. |
| `retrieval.threshold` | `0.25` | minimum blended score for `suggest` to report a match. `search` ranks everything regardless. |

The scorer is **deterministic** — no embeddings, no network — so a `suggest`
result is reproducible and testable. An optional semantic pass can be layered
later, gated like coherence's optional LLM pass.

```text
score = weight_lexical      * lexical(query, recipe text)
      + weight_tags         * signal_match(query files/columns, recipe signals)
      + weight_success_rate * verifier_success_rate
```

Stale recipes are scored at half weight so an active alternative outranks them
but they stay retrievable — a passing verifier re-blesses a stale recipe.

## Relationship to coherence

chant and coherence are the two halves of an agent-edited-repo harness:

| | coherence | chant |
| --- | --- | --- |
| Question | *What broke?* | *What worked and is reusable?* |
| Object | drift signals across the repo graph | verified, versioned recipes (enchantments) |
| Direction | catches regressions after edits | captures + reuses successful procedures |
| Trust model | verdict (`clean` / `telemetry` / `warn`) | verifier-first `trusted` gate |
| State dir | `.coherence/` (gitignored) | `.chant/` (gitignored) |
| Config | `ontology.yml` (rules) | `chant.yml` (retrieval) |
| Agent surface | `--json` outcome contract | `--json` outcome contract |

They are designed to share muscle memory: the same `--json`-first contract, the
same `init` / `doctor` / `bench` / `status` command shapes, the same
committed-config-vs-gitignored-runtime split. chant's `ontology.yml` keeps
coherence's drift rules wired so `coherence doctor` and a pre-commit
`coherence` gate keep working in a chant repo. The planned enchantment-metadata
layer deliberately reuses coherence's typed-edge vocabulary so a repo running
both tools gets one unified graph.

## Pre-commit

chant pairs with coherence rather than shipping its own hook. If you want a
chant gate too, add a `chant status` or `chant bench` line to your existing
`.githooks/pre-commit`. `chant init` prints this hint.

## Tests

```bash
go test ./...
```

## Bench

`chant bench` runs the shipped validation suites and exits 1 on any failure —
same CI muscle memory as `coherence bench`:

```bash
chant bench                      # default: all suites
chant bench --suite=retrieval    # retrieval ranking scenarios (incl. true negatives)
chant bench --suite=e2e          # run + verify every recipe with an example, asserting the trust gate
chant bench --json               # machine-readable
```

The retrieval suite asserts which recipe ranks first for a query (and that an
unrelated query yields **no** match). The e2e suite proves the verifier-first
gate end to end: each recipe with a verifier and an example is run and verified,
and is only counted as a pass when the verifier establishes trust. See
[`internal/bench/bench.go`](internal/bench/bench.go).
