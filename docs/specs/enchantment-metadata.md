# Spec: Enchantment metadata & reuse model

Status: draft (v0.1) · Owner: lead · Audience: chant developers, docs, QA

## 0. Terminology

An **enchantment** is a chant recipe: a verified, reusable procedure captured
from successful agent work. chant *casts* enchantments. "Enchantment" is the
product/narrative term; `recipe.yaml` / `recipes/` remain the on-disk form
because they read plainly to newcomers. The two words are synonyms throughout
chant — code type `recipe.Recipe`, user-facing noun "enchantment".

> Decision needed (D-1): keep `recipe` on disk + `enchantment` in prose
> (current plan), or rename the on-disk format and CLI nouns to `enchantment`.
> Recommendation: keep both as synonyms now; add `chant enchantments` as an
> alias of `chant list` later. Low risk, high brand fit.

## 1. Why metadata (the problem)

Today an enchantment knows how to solve *one task in one repo*. The valuable
scenarios need more:

1. **Cross-package discovery.** With many projects, an enchantment captured in
   project A may be exactly what project B needs. In a perfect world `chant
   suggest` in B finds A's enchantment. This requires portable identity +
   provenance + a portability contract.
2. **Universality graduation.** An enchantment starts *project-specific* and
   *becomes universal* as it proves itself across independent contexts. We need
   to record where it has been verified and promote its scope as evidence
   accumulates — a maturity channel, like a release stability tier.
3. **Relations & lineage.** Enchantments relate to data/config and to each
   other: this version supersedes that one; this Python enchantment mirrors a
   Go one; this one depends on a schema. That lineage *is* versioning,
   generalized to procedures.

## 2. Design principles

- **Additive & backward compatible.** Every field below is optional
  (`omitempty`). Existing `recipe.yaml` files stay valid; an enchantment with no
  metadata behaves exactly as today.
- **Deterministic identity.** Cross-package recognition must not depend on a
  human-chosen id. We derive a content-addressed `spell_hash`.
- **Coherent with coherence.** Relation kinds reuse coherence's typed-edge
  vocabulary (`supersedes`, `mirrors`, `depends_on`, `implements`, …) so a repo
  running both harnesses gets one unified graph, not two.
- **Verifier-first, still.** Discovery across packages widens the candidate set;
  it never widens trust. A foreign enchantment is the *most* suspect kind of hit
  and MUST pass its verifier (adapted to the local context) before use.

## 3. Metadata schema (additions to recipe.yaml)

```yaml
# ── identity (cross-package recognition) ──────────────────────────────
spell_hash: "9f2c…"      # sha256 of the normalized procedure (see §4). Stable
                          # across repos: same procedure ⇒ same hash.
lineage_id: "csv-revenue" # stable family id shared across versions/forks.

# ── provenance (where it came from) ───────────────────────────────────
provenance:
  origin: "github.com/fireharp/bitgn"   # repo/package the enchantment was born in
  captured_from: "run:20260527T2109Z"    # chant run id / git commit / agent id
  captured_at: "2026-05-27T21:09:37Z"
  author: "agent:claude-opus-4-7"

# ── scope / universality (maturity channel) ───────────────────────────
scope: project            # project | domain | universal  (see §5)
domains: [ecommerce, csv] # discovery labels broader than tags
verified_in:              # distinct contexts where the verifier passed
  - context: "fireharp/bitgn"      ; at: "2026-05-27T…"
  - context: "fireharp/tinkershop" ; at: "2026-05-28T…"
# promotion is computed from verified_in (see §5), not hand-set.

# ── portability (can it move to another package?) ─────────────────────
portability:
  determinism: deterministic   # deterministic | effectful  (effectful = has side effects)
  side_effects: []             # e.g. ["/bin/payments refund"] — empty ⇒ pure
  input_contract:              # what the inputs must look like
    schema_fingerprint: "cols:channel|revenue"
    required_columns_any: [[channel, source, utm_source], [revenue, amount, price]]
  requires:
    runtime: "python: >=3.8"
    packages: {}
    env: []                    # required env vars; empty ⇒ context-free

# ── relations (typed lineage; coherence edge vocabulary) ──────────────
relations:
  supersedes: [csv-revenue-by-channel@1]    # version lineage
  mirrors:    [csv-revenue-by-channel-go]    # same procedure, other language/package
  generalizes: []                            # this is a broader form of …
  specializes: []                            # this is a narrower form of …
  depends_on: ["data:orders-schema", "config:chant.yml"]
  implements: [US-014]                       # the story/policy it fulfils
```

