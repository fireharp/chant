package retrieve

import (
	"reflect"
	"testing"

	"github.com/fireharp/chant/internal/config"
	"github.com/fireharp/chant/internal/recipe"
)

func TestTokenizeDropsStopwordsAndShortTokens(t *testing.T) {
	got := tokenize("Compute the revenue by channel from a CSV")
	// "the", "by", "from", "a" are stopwords; nothing here is 1-char.
	want := map[string]int{"compute": 1, "revenue": 1, "channel": 1, "csv": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize = %v, want %v", got, want)
	}
}

func TestTokenizeOneCharDropped(t *testing.T) {
	got := tokenize("a b c hello 9 99")
	// single chars (a,b,c,9) dropped; "hello" and "99" survive.
	want := map[string]int{"hello": 1, "99": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenize = %v, want %v", got, want)
	}
}

func TestTokenizeTermFrequency(t *testing.T) {
	got := tokenize("revenue revenue channel")
	if got["revenue"] != 2 {
		t.Errorf("term frequency for 'revenue' = %d, want 2", got["revenue"])
	}
	if got["channel"] != 1 {
		t.Errorf("term frequency for 'channel' = %d, want 1", got["channel"])
	}
}

func TestTokenizeEmpty(t *testing.T) {
	if got := tokenize("   the a of !!!  "); len(got) != 0 {
		t.Errorf("tokenize of stopwords/punct = %v, want empty", got)
	}
}

func TestColumnsCover(t *testing.T) {
	groups := [][]string{
		{"channel", "source", "utm_source"},
		{"price", "amount", "revenue"},
	}
	tests := []struct {
		name      string
		available []string
		want      bool
	}{
		{"both groups covered", []string{"utm_source", "amount"}, true},
		{"first alias of each", []string{"channel", "price"}, true},
		{"case insensitive", []string{"UTM_SOURCE", "Revenue"}, true},
		{"whitespace trimmed", []string{" channel ", " price "}, true},
		{"second group uncovered", []string{"channel"}, false},
		{"first group uncovered", []string{"price"}, false},
		{"none covered", []string{"unrelated", "columns"}, false},
		{"extra columns ok", []string{"channel", "price", "noise", "id"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := columnsCover(groups, tt.available); got != tt.want {
				t.Errorf("columnsCover(%v) = %v, want %v", tt.available, got, tt.want)
			}
		})
	}
}

func TestLexicalCoverage(t *testing.T) {
	query := tokenize("compute revenue channel csv") // 4 distinct tokens

	tests := []struct {
		name   string
		corpus string
		want   float64
	}{
		{"full coverage", "compute the revenue by channel from csv", 1.0},
		{"half coverage", "compute revenue", 0.5},
		{"quarter coverage", "revenue numbers only", 0.25},
		{"no coverage", "rotate kubernetes certificates", 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lexical(query, tokenize(tt.corpus))
			if got != tt.want {
				t.Errorf("lexical(%q) = %v, want %v", tt.corpus, got, tt.want)
			}
		})
	}
}

func TestLexicalEmptyQuery(t *testing.T) {
	if got := lexical(map[string]int{}, tokenize("anything here")); got != 0 {
		t.Errorf("lexical(empty query) = %v, want 0", got)
	}
}

