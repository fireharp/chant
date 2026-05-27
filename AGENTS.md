# Repository Guidelines

> **Command reference**: see [`docs/`](docs/README.md) for a long-form page on
> every command, including flags, the JSON outcome contract, and real CLI
> output. Agents wiring chant into a hook should start there.

chant is a recipe cache for coding agents: it caches the tested *method*, not
the answer. A chant recipe is also called an **enchantment** (synonym; the code
type is `recipe.Recipe`, the on-disk form is `recipe.yaml`). chant is the
complement to coherence — coherence catches what broke (drift); chant captures
what worked and is reusable (recipes). The non-negotiable rule everywhere is the
**verifier-first trust gate**: a retrieved recipe is a *candidate*; a reuse
result is `trusted` only after its verifier passes. `retrieve → adapt → execute
→ verify → accept`, never `retrieve → trust`.

## Project Structure & Module Organization

This is a Go CLI. `cmd/chant/main.go` owns argument parsing, command dispatch,
the usage text, and the shared `--json` / store helpers; `cmd/chant/commands.go`
implements every subcommand; `cmd/chant/init.go` holds the `init` scaffolder and
its embedded `chant.yml` / skill templates. Shared behavior lives under
`internal/`:

- `config` loads `chant.yml` (via `gopkg.in/yaml.v3`) with built-in defaults.
- `recipe` defines the `Recipe` type (the on-disk `recipe.yaml` card),
  fingerprinting, slugging, metrics, and stale/active status.
- `store` is the filesystem layout: repo-root discovery, the committed recipe
  library under `recipes/`, and the gitignored runtime state under `.chant/`
  (the `index.json` and per-run logs).
- `retrieve` ranks recipes against a query (deterministic hybrid scorer: lexical
  + structural signal + verifier success rate). No embeddings, no network.
- `runner` executes a recipe's procedure and verifier and enforces the trust
  gate — `Run` never decides trust; `Verify` returns `trusted` only on a passing
  verifier.
- `outcome` computes the shared JSON outcome vocabulary (`subcommand`,
  `match_found`, `hits`, `trusted`, `verifier_ran`, `recommended_next_command`,
  …), mirroring coherence's contract.
- `glob` is the local glob matcher used for file-signal matching.
- `status` writes `.chant/STATUS.md`; `bench` runs the retrieval + e2e
  validation suites.

`recipes/` is **committed** — it is the recipe library. `.chant/` is **runtime
state** and is gitignored (`chant init` adds it to `.gitignore`). `chant.yml` is
the default config used by the CLI from the repository root; it is committed and
optional (chant works with zero config).

`ontology.yml` keeps coherence's drift rules wired for this repo so a
`coherence` pre-commit gate and `coherence doctor` keep working. Treat it as the
coherence surface, not a chant config file.

## Build, Test, and Development Commands

Use Go 1.26 or newer.

- `go test ./...` runs the full test suite.
- `go build -o bin/chant ./cmd/chant` produces the CLI binary.
- `go install ./cmd/chant` installs `chant` to `$GOBIN`.
- `./bin/chant init [--force] [--json]` scaffolds a fresh repo: writes
  `chant.yml`, `recipes/.gitkeep`, the `.agents/skills/chant/SKILL.md` skill,
  appends `.chant/` to `.gitignore`, and creates the `.chant/` state dir. It is
  idempotent — existing files are skipped without `--force`.
- `./bin/chant suggest --task "..." [--files a,b] [--columns a,b] [--json]`
  ranks the library against a task and reports candidates at or above the
  retrieval threshold. This is the **pre-write hook**.
- `./bin/chant capture --id <id> --task "..." --command "..." [--verifier "..."]
  [--expect-artifacts a,b] [--entrypoint-src path] [--columns a,b]
  [--author id] [--tags ...] [--force] [--json]` distills solved work into a
  recipe card and writes enchantment metadata (`spell_hash`, `provenance`,
  `scope`, `portability`). This is the **after-success hook**. A recipe runs
  inside `recipes/<id>/`, so any file the command/verifier references must live
  there — `--entrypoint-src <path>` copies it in at capture time.
- `./bin/chant run <id> [--input k=v ...] [--timeout 60s] [--json]` executes a
  recipe's procedure after substituting `{{var}}` placeholders. **Never sets
  `trusted`** — `run` alone is not reuse.
- `./bin/chant verify <id> [--input k=v ...] [--run=false] [--timeout 60s]
  [--json]` runs the procedure (unless `--run=false`) then the verifier, and
  records the verification as the trust event. `trusted` is `true` only on a
  passing verifier; a passing verifier also re-blesses a stale recipe.