All groups are optional. `chant capture` fills `spell_hash`, `provenance`,
`portability.requires`, and `input_contract` automatically; `scope`,
`domains`, and `relations` may be set by hand or inferred.

## 4. `spell_hash` — portable identity

The hash that lets project B recognize project A's enchantment as "the same
spell":

```
spell_hash = sha256(
  normalize(what_to_do.command)          # placeholders → {{·}}, whitespace-collapsed
  ⧺ canonical(entrypoint source)         # comments stripped, formatted (reuse coherence semantic_hash rules)
  ⧺ sorted(input_contract.required_columns_any)
)
```

Two enchantments with the same `spell_hash` are the same procedure even if their
ids/descriptions differ. This powers: dedup, "this also lives in project X",
and `mirrors` auto-detection. `lineage_id` groups versions/forks that drift
apart but share an ancestor.

## 5. Scope & the universality ladder (the versioning parallel)

Scope is a **maturity channel**, analogous to release stability tiers:

| scope       | meaning                                   | promotion rule (proposed)                                   |
|-------------|-------------------------------------------|-------------------------------------------------------------|
| `project`   | verified only in its origin repo          | default at capture                                          |
| `domain`    | verified across ≥2 repos in the same domain | `verified_in` has ≥2 distinct contexts sharing a `domains` tag |
| `universal` | verified across ≥3 repos in ≥2 domains    | `verified_in` ≥3 distinct contexts spanning ≥2 domains      |

Promotion is **earned, not declared** — driven by `verified_in`, which only
grows when a verifier passes in a new context (verifier-first again).
Demotion happens on repeated failures (ties into `invalidate`/stale).

`chant suggest --scope=domain+` would then prefer portable enchantments;
`chant promote <id>` (future) recomputes scope from evidence.

## 6. Cross-package discovery

- **Local registry:** a per-machine index at `~/.chant/registry/index.json`
  aggregating enchantments across the user's projects (each `chant index` also
  upserts into the registry, keyed by `spell_hash`).
- **`chant suggest --global`** searches the registry, returning foreign hits
  annotated with `provenance.origin` and `scope`. A foreign hit's `reuse_command`
  copies the enchantment locally (`chant import <origin> <id>`) then verifies.
- **Remotes (future):** a registry can point at a shared/team URL so a team
  pools enchantments. Out of scope for the MVP; the schema is designed for it.

## 7. Relationship to the coherence graph

chant relation kinds are a subset of coherence's edge vocabulary
(`supersedes`, `mirrors`, `depends_on`, `implements`, `specializes`/
`generalizes` map to `describes`/`generates`-style refinements). An
`enchantment` should surface as a first-class node kind so that, in a repo
running both tools, coherence's graph shows enchantments alongside docs, tests,
and code — and chant can ask coherence "what data/config does this depend on?"
rather than re-deriving it. Tracked as an integration item (see §9).

## 8. Backward compatibility & rollout

1. Add the optional structs to `recipe.Recipe` (new file
   `internal/recipe/enchantment.go`) — pure addition, `omitempty`.
2. `chant capture` populates `spell_hash`, `provenance`, `portability`.
3. `chant index` upserts into `~/.chant/registry`.
4. `chant suggest --global` and `chant import` land after the registry.
5. `scope` promotion + `chant promote` last.

Steps 1–2 are MVP+1; 3–5 are follow-ups. No step breaks an existing recipe.

## 9. Open decisions for the user

- **D-1** naming: `recipe` on disk + `enchantment` in prose (recommended), or
  full rename?
- **D-2** registry location/format: `~/.chant/registry/index.json` (proposed) —
  acceptable, or prefer in-repo `.chant/registry`?
- **D-3** promotion thresholds in §5 — are 2/3-context bars right, or stricter?
- **D-4** should `enchantment` become a coherence node kind now, or after the
  registry lands?
