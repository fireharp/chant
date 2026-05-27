Your idea is real, but it is split across several names. The clean framing is:

**“Don’t cache the answer. Cache the reusable procedure that produced the answer, plus when it is allowed to be reused.”**

A good product name for the concept: **procedural cache**, **recipe cache**, **method cache**, or **function-level agent cache**.

## 1. Research map: where this already exists

| Area                                          | What it already does                                                                                                                                                                                                                                                                                   | Relevance to your idea                                                                       |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------- |
| **Incremental / self-adjusting computation**  | Records dependencies and execution traces so changed inputs only recompute affected parts. CEAL describes traces that record program-code components and runtime environments, then re-executes only affected closures after modifications.                                                            | Closest classic CS ancestor. It caches the computation graph, not just final data.           |
| **Adapton / Salsa-style computation systems** | Salsa defines computations as pure query functions, memoizes query results, and intelligently reuses them after inputs change. ([salsa-rs.github.io][1]) Adapton is a Rust incremental-computation library. ([GitHub][2])                                                                              | Good implementation inspiration, but too compiler-ish for a small funky MVP.                 |
| **Partial evaluation / specialization**       | Specializes a program using stable parts of the input, producing a faster residual/specialized program. A recent overview explicitly links incremental computation and partial evaluation. ([arXiv][3])                                                                                                | This is “cache the function specialized to a situation.” Useful vocabulary.                  |
| **Agentic plan caching**                      | Stores structured plan templates from completed agent executions and adapts them for similar future tasks; one paper reports average cost reduction while maintaining most task accuracy. ([arXiv][4]) AgentReuse similarly reuses LLM-generated plans for semantically similar requests. ([arXiv][5]) | Very close to your agentic angle, but often caches plans, not executable verified functions. |
| **Agent skill libraries**                     | Voyager stores and retrieves executable code skills, with environment feedback and self-verification. ([GitHub][6]) A 2026 SoK paper defines agentic skills as reusable procedural capabilities with applicability conditions, execution policies, and interfaces. ([arXiv][7])                        | This is the strongest modern cousin: reusable “skills” as code.                              |
| **Method-based reasoning**                    | Proposes reusable methods extracted from generated responses/user interactions, stored externally and retrieved for new queries. ([arXiv][8])                                                                                                                                                          | Close conceptually, but sounds more like a reasoning framework than a dev library.           |
| **LLM semantic/prompt caching**               | GPTCache, LangChain cache, DSPy cache, OpenAI prompt caching cache prompts/responses/tokens for cost/latency. ([GitHub][9])                                                                                                                                                                            | Important contrast: these cache model IO, not reusable executable calculation logic.         |

## 2. Existing repos worth inspecting

| Repo / project                 | Why it matters                                                                                                                                                                                 |
| ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **salsa-rs/salsa**             | Mature-ish Rust framework for on-demand incrementalized computation; GitHub describes queries as pure functions whose memoized results are reused after inputs change. ([GitHub][10])          |
| **Adapton/adapton.rust**       | General-purpose Rust incremental-computation library, explicitly tied to Adapton papers. ([GitHub][2])                                                                                         |
| **ocurrent/current_incr**      | Small OCaml self-adjusting computation library, based on Adaptive Functional Programming, with no external dependencies. ([GitHub][11])                                                        |
| **amakelov/mandala**           | Very relevant Python project: persistent memoization, computation versioning, dependency tracking, and treating ordinary Python computation as a storage/query interface. ([Alex Makelov][12]) |
| **MineDojo/Voyager**           | Stores executable skills and retrieves them for new tasks; very relevant if the product story is “agents learn reusable code.” ([GitHub][6])                                                   |
| **orra-dev/orra plan caching** | Practical agent plan-cache implementation: similar action → reused plan with parameter substitution. ([GitHub][13])                                                                            |
| **GPTCache**                   | Baseline/contrast only: semantic response cache, not your core idea. ([GitHub][9])                                                                                                             |

## 3. What is actually novel here?

Not novel:

**“Cache computations / traces / dependency graphs.”** That is incremental computation.

**“Cache LLM responses.”** That is semantic/prompt caching.

**“Cache agent plans.”** That exists already.

Potentially novel / productable:

**A tiny agent-facing library that turns successful agent work into versioned, executable, validated function recipes.**

The key difference:

```text
Input task/context
  ↓
Agent solves it once
  ↓
Library distills the successful trajectory into:
  - reusable function/code
  - applicability conditions
  - examples
  - tests/verifiers
  - dependency/schema fingerprints
  - invalidation rules
  ↓
Next similar task:
  retrieve recipe → adapt parameters → run verifier → reuse or repair
```

