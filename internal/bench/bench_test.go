package bench

import (
	"strings"
	"testing"

	"github.com/fireharp/chant/internal/config"
)

// TestRunRetrieval_AllPass asserts the shipped retrieval suite is green under
// the default weights/threshold and that every catalogued scenario id is
// present (RET-001..008).
func TestRunRetrieval_AllPass(t *testing.T) {
	sum := RunRetrieval(config.Default().Retrieval)
	if sum.Failed != 0 {
		for _, r := range sum.Results {
			if !r.Pass {
				t.Errorf("scenario %s failed: %s", r.ID, r.Detail)
			}
		}
	}
	if sum.Total != 8 {
		t.Errorf("retrieval suite total = %d, want 8", sum.Total)
	}
	want := []string{"RET-001", "RET-002", "RET-003", "RET-004", "RET-005", "RET-006", "RET-007", "RET-008"}
	got := map[string]Result{}
	for _, r := range sum.Results {
		got[r.ID] = r
	}
	for _, id := range want {
		if _, ok := got[id]; !ok {
			t.Errorf("missing retrieval scenario %s", id)
		}
	}
}

// TestRetrieval_StalePenalty checks RET-007 specifically: the stale hit's
// detail reports the ×0.5 penalty and references the active comparison.
func TestRetrieval_StalePenalty(t *testing.T) {
	sum := RunRetrieval(config.Default().Retrieval)
	var ret007 *Result
	for i := range sum.Results {
		if sum.Results[i].ID == "RET-007" {
			ret007 = &sum.Results[i]
		}
	}
	if ret007 == nil {
		t.Fatal("RET-007 not found")
	}
	if !ret007.Pass {
		t.Fatalf("RET-007 did not pass: %s", ret007.Detail)
	}
	if !strings.Contains(ret007.Detail, "×0.5") {
		t.Errorf("RET-007 detail does not mention the ×0.5 penalty: %q", ret007.Detail)
	}
}

// TestRetrieval_ScoreBounds verifies the MinScore/MaxScore assertions fire
// correctly by feeding scenarios that violate them.
func TestRetrieval_ScoreBounds(t *testing.T) {
	cfg := config.Default().Retrieval

	// A MinScore set absurdly high must fail even for a strong hit.
	tooHigh := RetrievalScenario{
		ID:          "TEST-MIN",
		Name:        "impossible min score",
		Recipes:     RetrievalSuite()[3].Recipes, // the full set
		Query:       RetrievalSuite()[3].Query,   // RET-004's strong query
		ExpectTop:   "csv-revenue-by-channel",
		ExpectMatch: true,
		MinScore:    1.5,
	}
	res := runOne(tooHigh, cfg)
	if res.Pass {
		t.Errorf("expected MinScore=1.5 to fail, but scenario passed: %s", res.Detail)
	}

	// A MaxScore set absurdly low must fail for a strong hit.
	tooLow := tooHigh
	tooLow.ID = "TEST-MAX"
	tooLow.MinScore = 0
	tooLow.MaxScore = 0.1
	res = runOne(tooLow, cfg)
	if res.Pass {
		t.Errorf("expected MaxScore=0.1 to fail, but scenario passed: %s", res.Detail)
	}
}

// runOne evaluates a single RetrievalScenario through the same logic as
// RunRetrieval (by running a one-element suite via a local helper).
func runOne(sc RetrievalScenario, cfg config.Retrieval) Result {
	sum := runRetrievalScenarios([]RetrievalScenario{sc}, cfg)
	return sum.Results[0]
}

// TestRunE2EScenarios_AllPass asserts the isolated fixture-driven e2e suite is
// green and covers E2E-001..005, and that it does NOT touch the live recipes/
// library (it runs entirely in a temp dir, so no Store is involved).
func TestRunE2EScenarios_AllPass(t *testing.T) {
	sum, err := RunE2EScenarios()
	if err != nil {
		t.Fatalf("RunE2EScenarios error: %v", err)
	}
	if sum.Failed != 0 {
		for _, r := range sum.Results {
			if !r.Pass {
				t.Errorf("scenario %s failed: %s", r.ID, r.Detail)
			}
		}
	}
	want := []string{"E2E-001", "E2E-002", "E2E-003", "E2E-004", "E2E-005"}
	got := map[string]Result{}
	for _, r := range sum.Results {
		got[r.ID] = r
	}
	for _, id := range want {
		r, ok := got[id]
		if !ok {
			t.Errorf("missing e2e scenario %s", id)
			continue
		}
		if !r.Pass {
			t.Errorf("e2e scenario %s failed: %s", id, r.Detail)
		}
	}
}

// TestE2E_NegativeGates asserts the negative-gate scenarios specifically: a
// PASS here means the trust gate correctly *withheld* trust. This guards the
// wrong-answer-amplifier failure mode the verifier-first thesis exists to
// prevent.
func TestE2E_NegativeGates(t *testing.T) {
	sum, err := RunE2EScenarios()
	if err != nil {
		t.Fatalf("RunE2EScenarios error: %v", err)
	}
	byID := map[string]Result{}
	for _, r := range sum.Results {
		byID[r.ID] = r
	}
	// E2E-001 failing verifier, E2E-002 missing artifact, E2E-003 missing input.
	for _, id := range []string{"E2E-001", "E2E-002", "E2E-003"} {
		r := byID[id]
		if !r.Pass {
			t.Errorf("negative-gate %s should PASS (correctly withholds trust), got fail: %s", id, r.Detail)
		}
		if !strings.Contains(strings.ToLower(r.Detail), "not trusted") &&
			!strings.Contains(strings.ToLower(r.Detail), "aborted") {
			t.Errorf("negative-gate %s detail should reflect a withheld/aborted outcome: %q", id, r.Detail)
		}
	}
}
