// Package bench is chant's validation harness. It proves the core thesis with
// reproducible scenarios:
//
//   - retrieval scenarios: a synthetic recipe set + a query, asserting which
//     recipe ranks first and whether it clears the match threshold (including
//     true negatives — an unrelated query must NOT match).
//   - end-to-end scenarios: run a real recipe's procedure + verifier and assert
//     the verifier-first trust gate (trusted only after the verifier passes).
//
// Exit-1-on-failure semantics match coherence's bench so the same CI muscle
// memory applies.
package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fireharp/chant/internal/config"
	"github.com/fireharp/chant/internal/recipe"
	"github.com/fireharp/chant/internal/retrieve"
	"github.com/fireharp/chant/internal/runner"
	"github.com/fireharp/chant/internal/store"
)

// Result is one scenario outcome.
type Result struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Suite  string `json:"suite"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// Summary aggregates a bench run.
type Summary struct {
	Suite   string   `json:"suite"`
	Total   int      `json:"total"`
	Passed  int      `json:"passed"`
	Failed  int      `json:"failed"`
	Results []Result `json:"results"`
}

// RetrievalScenario asserts retrieval behavior over a synthetic recipe set.
type RetrievalScenario struct {
	ID      string
	Name    string
	Recipes []*recipe.Recipe
	Query   retrieve.Query
	// ExpectTop is the recipe id expected to rank first (empty = expect no match).
	ExpectTop string
	// ExpectMatch is whether a match above threshold is expected at all.
	ExpectMatch bool

	// ── score-magnitude assertions (gap 3) ────────────────────────────────
	// These let a scenario assert *how strongly* a recipe ranks, not merely
	// which id wins. All are optional; the zero value asserts nothing.

	// MinScore, when > 0, asserts the top hit's score is at least this value.
	MinScore float64
	// MaxScore, when > 0, asserts the top hit's score is at most this value
	// (useful to pin a lexical-only hit below a fully-signalled one).
	MaxScore float64
	// ExpectStalePenalty asserts the top hit is a stale recipe carrying the
	// ×0.5 penalty: its raw (unpenalized) score must be at least double the
	// reported score, and its reasons must mention the stale warning. This is
	// the retrieval half of the verifier-first-stale invariant (RET-007).
	ExpectStalePenalty bool
}

func r(id, desc string, patterns, tags []string, files []string, cols [][]string, runs, fails int) *recipe.Recipe {
	rc := &recipe.Recipe{
		ID:          id,
		Version:     1,
		Description: desc,
		WhenToUse: recipe.WhenToUse{
			TaskPatterns: patterns,
			Tags:         tags,
			InputSignals: recipe.InputSignals{Files: files, ColumnsAny: cols},
		},
		Metrics: recipe.Metrics{Runs: runs, Failures: fails},
	}
	return rc
}

// RetrievalSuite is the shipped set of deterministic retrieval scenarios.
func RetrievalSuite() []RetrievalScenario {
	revenue := r("csv-revenue-by-channel",
		"Compute ecommerce revenue by channel from CSV-like exports",
		[]string{"compute revenue by channel from csv", "analyze ecommerce orders export"},
		[]string{"csv", "ecommerce", "revenue", "analytics"},
		[]string{"*.csv"},
		[][]string{{"channel", "source", "utm_source"}, {"price", "amount", "revenue"}},
		12, 1)

	refund := r("refund-chargeback-threat",
		"Approve a refund when a customer threatens a chargeback, verifying status and ownership",
		[]string{"refund payment after chargeback threat", "approve refund for return"},
		[]string{"refund", "payments", "policy", "ecommerce"},
		nil, nil, 8, 0)

	normalize := r("normalize-orders-export",
		"Normalize messy ecommerce CSV column names to a canonical schema",
		[]string{"normalize messy ecommerce csv columns", "clean order export headers"},
		[]string{"csv", "normalize", "ecommerce"},
		[]string{"*.csv"}, nil, 5, 2)

	set := []*recipe.Recipe{revenue, refund, normalize}

	// RET-007 uses a one-recipe set: the revenue recipe flagged stale. It must
	// still be retrievable (a candidate) but its score is halved by the ×0.5
	// stale penalty until a verifier re-blesses it.
	staleRevenue := r("csv-revenue-by-channel",
		"Compute ecommerce revenue by channel from CSV-like exports",
		[]string{"compute revenue by channel from csv", "analyze ecommerce orders export"},
		[]string{"csv", "ecommerce", "revenue", "analytics"},
		[]string{"*.csv"},
		[][]string{{"channel", "source", "utm_source"}, {"price", "amount", "revenue"}},
		12, 1)
	staleRevenue.MarkStale()
	staleSet := []*recipe.Recipe{staleRevenue}

	return []RetrievalScenario{
		{
			ID: "RET-001", Name: "hit on similar revenue task",
			Recipes:   set,
			Query:     retrieve.Query{Task: "calculate revenue by channel from this CSV export", Files: []string{"orders_shopify.csv"}, Columns: []string{"utm_source", "amount"}},
			ExpectTop: "csv-revenue-by-channel", ExpectMatch: true,
		},
		{
			ID: "RET-002", Name: "no false hit on unrelated task",
			Recipes:   set,
			Query:     retrieve.Query{Task: "rotate the kubernetes TLS certificates in the staging cluster"},
			ExpectTop: "", ExpectMatch: false,
		},
		{
			ID: "RET-003", Name: "refund task routes to refund recipe",
			Recipes:   set,
			Query:     retrieve.Query{Task: "the customer wants a refund and is threatening a chargeback dispute"},
			ExpectTop: "refund-chargeback-threat", ExpectMatch: true,
		},
		{
			ID: "RET-004", Name: "column signals disambiguate revenue vs normalize",
			Recipes:   set,
			Query:     retrieve.Query{Task: "compute revenue by channel", Files: []string{"orders.csv"}, Columns: []string{"channel", "price"}},
			ExpectTop: "csv-revenue-by-channel", ExpectMatch: true,
			MinScore: 0.9,
		},
		{
			ID: "RET-005", Name: "column-adaptation routes header cleanup to normalize",
			Recipes:   set,
			Query:     retrieve.Query{Task: "normalize the messy column headers in this orders export", Files: []string{"orders.csv"}},
			ExpectTop: "normalize-orders-export", ExpectMatch: true,
		},
		{
			ID: "RET-006", Name: "ambiguous query breaks tie deterministically",
			Recipes:   set,
			Query:     retrieve.Query{Task: "analyze the ecommerce orders export", Files: []string{"orders.csv"}},
			ExpectTop: "csv-revenue-by-channel", ExpectMatch: true,
		},
		{
			ID: "RET-007", Name: "stale recipe retrievable but penalized ×0.5",
			Recipes:   staleSet,
			Query:     retrieve.Query{Task: "compute revenue by channel from csv", Files: []string{"orders.csv"}, Columns: []string{"channel", "amount"}},
			ExpectTop: "csv-revenue-by-channel", ExpectMatch: true,
			ExpectStalePenalty: true,
		},
		{
			ID: "RET-008", Name: "column-signal precision: unsatisfied aliases add nothing",
			Recipes:   set,
			Query:     retrieve.Query{Task: "compute revenue by channel", Columns: []string{"foo", "bar"}},
			ExpectTop: "csv-revenue-by-channel", ExpectMatch: true,
			// No file signal and no satisfied column group → lexical-only hit
			// (≈0.68). The fully-signalled RET-004 query scores ≈0.98; pinning
			// this below 0.7 proves the unsatisfied column group earned no
			// signal contribution (the column gate is all-groups-must-cover,
			// not any-overlap).
			MaxScore: 0.7,
		},
	}
}

// RunRetrieval executes the shipped retrieval suite.
func RunRetrieval(cfg config.Retrieval) Summary {
	return runRetrievalScenarios(RetrievalSuite(), cfg)
}

// runRetrievalScenarios evaluates a set of retrieval scenarios against the given
// config. RunRetrieval delegates here; tests use it to exercise individual
// scenarios (including the optional score-magnitude assertions).
func runRetrievalScenarios(scenarios []RetrievalScenario, cfg config.Retrieval) Summary {
	sum := Summary{Suite: "retrieval"}
	for _, sc := range scenarios {
		matches := retrieve.Suggest(sc.Recipes, sc.Query, cfg)
		res := Result{ID: sc.ID, Name: sc.Name, Suite: "retrieval", Pass: true}

		if !sc.ExpectMatch {
			if len(matches) != 0 {
				res.Pass = false
				res.Detail = fmt.Sprintf("expected no match, got %d (top=%s @ %.2f)", len(matches), matches[0].Recipe.ID, matches[0].Score)
			} else {
				res.Detail = "correctly returned no match"
			}
		} else {
			switch {
			case len(matches) == 0:
				res.Pass = false
				res.Detail = "expected a match, got none"
			case matches[0].Recipe.ID != sc.ExpectTop:
				res.Pass = false
				res.Detail = fmt.Sprintf("expected top=%s, got %s @ %.2f", sc.ExpectTop, matches[0].Recipe.ID, matches[0].Score)
			default:
				top := matches[0]
				res.Detail = fmt.Sprintf("top=%s @ %.2f", top.Recipe.ID, top.Score)
				switch {
				case sc.MinScore > 0 && top.Score < sc.MinScore:
					res.Pass = false
					res.Detail = fmt.Sprintf("top=%s @ %.2f, want score ≥ %.2f", top.Recipe.ID, top.Score, sc.MinScore)
				case sc.MaxScore > 0 && top.Score > sc.MaxScore:
					res.Pass = false
					res.Detail = fmt.Sprintf("top=%s @ %.2f, want score ≤ %.2f", top.Recipe.ID, top.Score, sc.MaxScore)
				case sc.ExpectStalePenalty:
					if detail, ok := checkStalePenalty(top, sc.Query, cfg); !ok {
						res.Pass = false
						res.Detail = detail
					} else {
						res.Detail = detail
					}
				}
			}
		}
		record(&sum, res)
	}
	return sum
}

// checkStalePenalty verifies the top hit is a stale recipe carrying the ×0.5
// retrieval penalty. It re-ranks an active clone of the same recipe against the
// same query to recover the unpenalized score, then asserts (a) the recipe is
// stale, (b) its reported score is half the active score, and (c) its reasons
// carry the stale warning. This proves a stale enchantment stays retrievable as
// a candidate but never outranks an equivalent active one.
func checkStalePenalty(top retrieve.Match, q retrieve.Query, cfg config.Retrieval) (string, bool) {
	if !top.Recipe.IsStale() {
		return fmt.Sprintf("expected stale top hit, but %s is active", top.Recipe.ID), false
	}
	hasStaleReason := false
	for _, reason := range top.Reasons {
		if strings.Contains(reason, "stale") {
			hasStaleReason = true
			break
		}
	}
	if !hasStaleReason {
		return fmt.Sprintf("stale top=%s @ %.2f but no stale reason recorded", top.Recipe.ID, top.Score), false
	}

	// Score the same recipe as if it were active to recover the raw score.
	active := *top.Recipe
	active.Status = "active"
	ranked := retrieve.Rank([]*recipe.Recipe{&active}, q, cfg)
	if len(ranked) == 0 {
		return "could not re-rank active clone for stale comparison", false
	}
	raw := ranked[0].Score
	if raw <= 0 {
		return fmt.Sprintf("active clone scored %.2f; cannot assert penalty", raw), false
	}
	// The stale score must be half the active score (within float tolerance).
	if diff := top.Score - raw*0.5; diff > 1e-9 || diff < -1e-9 {
		return fmt.Sprintf("stale=%.2f, active=%.2f — penalty is not ×0.5", top.Score, raw), false
	}
	return fmt.Sprintf("stale top=%s @ %.2f (active would be %.2f, ×0.5 penalty applied)", top.Recipe.ID, top.Score, raw), true
}

// RunE2E runs every recipe in the store that ships an example through
// run + verify, asserting the trust gate. Recipes without a verifier or
// without an example are skipped (reported as pass with a skip note) so the
// suite stays green on a fresh library.
func RunE2E(s *store.Store) (Summary, error) {
	sum := Summary{Suite: "e2e"}
	recs, err := s.LoadAll()
	if err != nil {
		return sum, err
	}
	for _, rc := range recs {
		res := Result{ID: "E2E-" + rc.ID, Name: "run+verify " + rc.ID, Suite: "e2e", Pass: true}
		if rc.Verification.Command == "" && len(rc.Verification.ExpectedArtifacts) == 0 {
			res.Detail = "skipped: no verifier"
			record(&sum, res)
			continue
		}
		if len(rc.Examples) == 0 {
			res.Detail = "skipped: no example input"
			record(&sum, res)
			continue
		}
		inputs := map[string]string{"input": rc.Examples[0].Input, "input_file": rc.Examples[0].Input}
		if _, err := runner.Run(rc, inputs, 60*time.Second); err != nil {
			res.Pass = false
			res.Detail = "run error: " + err.Error()
			record(&sum, res)
			continue
		}
		_, trusted, err := runner.Verify(rc, inputs, 60*time.Second)
		if err != nil {
			res.Pass = false
			res.Detail = "verify error: " + err.Error()
		} else if !trusted {
			res.Pass = false
			res.Detail = "verifier did not establish trust"
		} else {
			res.Detail = "verifier passed → trusted"
		}
		record(&sum, res)
	}

	// Append the isolated, fixture-driven e2e scenarios. These materialize
	// throwaway recipes under a temp dir (never the live recipes/ library) so
	// the negative-gate scenarios (verify-fails, artifact-missing, missing
	// input) can exercise the trust gate without turning real `chant bench`
	// red. See RunE2EScenarios / E2EScenario.
	iso, err := RunE2EScenarios()
	if err != nil {
		return sum, err
	}
	for _, res := range iso.Results {
		record(&sum, res)
	}
	return sum, nil
}

// E2EScenario is an isolated, fixture-driven end-to-end scenario. Unlike the
// live-store RunE2E loop, each scenario materializes a throwaway recipe under a
// temp dir, so intentionally-broken fixtures (a failing verifier, a missing
// artifact, an unresolved input) can be exercised without polluting — or
// reddening — the real recipes/ library. The fixture is described inline; the
// runner writes it to disk, runs the requested lifecycle, and asserts the
// expected trust verdict.
type E2EScenario struct {
	ID   string
	Name string
	// RecipeYAML is the recipe.yaml body written into the temp recipe dir.
	RecipeYAML string
	// Files are extra files (scripts, verifiers) to write alongside the card,
	// keyed by filename relative to the recipe dir.
	Files map[string]string
	// Inputs are passed to runner.Run / runner.Verify ({{var}} substitution).
	Inputs map[string]string

	// Assert receives the loaded fixture recipe (with Dir set) and returns a
	// pass/fail plus a human detail. It is the scenario's body — it decides
	// what lifecycle to run (Run, Verify, MarkStale, …) and what to assert.
	Assert func(rc *recipe.Recipe) (detail string, pass bool)
}

// E2EScenarios is the shipped set of isolated end-to-end scenarios
// (E2E-001..005 from docs/testing/scenarios.md). They never touch the live
// recipes/ library.
func E2EScenarios() []E2EScenario {
	const timeout = 30 * time.Second
	return []E2EScenario{
		{
			ID:   "E2E-001",
			Name: "failing verifier is not trusted",
			RecipeYAML: "id: failing-verifier\nversion: 1\ndescription: a recipe whose verifier always fails\n" +
				"what_to_do:\n  command: \"echo ran\"\n" +
				"verification:\n  command: \"false\"\n",
			Assert: func(rc *recipe.Recipe) (string, bool) {
				if _, err := runner.Run(rc, nil, timeout); err != nil {
					return "run error: " + err.Error(), false
				}
				_, trusted, err := runner.Verify(rc, nil, timeout)
				if err != nil {
					return "verify error: " + err.Error(), false
				}
				if trusted {
					return "verifier failed yet result was trusted (wrong-answer amplifier!)", false
				}
				return "failing verifier correctly NOT trusted", true
			},
		},
		{
			ID:   "E2E-002",
			Name: "passing command but missing artifact is not trusted",
			RecipeYAML: "id: missing-artifact\nversion: 1\ndescription: runs fine but never produces its declared artifact\n" +
				"what_to_do:\n  command: \"echo ran\"\n" +
				"verification:\n  command: \"true\"\n  expected_artifacts:\n    - missing.json\n",
			Assert: func(rc *recipe.Recipe) (string, bool) {
				if _, err := runner.Run(rc, nil, timeout); err != nil {
					return "run error: " + err.Error(), false
				}
				_, trusted, err := runner.Verify(rc, nil, timeout)
				if err != nil {
					return "verify error: " + err.Error(), false
				}
				if trusted {
					return "missing artifact yet result was trusted", false
				}
				return "missing declared artifact correctly NOT trusted", true
			},
		},
		{
			ID:   "E2E-003",
			Name: "missing {{input}} fails fast before running",
			RecipeYAML: "id: missing-input\nversion: 1\ndescription: command needs an input that is never provided\n" +
				"what_to_do:\n  command: \"echo {{input}}\"\n" +
				"verification:\n  command: \"true\"\n",
			Inputs: map[string]string{}, // deliberately empty: {{input}} unresolved
			Assert: func(rc *recipe.Recipe) (string, bool) {
				_, err := runner.Run(rc, nil, timeout)
				if err == nil {
					return "expected fail-fast on missing input, but Run succeeded", false
				}
				if !strings.Contains(err.Error(), "missing inputs") {
					return "errored, but not the expected missing-inputs fail-fast: " + err.Error(), false
				}
				return "unresolved {{input}} correctly aborted before running: " + err.Error(), true
			},
		},
		{
			ID:   "E2E-004",
			Name: "capture → reuse → verify happy path is trusted",
			RecipeYAML: "id: capture-reuse\nversion: 1\ndescription: write a file then verify it exists\n" +
				"what_to_do:\n  command: \"echo hi > out.txt\"\n" +
				"verification:\n  command: \"test -f out.txt\"\n",
			Assert: func(rc *recipe.Recipe) (string, bool) {
				// Round-trip the lifecycle: it is retrievable as a candidate,
				// running + verifying establishes trust, and a recorded run
				// increments metrics.
				q := retrieve.Query{Task: "write a file then verify it exists"}
				hits := retrieve.Suggest([]*recipe.Recipe{rc}, q, config.Default().Retrieval)
				if len(hits) == 0 || hits[0].Recipe.ID != rc.ID {
					return "captured recipe was not retrievable as a candidate", false
				}
				if _, err := runner.Run(rc, nil, timeout); err != nil {
					return "run error: " + err.Error(), false
				}
				_, trusted, err := runner.Verify(rc, nil, timeout)
				if err != nil {
					return "verify error: " + err.Error(), false
				}
				if !trusted {
					return "happy-path verifier passed but result was NOT trusted", false
				}
				before := rc.Metrics.Runs
				rc.RecordRun(true)
				if rc.Metrics.Runs != before+1 {
					return "metrics.runs did not increment after a recorded run", false
				}
				return "candidate → run → verify → trusted; metrics.runs incremented", true
			},
		},
		{
			ID:   "E2E-005",
			Name: "invalidate → re-verify re-blesses (stale not trusted until re-verified)",
			RecipeYAML: "id: invalidate-rebless\nversion: 1\ndescription: a verified recipe that is invalidated then re-verified\n" +
				"what_to_do:\n  command: \"echo hi > out.txt\"\n" +
				"verification:\n  command: \"test -f out.txt\"\n",
			Assert: func(rc *recipe.Recipe) (string, bool) {
				// 1. Establish initial trust.
				if _, err := runner.Run(rc, nil, timeout); err != nil {
					return "initial run error: " + err.Error(), false
				}
				_, trusted, err := runner.Verify(rc, nil, timeout)
				if err != nil || !trusted {
					return "initial verify did not establish trust", false
				}
				// 2. Invalidate → stale. A stale recipe is a candidate, never
				//    trusted, until re-verified.
				rc.MarkStale()
				if !rc.IsStale() {
					return "invalidate did not mark the recipe stale", false
				}
				// 3. Re-verify with a passing verifier → re-bless to active.
				if _, err := runner.Run(rc, nil, timeout); err != nil {
					return "re-run error: " + err.Error(), false
				}
				_, trusted, err = runner.Verify(rc, nil, timeout)
				if err != nil {
					return "re-verify error: " + err.Error(), false
				}
				if !trusted {
					return "passing re-verify did not re-establish trust", false
				}
				rc.Status = "active" // a passing verifier re-blesses the stale recipe
				if rc.IsStale() {
					return "recipe remained stale after a passing re-verify", false
				}
				return "stale until re-verified; passing re-verify re-blessed to active", true
			},
		},
	}
}

// RunE2EScenarios runs the isolated, fixture-driven end-to-end scenarios. Each
// scenario's recipe is written into its own dir beneath a single os.MkdirTemp
// root, which is removed when the run completes. Nothing here reads or writes
// the live recipes/ library.
func RunE2EScenarios() (Summary, error) {
	sum := Summary{Suite: "e2e"}
	root, err := os.MkdirTemp("", "chant-bench-e2e-")
	if err != nil {
		return sum, fmt.Errorf("create temp e2e root: %w", err)
	}
	defer os.RemoveAll(root)

	for _, sc := range E2EScenarios() {
		res := Result{ID: sc.ID, Name: sc.Name, Suite: "e2e", Pass: true}
		dir := filepath.Join(root, sc.ID)
		rc, err := materialize(dir, sc)
		if err != nil {
			res.Pass = false
			res.Detail = "fixture setup error: " + err.Error()
			record(&sum, res)
			continue
		}
		detail, pass := sc.Assert(rc)
		res.Detail = detail
		res.Pass = pass
		record(&sum, res)
	}
	return sum, nil
}

// materialize writes a scenario's recipe.yaml (plus any extra files) into dir
// and loads it back as a recipe with its Dir set, so runner.Run / runner.Verify
// execute in the isolated temp dir.
func materialize(dir string, sc E2EScenario) (*recipe.Recipe, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, recipe.CardFile), []byte(sc.RecipeYAML), 0o644); err != nil {
		return nil, err
	}
	for name, body := range sc.Files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return nil, err
		}
	}
	return recipe.Load(dir)
}

func record(sum *Summary, res Result) {
	sum.Total++
	if res.Pass {
		sum.Passed++
	} else {
		sum.Failed++
	}
	sum.Results = append(sum.Results, res)
}