- `./bin/chant list [--json]` lists recipes with version, run count, and success
  rate.
- `./bin/chant search "<query>" [--json]` ranks every recipe against a query
  (no threshold filtering — unlike `suggest`).
- `./bin/chant explain <id> [--json]` prints a recipe card.
- `./bin/chant invalidate <id> [--reason ...] [--json]` marks a recipe stale.
- `./bin/chant index [--json]` rebuilds `.chant/index.json` from the library.
- `./bin/chant status [--json]` rewrites `.chant/STATUS.md` (and emits the
  structured report with `--json`: `recipe_count`, `active_count`,
  `stale_count`, `total_runs`, per-recipe stats including `has_verifier`).
- `./bin/chant doctor [--json]` validates `chant.yml`, the `recipes/` dir,
  verifier coverage, and the `.chant/` gitignore entry. Exit code is `1` only
  when a check is `fail`.
- `./bin/chant bench [--suite=retrieval|e2e|all] [--json]` runs the validation
  suites. Exit `1` on any scenario failure.

Flags may appear in any position — before or after a positional recipe id both
work (`chant verify <id> --json` and `chant verify --json <id>` are equivalent).

## The agent hook workflow

chant is built to be wired into an agent's hooks, language-agnostically. The
three load-bearing moments:

1. **Suggest before writing code.** Before the agent creates a new script, run
   `chant suggest --task "<what you're about to do>" --files "<inputs>" --json`.
   Read `match_found`, `hits[0].id`, `hits[0].verifier_exists`, and
   `hits[0].reuse_command`. If a hit has a verifier, reuse it instead of writing
   new code.

2. **Verify before trusting.** Reuse is always verifier-first. Run the hit's
   `reuse_command` (`chant verify --json <id> --input ...`) and trust the result
   **only** when the JSON reports `"trusted": true`. A cache hit is a candidate;
   the verifier blesses it. If the verifier fails, repair the recipe or
   `chant invalidate` it — never ship an unverified reuse.

3. **Capture after success.** When the agent solves a recurring task and it
   passes a test/verifier, capture the procedure so the next similar task reuses
   it: `chant capture --id <slug> --task "..." --command "..." --verifier "..."
   --entrypoint-src <script> --tags "..." --json`, then `chant verify <slug>` to
   confirm the verifier passes and establish the first trust event. The recipe
   runs inside `recipes/<slug>/`, so use `--entrypoint-src` to copy any script
   the command/verifier references into the recipe dir — otherwise verify fails
   because the file isn't there.

The shipped skill at `.agents/skills/chant/SKILL.md` (written by `chant init`)
encodes this loop for Codex/Claude-style agents. Agents should consume the
`--json` outcome contract (`internal/outcome/outcome.go`) rather than parsing
human prose: `subcommand`, `match_found`, `hits[]`, `trusted`, `verifier_ran`,
`recommended_next_command`. `match_found` is always present (`false` too), so a
consumer never has to distinguish "no match" from "field absent".

**Error contract.** Under `--json`, a command that fails prints a JSON object to
**stdout** — `{"subcommand", "blocking_error": true, "message"}` — and exits 1,
rather than writing prose to stderr. A `--json` consumer therefore always gets
parseable JSON and can gate on `blocking_error`. A capture that succeeds without
a verifier is **not** an error (exit 0): it reports the gap in `message` and a
`suggested_commands` entry showing how to add one.

## Coding Style & Naming Conventions

Standard Go style: `gofmt` / `goimports`, tab indentation, lowerCamelCase
locals, PascalCase exports, package names short and lowercase. Keep CLI output
stable and concise because hooks consume it directly. The `--json` outcome
vocabulary is a **contract** — add fields (with `omitempty`) rather than
renaming or repurposing existing ones, mirroring coherence's stability promise.
Recipe-card fields use `snake_case` YAML keys; the planned enchantment-metadata
additions are all optional (`omitempty`) so existing cards stay valid.

## Testing Guidelines

Tests live next to the package they cover (`*_test.go`) and use Go's `testing`
package. Add focused cases for the retrieval scorer, glob matcher, recipe
parsing/fingerprinting, the runner's trust gate, and the outcome contract when
changing those modules. The `bench` suites double as regression guards: the
retrieval suite asserts ranking + true negatives, and the e2e suite asserts the
verifier-first gate end to end. Run `go test ./...` and `./bin/chant bench`
before handing off a change that touches `internal/`.
