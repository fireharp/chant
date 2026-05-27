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
		},
	}
}

// RunRetrieval executes the retrieval suite.
func RunRetrieval(cfg config.Retrieval) Summary {
	sum := Summary{Suite: "retrieval"}
	for _, sc := range RetrievalSuite() {
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
				res.Detail = fmt.Sprintf("top=%s @ %.2f", matches[0].Recipe.ID, matches[0].Score)
			}
		}
		record(&sum, res)
	}
	return sum
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
	return sum, nil
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