// helper to build a minimal recipe for ranking tests.
func mkRecipe(id, desc string, patterns, tags, files []string, cols [][]string, runs, fails int) *recipe.Recipe {
	return &recipe.Recipe{
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
}

func TestRankOrdering(t *testing.T) {
	cfg := config.Default().Retrieval
	revenue := mkRecipe("csv-revenue", "Compute ecommerce revenue by channel from CSV",
		[]string{"compute revenue by channel from csv"},
		[]string{"csv", "revenue"}, []string{"*.csv"},
		[][]string{{"channel", "utm_source"}, {"amount", "price"}}, 12, 1)
	refund := mkRecipe("refund", "Approve a refund after a chargeback threat",
		[]string{"refund payment after chargeback"},
		[]string{"refund", "payments"}, nil, nil, 8, 0)

	q := Query{Task: "calculate revenue by channel from this CSV export",
		Files: []string{"orders.csv"}, Columns: []string{"utm_source", "amount"}}

	matches := Rank([]*recipe.Recipe{refund, revenue}, q, cfg)
	if len(matches) != 2 {
		t.Fatalf("Rank returned %d matches, want 2", len(matches))
	}
	if matches[0].Recipe.ID != "csv-revenue" {
		t.Errorf("top match = %q, want csv-revenue", matches[0].Recipe.ID)
	}
	if matches[0].Score < matches[1].Score {
		t.Errorf("matches not sorted descending: %.3f then %.3f", matches[0].Score, matches[1].Score)
	}
	if matches[0].SignalMatch != 1.0 {
		t.Errorf("expected full signal match (files+columns) = 1.0, got %v", matches[0].SignalMatch)
	}
}

func TestRankTieBreakByID(t *testing.T) {
	cfg := config.Default().Retrieval
	// Two recipes with identical (zero) scores against an unrelated query.
	a := mkRecipe("zebra", "totally unrelated topic alpha", nil, nil, nil, nil, 0, 0)
	b := mkRecipe("apple", "totally unrelated topic alpha", nil, nil, nil, nil, 0, 0)
	q := Query{Task: "rotate kubernetes tls certificates"}
	matches := Rank([]*recipe.Recipe{a, b}, q, cfg)
	if matches[0].Score != matches[1].Score {
		t.Skip("scores differ; tie-break not exercised")
	}
	if matches[0].Recipe.ID != "apple" {
		t.Errorf("tie-break: first = %q, want apple (lower id)", matches[0].Recipe.ID)
	}
}

func TestSuggestThresholdTrueNegative(t *testing.T) {
	cfg := config.Default().Retrieval
	revenue := mkRecipe("csv-revenue", "Compute ecommerce revenue by channel from CSV",
		[]string{"compute revenue by channel from csv"},
		[]string{"csv", "revenue"}, []string{"*.csv"}, nil, 12, 1)

	// Unrelated query: must NOT clear the threshold.
	q := Query{Task: "rotate the kubernetes TLS certificates in the staging cluster"}
	matches := Suggest([]*recipe.Recipe{revenue}, q, cfg)
	if len(matches) != 0 {
		t.Errorf("Suggest on unrelated query returned %d matches, want 0 (top=%v)", len(matches), matches)
	}
}

func TestSuggestThresholdPositive(t *testing.T) {
	cfg := config.Default().Retrieval
	revenue := mkRecipe("csv-revenue", "Compute ecommerce revenue by channel from CSV",
		[]string{"compute revenue by channel from csv"},
		[]string{"csv", "revenue"}, []string{"*.csv"},
		[][]string{{"channel", "utm_source"}, {"amount"}}, 12, 1)
	q := Query{Task: "compute revenue by channel from csv", Files: []string{"x.csv"}, Columns: []string{"channel", "amount"}}
	matches := Suggest([]*recipe.Recipe{revenue}, q, cfg)
	if len(matches) != 1 {
		t.Fatalf("Suggest on matching query returned %d matches, want 1", len(matches))
	}
}

func TestStalePenalty(t *testing.T) {
	cfg := config.Default().Retrieval
	// Two identical recipes; one stale. The active one must outrank the stale.
	active := mkRecipe("active-rec", "compute revenue by channel from csv",
		[]string{"compute revenue by channel from csv"}, []string{"csv"}, nil, nil, 10, 0)
	stale := mkRecipe("stale-rec", "compute revenue by channel from csv",
		[]string{"compute revenue by channel from csv"}, []string{"csv"}, nil, nil, 10, 0)
	stale.MarkStale()

	q := Query{Task: "compute revenue by channel from csv"}
	matches := Rank([]*recipe.Recipe{stale, active}, q, cfg)

	var activeScore, staleScore float64
	for _, m := range matches {
		if m.Recipe.ID == "active-rec" {
			activeScore = m.Score
		} else {
			staleScore = m.Score
		}
	}
	if staleScore >= activeScore {
		t.Errorf("stale score %.3f should be below active score %.3f", staleScore, activeScore)
	}
	// The penalty is exactly half.
	if staleScore != activeScore*0.5 {
		t.Errorf("stale score %.4f, want half of active %.4f", staleScore, activeScore)
	}
	if matches[0].Recipe.ID != "active-rec" {
		t.Errorf("top = %q, want active-rec", matches[0].Recipe.ID)
	}
	// Stale match should carry an explanatory reason.
	for _, m := range matches {
		if m.Recipe.ID == "stale-rec" {
			found := false
			for _, reason := range m.Reasons {
				if reason == "recipe is stale — re-verify before trusting" {
					found = true
				}
			}
			if !found {
				t.Errorf("stale recipe missing stale reason: %v", m.Reasons)
			}
		}
	}
}

func TestSignalMatchFilesOnly(t *testing.T) {
	cfg := config.Default().Retrieval
	rec := mkRecipe("files-rec", "noop", nil, nil, []string{"*.csv"}, nil, 0, 0)
	q := Query{Task: "irrelevant", Files: []string{"data/orders.csv"}}
	matches := Rank([]*recipe.Recipe{rec}, q, cfg)
	if matches[0].SignalMatch != 0.5 {
		t.Errorf("file-only signal match = %v, want 0.5", matches[0].SignalMatch)
	}
}

func TestSignalMatchNoSignals(t *testing.T) {
	cfg := config.Default().Retrieval
	rec := mkRecipe("no-sig", "noop recipe", nil, nil, nil, nil, 0, 0)
	q := Query{Task: "noop recipe"}
	matches := Rank([]*recipe.Recipe{rec}, q, cfg)
	if matches[0].SignalMatch != 0 {
		t.Errorf("signal match with no declared signals = %v, want 0", matches[0].SignalMatch)
	}
}
