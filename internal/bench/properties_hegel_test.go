//go:build hegel

package bench

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fireharp/chant/internal/config"
	"github.com/fireharp/chant/internal/recipe"
	"github.com/fireharp/chant/internal/retrieve"
	"github.com/fireharp/chant/internal/runner"
	"hegel.dev/go/hegel"
)

func TestRunProperties_AllPass(t *testing.T) {
	sum := RunProperties(repoRoot(t))
	if sum.Failed != 0 {
		for _, r := range sum.Results {
			if !r.Pass {
				t.Errorf("%s failed: %s", r.ID, r.Detail)
			}
		}
	}
	if sum.Suite != "properties" || sum.Total != 1 {
		t.Errorf("summary = %+v, want one properties result", sum)
	}
}

func TestHegelProperties(t *testing.T) {
	setupHegel(t)

	t.Run("retrieval stale penalty", propRetrievalStalePenalty)
	t.Run("retrieval signal monotonicity", propRetrievalSignalMonotonicity)
	t.Run("runner trust gate", propRunnerTrustGate)
	t.Run("spell hash stability", propSpellHashStability)
	t.Run("csv recipe oracle", propCSVRecipeOracle)
}

func setupHegel(t *testing.T) {
	t.Helper()
	dir := os.Getenv("CHANT_HEGEL_DIR")
	if dir == "" {
		dir = filepath.Join(repoRoot(t), ".chant", "hegel", ".hegel")
	}
	if err := os.MkdirAll(filepath.Join(dir, "db"), 0o755); err != nil {
		t.Fatalf("create Hegel dir: %v", err)
	}
	hegel.SetHegelDirectory(dir)
}

func hegelOpts(t *testing.T, cases int) []hegel.Option {
	t.Helper()
	dir := os.Getenv("CHANT_HEGEL_DIR")
	if dir == "" {
		dir = filepath.Join(repoRoot(t), ".chant", "hegel", ".hegel")
	}
	db := filepath.Join(dir, "db")
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatalf("create Hegel db: %v", err)
	}
	return []hegel.Option{
		hegel.WithTestCases(cases),
		hegel.WithDatabase(hegel.Database(db)),
		hegel.WithDerandomize(true),
	}
}

func propRetrievalStalePenalty(t *testing.T) {
	cfg := config.Default().Retrieval
	tasks := []string{
		"compute revenue by channel from csv",
		"analyze ecommerce orders export",
		"revenue breakdown by marketing channel",
	}
	files := []string{"orders.csv", "exports/orders.csv", "orders.tsv"}
	channelAliases := []string{"channel", "source", "utm_source"}
	revenueAliases := []string{"revenue", "amount", "price", "total"}

	hegel.Test(t, func(ht *hegel.T) {
		runs := hegel.Draw(ht, hegel.Integers(0, 50))
		fails := hegel.Draw(ht, hegel.Integers(0, runs))
		task := hegel.Draw(ht, hegel.SampledFrom(tasks))
		file := hegel.Draw(ht, hegel.SampledFrom(files))
		channel := hegel.Draw(ht, hegel.SampledFrom(channelAliases))
		revenue := hegel.Draw(ht, hegel.SampledFrom(revenueAliases))

		active := revenueRecipe("active", runs, fails)
		stale := revenueRecipe("stale", runs, fails)
		stale.MarkStale()
		q := retrieve.Query{Task: task, Files: []string{file}, Columns: []string{channel, revenue}}

		activeScore := retrieve.Rank([]*recipe.Recipe{active}, q, cfg)[0].Score
		staleHit := retrieve.Rank([]*recipe.Recipe{stale}, q, cfg)[0]
		if !near(staleHit.Score, activeScore*0.5) {
			ht.Fatalf("stale score %.6f, active %.6f; want exact half", staleHit.Score, activeScore)
		}
		if !hasReason(staleHit, "stale") {
			ht.Fatalf("stale hit did not carry stale reason: %v", staleHit.Reasons)
		}
	}, hegelOpts(t, 50)...)
}