That is more interesting than plain plan caching because the cached object is not “a previous answer” and not merely “a natural language plan.” It is a **verified executable method**.

## 4. MVP: keep it small

Build it as a Python library + CLI.

Working name: **`howcache`** or **`recipecache`**.

### Core API

```python
from howcache import RecipeCache

cache = RecipeCache(".howcache")

result = cache.solve(
    task="Given a CSV of orders, compute revenue by channel and plot it.",
    context={
        "files": ["orders.csv"],
        "schema_hint": "unknown ecommerce export",
    },
    verifier="pytest tests/test_revenue_report.py",
)
```

On cache miss:

1. Ask the agent/LLM to solve.
2. Save the produced code as a recipe.
3. Run tests/verifier.
4. Store the verified recipe.

On cache hit:

1. Retrieve similar recipe by embedding + structured tags.
2. Adapt parameters/file names/schema mapping.
3. Execute the cached function.
4. Run verifier.
5. If verifier fails, mark stale and ask the agent to repair.

### Stored object

```yaml
id: revenue_by_channel_v3
description: "Compute ecommerce revenue by channel from CSV-like exports"
applicability:
  input_type: csv
  required_columns_any:
    - ["channel", "source", "utm_source"]
    - ["price", "amount", "revenue"]
dependencies:
  python: ">=3.11"
  packages:
    pandas: ">=2"
fingerprints:
  recipe_code_hash: "..."
  verifier_hash: "..."
  schema_fingerprint: "..."
entrypoint: recipes/revenue_by_channel_v3.py:run
examples:
  - input: examples/orders_shopify.csv
    output: examples/revenue_by_channel.json
metrics:
  last_success_at: "..."
  runs: 12
  failures: 1
```

### Tiny decorator version

```python
@cache.recipe(
    description="Normalize messy ecommerce CSV columns",
    verifier="tests/test_normalize_orders.py",
)
def normalize_orders(df):
    ...
```

That gets you the “simple library” feel while still leaving room for agent-generated recipes.

## 5. Best demo applications

The demo matters more than the library.

### Demo 1 — “AI data analyst that learns functions”

Give it three messy CSV exports:

```text
orders_shopify.csv
orders_stripe.csv
orders_custom.csv
```

Ask:

```text
Compute revenue by channel.
```

First run: agent writes a function, tests it, stores recipe.

Second run on a different CSV: no full reasoning. It retrieves the recipe, maps columns, runs, verifies, and shows:

```text
Recipe hit: revenue_by_channel_v3
LLM calls avoided: 7
Runtime: 1.2s instead of 28s
Verifier: passed
```

This is probably the cleanest MVP because it is visual, understandable, and useful.

### Demo 2 — “Agent learns repo-fix recipes”

Example:

```text
Fix failing mypy errors after Pydantic v2 migration.
```

The first time, the agent solves it manually. The system distills a recipe:

```text
If error contains `BaseModel.Config` / `orm_mode`,
replace with `model_config = ConfigDict(from_attributes=True)`.
```

Next similar repo: retrieve, patch, run tests. This is good for agentic coding, but harder to make deterministic.

### Demo 3 — “Playwright workflow recipe cache”

Task:

```text
Go to admin UI, find the user, update subscription status.
```

Cache:

```text
selectors + navigation sequence + assertions + DOM fingerprints
```

Invalidate if DOM fingerprint changes. This is a strong agent demo, but more fragile and security-sensitive.

### Demo 4 — “Benchmark puzzle solver”

Use small transformation puzzles. First solve creates a reusable method like:

```text
Detect colored region → compute bounding box → mirror horizontally.
```

Next structurally similar puzzle uses the cached method. Very visual, very presentable.

## 6. Minimum architecture

```text
howcache/
  store.py          # SQLite metadata + local files
  index.py          # embeddings / lexical tags
  recipe.py         # recipe object + entrypoint
  verifier.py       # pytest / custom assertion runner
  distill.py        # agent trace/code → reusable recipe
  invalidate.py     # hashes, deps, schema, verifier changes
  cli.py            # inspect, run, list, explain
```

Storage:

```text
.howcache/
  recipes/
    revenue_by_channel_v3.py
  metadata.sqlite
  embeddings.jsonl
  examples/
  runs/
    2026-05-25T.../
      input.json
      output.json
      logs.txt
```

Use **SQLite first**. Add vector search later. For MVP, hybrid retrieval can be stupid but effective:

```text
score = 0.5 * embedding_similarity
      + 0.3 * structured_tag_match
      + 0.2 * verifier_success_rate
```

## 7. The hard part

The hard part is not caching. The hard part is **safe reuse**.

