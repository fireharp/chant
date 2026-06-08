# Hegel integration

Hegel is chant's opt-in generative verifier layer. It does not add a new trust
path: `chant verify` still trusts a recipe only when `verification.command`
exits 0 and every expected artifact exists.

## How it fits

Use Hegel when a recipe or internal invariant depends on input-shape variation:
schema aliases, edge-case values, stale status, artifact combinations, or
portable identity. Hegel supplies generated cases and shrinking; chant supplies
the recipe, command execution, and verifier-first trust gate.

For a recipe, keep using the existing verifier field:

```yaml
verification:
  command: go test ./property -run TestCSVRevenueProperty -count=1
  expected_artifacts:
    - property-report.json
```

The verifier command can run any Hegel client. In this Go repo, use
`hegel.dev/go/hegel` and ordinary `go test`.

## Built-in property suite

chant ships an opt-in Hegel suite:

```bash
chant bench --suite=properties
chant bench --suite=properties --json
```

The default `chant bench` / `chant bench --suite=all` path intentionally stays
fast and does not run Hegel. The property suite runs build-tagged tests with:

```bash
go test -tags hegel ./internal/bench -run TestHegelProperties -count=1
```

The suite covers retrieval stale penalties and structural signals, runner trust
gates, `spell_hash` stability, and the CSV recipe against a generated-input
oracle.

## Runtime notes

Hegel is currently beta, so chant pins the Go client version. Hegel starts a
`hegel-core` server at runtime; by default the client uses Python/`uv` to fetch
and run the compatible server. In hermetic CI, preinstall `hegel-core` and set:

```bash
export HEGEL_SERVER_COMMAND=/path/to/hegel
```

chant sets `CHANT_HEGEL_DIR` for the built-in suite so Hegel state lives under
`.chant/hegel/`. `.hegel/` is also gitignored as a fallback for Hegel server
logs.

References:

- [How Hegel works](https://hegel.dev/explanation/how-hegel-works)
- [Installation reference](https://hegel.dev/reference/installation)
- [Compatibility](https://hegel.dev/compatibility)
- [hegel-go API](https://pkg.go.dev/hegel.dev/go/hegel)