func propRetrievalSignalMonotonicity(t *testing.T) {
	cfg := config.Default().Retrieval
	channelAliases := []string{"channel", "source", "utm_source"}
	revenueAliases := []string{"revenue", "amount", "price", "total"}
	files := []string{"orders.csv", "exports/orders.csv"}
	badColumns := []string{"sku", "customer", "created_at", "country"}

	hegel.Test(t, func(ht *hegel.T) {
		channel := hegel.Draw(ht, hegel.SampledFrom(channelAliases))
		revenue := hegel.Draw(ht, hegel.SampledFrom(revenueAliases))
		file := hegel.Draw(ht, hegel.SampledFrom(files))
		badA := hegel.Draw(ht, hegel.SampledFrom(badColumns))
		badB := hegel.Draw(ht, hegel.SampledFrom(badColumns))
		r := revenueRecipe("revenue", 10, 0)

		base := retrieve.Rank([]*recipe.Recipe{r}, retrieve.Query{Task: "compute revenue by channel"}, cfg)[0]
		matched := retrieve.Rank([]*recipe.Recipe{r}, retrieve.Query{
			Task:    "compute revenue by channel",
			Files:   []string{file},
			Columns: []string{channel, revenue},
		}, cfg)[0]
		if matched.Score < base.Score {
			ht.Fatalf("matching structural signals lowered score: base %.6f matched %.6f", base.Score, matched.Score)
		}
		if matched.SignalMatch != 1.0 {
			ht.Fatalf("matching file+columns signal = %.2f, want 1.0", matched.SignalMatch)
		}

		unsatisfied := retrieve.Rank([]*recipe.Recipe{r}, retrieve.Query{
			Task:    "compute revenue by channel",
			Columns: []string{badA, badB},
		}, cfg)[0]
		if unsatisfied.SignalMatch != 0 {
			ht.Fatalf("unsatisfied columns earned signal %.2f", unsatisfied.SignalMatch)
		}
		if !near(unsatisfied.Score, base.Score) {
			ht.Fatalf("unsatisfied columns changed lexical-only score: base %.6f unsatisfied %.6f", base.Score, unsatisfied.Score)
		}
	}, hegelOpts(t, 50)...)
}

func propRunnerTrustGate(t *testing.T) {
	hegel.Test(t, func(ht *hegel.T) {
		verifierPasses := hegel.Draw(ht, hegel.Booleans())
		artifactDeclared := hegel.Draw(ht, hegel.Booleans())
		artifactExists := hegel.Draw(ht, hegel.Booleans())

		dir, err := os.MkdirTemp("", "chant-hegel-runner-")
		if err != nil {
			ht.Fatalf("temp dir: %v", err)
		}
		defer os.RemoveAll(dir)

		verifier := "true"
		if !verifierPasses {
			verifier = "false"
		}
		var artifacts []string
		if artifactDeclared {
			artifacts = []string{"out.txt"}
			if artifactExists {
				if err := os.WriteFile(filepath.Join(dir, "out.txt"), []byte("ok\n"), 0o644); err != nil {
					ht.Fatalf("write artifact: %v", err)
				}
			}
		}
		rc := &recipe.Recipe{
			ID:           "runner-prop",
			WhatToDo:     recipe.WhatToDo{Command: "true"},
			Verification: recipe.Verification{Command: verifier, ExpectedArtifacts: artifacts},
		}
		rc.SetDir(dir)

		res, trusted, err := runner.Verify(rc, nil, 5*time.Second)
		if err != nil {
			ht.Fatalf("Verify returned command-level error: %v", err)
		}
		want := verifierPasses && (!artifactDeclared || artifactExists)
		if trusted != want {
			ht.Fatalf("trusted=%v, want %v (verifierPasses=%v artifactDeclared=%v artifactExists=%v res=%+v)",
				trusted, want, verifierPasses, artifactDeclared, artifactExists, res)
		}
	}, hegelOpts(t, 50)...)
}

func propSpellHashStability(t *testing.T) {
	placeholders := []string{"input", "file", "orders", "orders.csv", "input_file"}
	hegel.Test(t, func(ht *hegel.T) {
		aName := hegel.Draw(ht, hegel.SampledFrom(placeholders))
		bName := hegel.Draw(ht, hegel.SampledFrom(placeholders))
		swapGroups := hegel.Draw(ht, hegel.Booleans())
		swapAliases := hegel.Draw(ht, hegel.Booleans())

		colsA := [][]string{{"channel", "source"}, {"amount", "revenue"}}
		colsB := [][]string{{"source", "channel"}, {"revenue", "amount"}}
		if swapAliases {
			colsB = [][]string{{"channel", "source"}, {"amount", "revenue"}}
		}
		if swapGroups {
			colsB[0], colsB[1] = colsB[1], colsB[0]
		}

		a := &recipe.Recipe{
			WhatToDo:    recipe.WhatToDo{Command: fmt.Sprintf("python3 run.py {{%s}}", aName)},
			Portability: recipe.Portability{InputContract: recipe.InputContract{RequiredColumnsAny: colsA}},
		}
		b := &recipe.Recipe{
			WhatToDo:    recipe.WhatToDo{Command: fmt.Sprintf("python3 \t run.py   {{ %s }}", bName)},
			Portability: recipe.Portability{InputContract: recipe.InputContract{RequiredColumnsAny: colsB}},
		}
		if a.ComputeSpellHash() != b.ComputeSpellHash() {
			ht.Fatalf("equivalent spell hashes differed: %s != %s", a.ComputeSpellHash(), b.ComputeSpellHash())
		}
	}, hegelOpts(t, 50)...)
}

