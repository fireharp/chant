// Package retrieve ranks recipes against a task query. It is deterministic by
// design (no embeddings, no network) so a `chant suggest` result is
// reproducible and testable. An optional semantic pass can be layered later,
// gated like coherence's optional LLM pass.
//
// The score is a weighted blend:
//
//	score = w_lexical * lexical(query, recipe text)
//	      + w_tags    * signal_match(query/files, recipe signals)
//	      + w_success * verifier_success_rate
//
// A score above the configured threshold is a *candidate* match. Per chant's
// verifier-first thesis a candidate is never trusted until a verifier confirms
// it — retrieval ranks, it does not bless.
package retrieve

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/fireharp/chant/internal/config"
	"github.com/fireharp/chant/internal/glob"
	"github.com/fireharp/chant/internal/recipe"
)

// Query is the input to retrieval.
type Query struct {
	// Task is the natural-language description of what the agent wants to do.
	Task string
	// Files are the input file paths/names in play (used for signal matching).
	Files []string
	// Columns are column names available in the input (for data recipes).
	Columns []string
}

// Match is one scored recipe.
type Match struct {
	Recipe      *recipe.Recipe
	Score       float64
	Lexical     float64
	SignalMatch float64
	SuccessRate float64
	Reasons     []string
}

// Rank scores every recipe against the query and returns matches sorted by
// descending score. Stale recipes are included but penalized so a fresh
// alternative outranks them.
func Rank(recs []*recipe.Recipe, q Query, cfg config.Retrieval) []Match {
	qTokens := tokenize(q.Task)
	var matches []Match
	for _, r := range recs {
		m := score(r, q, qTokens, cfg)
		matches = append(matches, m)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Recipe.ID < matches[j].Recipe.ID
	})
	return matches
}

// Suggest returns only matches at or above the configured threshold.
func Suggest(recs []*recipe.Recipe, q Query, cfg config.Retrieval) []Match {
	var out []Match
	for _, m := range Rank(recs, q, cfg) {
		if m.Score >= cfg.Threshold {
			out = append(out, m)
		}
	}
	return out
}

func score(r *recipe.Recipe, q Query, qTokens map[string]int, cfg config.Retrieval) Match {
	m := Match{Recipe: r}

	// 1. Lexical similarity against task patterns + description + tags.
	corpus := strings.Join(append([]string{r.Description}, r.WhenToUse.TaskPatterns...), " ")
	corpus += " " + strings.Join(r.WhenToUse.Tags, " ")
	m.Lexical = lexical(qTokens, tokenize(corpus))
	if m.Lexical > 0 {
		m.Reasons = append(m.Reasons, "task text overlaps recipe description/patterns")
	}

	// 2. Structural signal match (file globs + column aliases).
	m.SignalMatch = signalMatch(r.WhenToUse.InputSignals, q, &m)

	// 3. Verifier track record.
	m.SuccessRate = r.Metrics.SuccessRate()

	m.Score = cfg.WeightLexical*m.Lexical +
		cfg.WeightTags*m.SignalMatch +
		cfg.WeightSuccessRate*m.SuccessRate

	// Penalize stale recipes so an active alternative ranks first, but keep
	// them retrievable (a verifier can re-bless a stale recipe).
	if r.IsStale() {
		m.Score *= 0.5
		m.Reasons = append(m.Reasons, "recipe is stale — re-verify before trusting")
	}
	return m
}

// lexical is token-overlap similarity weighted toward query coverage. Returns
// the fraction of distinct query tokens present in the corpus. This rewards a
// recipe that "speaks to" the whole query rather than one that merely shares a
// common word.
func lexical(query, corpus map[string]int) float64 {
	if len(query) == 0 {
		return 0
	}
	hit := 0
	for tok := range query {
		if corpus[tok] > 0 {
			hit++
		}
	}
	return float64(hit) / float64(len(query))
}

// signalMatch scores the structural preconditions. Files and columns each
// contribute up to 0.5; a recipe with no declared signals scores 0 here and
// relies on lexical similarity alone.
func signalMatch(sig recipe.InputSignals, q Query, m *Match) float64 {
	var s float64
	if len(sig.Files) > 0 && len(q.Files) > 0 {
		if anyFileMatches(sig.Files, q.Files) {
			s += 0.5
			m.Reasons = append(m.Reasons, "input files match recipe file signal")
		}
	}
	if len(sig.ColumnsAny) > 0 && len(q.Columns) > 0 {
		if columnsCover(sig.ColumnsAny, q.Columns) {
			s += 0.5
			m.Reasons = append(m.Reasons, "input columns satisfy recipe column aliases")
		}
	}
	return s
}

func anyFileMatches(patterns, files []string) bool {
	for _, p := range patterns {
		for _, f := range files {
			if glob.Match(p, filepath.Base(f)) || glob.Match(p, f) {
				return true
			}
		}
	}
	return false
}

// columnsCover is true when every alias group has at least one member present
// in the available columns — i.e. the recipe's required logical columns are
// all satisfiable.
func columnsCover(groups [][]string, available []string) bool {
	have := make(map[string]bool, len(available))
	for _, c := range available {
		have[strings.ToLower(strings.TrimSpace(c))] = true
	}
	for _, group := range groups {
		ok := false
		for _, alias := range group {
			if have[strings.ToLower(strings.TrimSpace(alias))] {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "of": true, "to": true, "in": true,
	"on": true, "for": true, "and": true, "or": true, "with": true, "from": true,
	"by": true, "this": true, "that": true, "is": true, "are": true, "be": true,
	"how": true, "do": true, "we": true, "i": true, "it": true, "as": true,
}

// tokenize lowercases, splits on non-alphanumerics, and drops stopwords and
// 1-char tokens. Returns a term-frequency map.
func tokenize(s string) map[string]int {
	out := map[string]int{}
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tok := b.String()
		b.Reset()
		if len(tok) <= 1 || stopwords[tok] {
			return
		}
		out[tok]++
	}
	for _, c := range strings.ToLower(s) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else {
			flush()
		}
	}
	flush()
	return out
}