You need:

```text
recipe hit ≠ recipe trusted
```

A cache hit only means:

```text
This method might apply.
```

Then the verifier decides:

```text
This method actually worked here.
```

So the product should be verifier-first:

```text
retrieve → adapt → execute → verify → accept
```

Not:

```text
retrieve → trust
```

That is the main difference between a useful system and a wrong-answer amplifier.

## 8. Positioning

I would not position it as “new caching.” Too broad and old.

Better:

> **A tiny library for turning successful agent work into reusable, versioned, verified functions.**

Or:

> **Memoization for agentic procedures, not answers.**

Or more dev-facing:

> **A recipe cache for coding agents: cache the tested method, not the LLM response.**

## 9. Blunt verdict

This is worth exploring, but only if the MVP is narrow.

Do **not** try to build a general incremental computation engine. Salsa, Adapton, Mandala, and compiler/query systems already cover that territory.

Do **not** build another semantic cache. That space is crowded.

The good wedge is:

```text
agent solves task once
→ extract reusable executable recipe
→ store with applicability + tests + invalidation
→ reuse on similar tasks
→ show saved reasoning/cost/time visually
```

The smallest strong demo is probably **messy CSV/data-analysis recipes** or **repo-fix recipes**. Data-analysis is easier to present; repo-fix is more aligned with agentic coding.

[1]: https://salsa-rs.github.io/salsa/overview.html "Overview - Salsa"
[2]: https://github.com/Adapton/adapton.rust "GitHub - Adapton/adapton.rust: General-purpose abstractions for incremental computing, in Rust · GitHub"
[3]: https://arxiv.org/html/2312.07946v1 "Incremental Computation: What Is the Essence?"
[4]: https://arxiv.org/html/2506.14852v1 "Cost-Efficient Serving of LLM Agents via Test-Time Plan Caching"
[5]: https://arxiv.org/html/2512.21309v2 "A Plan Reuse Mechanism for LLM-Driven Agent"
[6]: https://github.com/MineDojo/Voyager?utm_source=chatgpt.com "MineDojo/Voyager: An Open-Ended Embodied Agent with ..."
[7]: https://arxiv.org/html/2602.20867v1 "SoK: Agentic Skills — Beyond Tool Use in LLM Agents"
[8]: https://arxiv.org/html/2508.04289 "Method-Based Reasoning for Large Language Models: Extraction, Reuse, and Continuous Improvement"
[9]: https://github.com/zilliztech/gptcache?utm_source=chatgpt.com "zilliztech/GPTCache: Semantic cache for LLMs. Fully ..."
[10]: https://github.com/salsa-rs/salsa "GitHub - salsa-rs/salsa: A generic framework for on-demand, incrementalized computation. Inspired by adapton, glimmer, and rustc's query system. · GitHub"
[11]: https://github.com/ocurrent/current_incr "GitHub - ocurrent/current_incr: Self-adjusting computations · GitHub"
[12]: https://amakelov.github.io/blog/pl/ "Mandala: Python programs that save, query and version themselves | Alex Makelov"
[13]: https://github.com/orra-dev/orra/blob/main/docs/plan-caching.md "orra/docs/plan-caching.md at main · orra-dev/orra · GitHub"

===

Yes — this split is the right one:

```md
with hooks?
- agent is working
- before writing scripts -> hook checks existing recipes/functions
- if agent did something useful -> hook writes it down

with runtime?
- probably language-specific
- compiled vs dynamic languages differ a lot
```

I’d shape it like this:

## 1. Do **hooks first**, not runtime

Runtime-level caching is hard and quickly becomes language/compiler/framework-specific.

Hooks are much cleaner for MVP:

```text
agent wants to write code
  ↓
pre-write hook checks recipe cache
  ↓
if similar recipe exists:
    suggest/reuse/adapt it
else:
    let agent continue
  ↓
post-success hook detects useful solved pattern
  ↓
writes recipe card + code + verifier
```

That means the first version does **not** need to understand Python/Rust/JS internals deeply. It only needs to watch agent behavior.

## 2. The object is not “cached result”, but “cached way”

The cache entry should look like:

```yaml
id: csv-revenue-by-channel
kind: executable_recipe

when_to_use:
  task_patterns:
    - "calculate revenue by channel from CSV"
    - "analyze ecommerce orders export"
  input_signals:
    files:
      - "*.csv"
    columns_any:
      - ["channel", "source", "utm_source"]
      - ["price", "amount", "revenue"]

what_to_do:
  entrypoint: recipes/csv_revenue_by_channel.py
  command: "python recipes/csv_revenue_by_channel.py {{input_file}}"

verification:
  command: "pytest recipes/test_csv_revenue_by_channel.py"
  expected_artifacts:
    - "revenue_by_channel.json"

invalidation:
  if_columns_missing: true
  if_tests_fail: true
  if_dependency_changed: true
```

