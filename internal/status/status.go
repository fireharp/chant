// Package status rewrites .chant/STATUS.md — a human-readable snapshot of the
// recipe library, mirroring coherence's STATUS.md convention.
package status

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fireharp/chant/internal/store"
)

// Report is the structured form of STATUS, also emitted by `chant status --json`.
type Report struct {
	GeneratedAt  string         `json:"generated_at"`
	RecipeCount  int            `json:"recipe_count"`
	ActiveCount  int            `json:"active_count"`
	StaleCount   int            `json:"stale_count"`
	TotalRuns    int            `json:"total_runs"`
	TotalFailures int           `json:"total_failures"`
	Recipes      []RecipeStat   `json:"recipes"`
}

// RecipeStat is one row of the status table.
type RecipeStat struct {
	ID          string  `json:"id"`
	Version     int     `json:"version"`
	Status      string  `json:"status"`
	Runs        int     `json:"runs"`
	Failures    int     `json:"failures"`
	SuccessRate float64 `json:"success_rate"`
	HasVerifier bool    `json:"has_verifier"`
}

// Build computes the status report from the recipe library.
func Build(s *store.Store) (*Report, error) {
	recs, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	rep := &Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		RecipeCount: len(recs),
	}
	for _, r := range recs {
		rep.TotalRuns += r.Metrics.Runs
		rep.TotalFailures += r.Metrics.Failures
		if r.IsStale() {
			rep.StaleCount++
		} else {
			rep.ActiveCount++
		}
		rep.Recipes = append(rep.Recipes, RecipeStat{
			ID:          r.ID,
			Version:     r.Version,
			Status:      r.Status,
			Runs:        r.Metrics.Runs,
			Failures:    r.Metrics.Failures,
			SuccessRate: r.Metrics.SuccessRate(),
			HasVerifier: r.Verification.Command != "" || len(r.Verification.ExpectedArtifacts) > 0,
		})
	}
	sort.Slice(rep.Recipes, func(i, j int) bool { return rep.Recipes[i].ID < rep.Recipes[j].ID })
	return rep, nil
}

// Write renders the report to .chant/STATUS.md.
func Write(s *store.Store, rep *Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# chant status\n\n")
	fmt.Fprintf(&b, "Generated %s\n\n", rep.GeneratedAt)
	fmt.Fprintf(&b, "- recipes: **%d** (%d active, %d stale)\n", rep.RecipeCount, rep.ActiveCount, rep.StaleCount)
	fmt.Fprintf(&b, "- recorded runs: %d (%d failures)\n\n", rep.TotalRuns, rep.TotalFailures)

	if len(rep.Recipes) == 0 {
		b.WriteString("_No recipes captured yet. Run `chant capture` after the agent solves a task._\n")
	} else {
		b.WriteString("| recipe | v | status | runs | fails | success | verifier |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, r := range rep.Recipes {
			ver := "—"
			if r.HasVerifier {
				ver = "yes"
			}
			fmt.Fprintf(&b, "| `%s` | %d | %s | %d | %d | %.0f%% | %s |\n",
				r.ID, r.Version, r.Status, r.Runs, r.Failures, r.SuccessRate*100, ver)
		}
	}
	b.WriteString("\n> A recipe with a verifier can be *trusted* on reuse only after " +
		"`chant verify` passes. Retrieval ranks candidates; the verifier blesses them.\n")

	if err := os.MkdirAll(s.StatePath(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.StatePath("STATUS.md"), []byte(b.String()), 0o644)
}