func propCSVRecipeOracle(t *testing.T) {
	root := repoRoot(t)
	channelAliases := []string{"channel", "source", "utm_source"}
	revenueAliases := []string{"revenue", "amount", "price", "total"}
	rowGen := hegel.Composite(func(tc hegel.TestCase) csvPropRow {
		return csvPropRow{
			Channel: hegel.Draw(tc, hegel.SampledFrom([]string{"google", "facebook", "direct", "email", "", " google "})),
			Revenue: hegel.Draw(tc, hegel.SampledFrom([]string{"0", "1", "1.25", "200", "bad", "", " -3.5 ", "2.5"})),
		}
	})

	hegel.Test(t, func(ht *hegel.T) {
		channelCol := hegel.Draw(ht, hegel.SampledFrom(channelAliases))
		revenueCol := hegel.Draw(ht, hegel.SampledFrom(revenueAliases))
		rows := hegel.Draw(ht, hegel.Lists(rowGen).MaxSize(12))
		want := csvOracle(rows)

		got, err := runCSVRecipe(root, channelCol, revenueCol, rows)
		if err != nil {
			ht.Fatalf("run csv recipe: %v", err)
		}
		if !mapsNear(got, want) {
			ht.Fatalf("csv totals mismatch:\n got=%v\nwant=%v\nrows=%+v", got, want, rows)
		}

		reversed := append([]csvPropRow(nil), rows...)
		for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
			reversed[i], reversed[j] = reversed[j], reversed[i]
		}
		gotReversed, err := runCSVRecipe(root, channelCol, revenueCol, reversed)
		if err != nil {
			ht.Fatalf("run reversed csv recipe: %v", err)
		}
		if !mapsNear(gotReversed, want) {
			ht.Fatalf("csv totals changed under row reversal:\n got=%v\nwant=%v\nrows=%+v", gotReversed, want, reversed)
		}
	}, hegelOpts(t, 25)...)
}

func revenueRecipe(id string, runs, fails int) *recipe.Recipe {
	return &recipe.Recipe{
		ID:          id,
		Version:     1,
		Description: "Compute ecommerce revenue by channel from CSV-like exports",
		WhenToUse: recipe.WhenToUse{
			TaskPatterns: []string{"compute revenue by channel from csv", "analyze ecommerce orders export", "revenue breakdown by marketing channel"},
			Tags:         []string{"csv", "ecommerce", "revenue", "analytics"},
			InputSignals: recipe.InputSignals{
				Files:      []string{"*.csv"},
				ColumnsAny: [][]string{{"channel", "source", "utm_source"}, {"revenue", "amount", "price", "total"}},
			},
		},
		Metrics: recipe.Metrics{Runs: runs, Failures: fails},
	}
}

func hasReason(m retrieve.Match, needle string) bool {
	for _, reason := range m.Reasons {
		if strings.Contains(reason, needle) {
			return true
		}
	}
	return false
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

type csvPropRow struct {
	Channel string
	Revenue string
}

func runCSVRecipe(root, channelCol, revenueCol string, rows []csvPropRow) (map[string]float64, error) {
	dir, err := os.MkdirTemp("", "chant-hegel-csv-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	src := filepath.Join(root, "recipes", "csv-revenue-by-channel", "run.py")
	b, err := os.ReadFile(src)
	if err != nil {
		return nil, err
	}
	runPath := filepath.Join(dir, "run.py")
	if err := os.WriteFile(runPath, b, 0o755); err != nil {
		return nil, err
	}

	csvPath := filepath.Join(dir, "orders.csv")
	f, err := os.Create(csvPath)
	if err != nil {
		return nil, err
	}
	w := csv.NewWriter(f)
	if err := w.Write([]string{channelCol, revenueCol, "note"}); err != nil {
		f.Close()
		return nil, err
	}
	for _, row := range rows {
		if err := w.Write([]string{row.Channel, row.Revenue, "generated"}); err != nil {
			f.Close()
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}

	cmd := exec.Command("python3", runPath, csvPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	var got map[string]float64
	outJSON, err := os.ReadFile(filepath.Join(dir, "revenue_by_channel.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(outJSON, &got); err != nil {
		return nil, err
	}
	return got, nil
}

func csvOracle(rows []csvPropRow) map[string]float64 {
	totals := map[string]float64{}
	for _, row := range rows {
		ch := strings.TrimSpace(row.Channel)
		if ch == "" {
			continue
		}
		rev, err := strconv.ParseFloat(strings.TrimSpace(row.Revenue), 64)
		if err != nil {
			rev = 0
		}
		totals[ch] = math.Round((totals[ch]+rev)*100) / 100
	}
	return totals
}

func mapsNear(a, b map[string]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !near(av, bv) {
			return false
		}
	}
	return true
}

func repoRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("CHANT_REPO_ROOT"); root != "" {
		return root
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "recipes", "csv-revenue-by-channel", "run.py")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", wd)
		}
	}
}