This is basically **agent memory as executable recipes**.

## 3. Hook lifecycle

I would define four hooks:

```text
before_plan
before_write
after_success
after_failure
```

### `before_plan`

Checks whether the task resembles an existing recipe.

Example:

```text
User asks: "analyze this payments export and find failed chargebacks"

Hook says:
Possible recipe match:
- payments_chargeback_analysis_v2
- confidence: 0.78
- verifier exists: yes
```

### `before_write`

Runs before the agent creates a new script/file.

This is the strongest hook.

```text
Agent wants to create: analyze_orders.py

Hook:
"Before creating new script, check existing recipes:
- csv_revenue_by_channel.py
- normalize_payment_export.py
- stripe_failed_payment_summary.py"
```

This prevents agents from rewriting the same code forever.

### `after_success`

If the agent solved something and tests passed, write it down.

```text
Detected successful reusable workflow:
- created script
- ran script
- fixed errors
- produced expected output
- user accepted result

Create recipe? yes / auto if confidence high
```

### `after_failure`

Marks recipe as stale or too narrow.

```text
Recipe csv_revenue_by_channel_v1 failed on Shopify export.
Reason: missing `channel`; found `utm_source`.

Action:
- repair recipe
- add schema alias
- bump version to v2
```

## 4. Runtime version is later

Runtime is a separate product tier.

### Dynamic languages

Python/JS can be wrapped more easily:

```python
@recipe_cache.track()
def normalize_orders(df):
    ...
```

You can inspect:

```text
input schema
function source hash
dependency versions
output shape
exceptions
test result
```

### Compiled languages

Rust/Go/Java are harder. You probably cache at build/task level:

```text
cargo test failing pattern
compiler error pattern
code migration recipe
known patch strategy
```

Less “function runtime”, more “repo operation recipe”.

So yes: **runtime should be language-specific**, but **hook-level cache can be language-agnostic**.

## 5. MVP should be hook-based

Minimal MVP:

```text
.howcache/
  recipes/
    csv_revenue_by_channel/
      recipe.yaml
      run.py
      test_recipe.py
      examples/
  runs/
  index.sqlite
```

CLI:

```bash
howcache search "calculate revenue by channel from csv"
howcache run csv_revenue_by_channel --input orders.csv
howcache record --from-last-agent-run
howcache explain csv_revenue_by_channel
```

Agent integration:

```bash
# before writing a new script
howcache suggest --task "$TASK" --files "$FILES"

# after successful task
howcache capture --task "$TASK" --changed-files "$CHANGED_FILES" --tests "$TESTS"
```

## 6. Best architecture diagram

```text
                 ┌─────────────────────┐
                 │ User / Agent task    │
                 └──────────┬──────────┘
                            │
                     before_plan hook
                            │
                            ▼
                 ┌─────────────────────┐
                 │ Recipe retrieval     │
                 │ semantic + metadata  │
                 └──────────┬──────────┘
                            │
              ┌─────────────┴─────────────┐
              │                           │
        recipe hit                   recipe miss
              │                           │
              ▼                           ▼
   adapt + run recipe              agent writes code
              │                           │
              ▼                           ▼
        run verifier              run tests/verifier
              │                           │
              └─────────────┬─────────────┘
                            ▼
                  after_success hook
                            │
                            ▼
              ┌──────────────────────────┐
              │ store/update recipe       │
              │ code + tests + metadata   │
              └──────────────────────────┘
```

## 7. The clean product thesis

Use this in `LOCAL.md`:

```md
# howcache

howcache caches reusable procedures produced by agents.

It does not cache final answers.
It caches the tested way of solving a recurring task.

The first version works through agent hooks:

- before an agent writes a new script, check whether an existing recipe can solve the task
- after an agent successfully solves a task, capture the useful procedure as a recipe
- before reusing a recipe, adapt it to the current inputs
- after running a recipe, verify the result
- if verification fails, repair or invalidate the recipe

The cache entry contains:

- task description
- applicability conditions
- executable code
- examples
- verifier/tests
- dependency and schema fingerprints
- success/failure history

Runtime-level tracking may come later and will likely be language-specific.
The MVP should stay hook-based and language-agnostic.
```

Bluntly: **don’t start with runtime.** Start with agent hooks. The product becomes much easier to demo, and the idea stays distinct: not memoization, not semantic cache, not full incremental computing — **a verified recipe cache for agents**.

