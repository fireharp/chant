package recipe

import (
	"testing"
	"time"
)

// helper: build a recipe with the given domains and verified contexts.
func mkRecipe(domains []string, contexts ...string) *Recipe {
	r := &Recipe{Domains: domains}
	for _, c := range contexts {
		r.VerifiedIn = append(r.VerifiedIn, VerifiedContext{Context: c, At: "2026-05-28T00:00:00Z"})
	}
	return r
}

// TestComputeScope_DefaultProject: 0 or 1 verified contexts ⇒ project.
func TestComputeScope_DefaultProject(t *testing.T) {
	if got := ComputeScope(mkRecipe([]string{"csv"})); got != ScopeProject {
		t.Errorf("0 contexts: got %q, want %q", got, ScopeProject)
	}
	if got := ComputeScope(mkRecipe([]string{"csv"}, "fireharp/bitgn")); got != ScopeProject {
		t.Errorf("1 context: got %q, want %q", got, ScopeProject)
	}
}

// TestComputeScope_TwoContextsSameDomain: ≥2 contexts under a single domain
// label ⇒ domain.
func TestComputeScope_TwoContextsSameDomain(t *testing.T) {
	r := mkRecipe([]string{"csv"}, "fireharp/bitgn", "fireharp/tinkershop")
	if got := ComputeScope(r); got != ScopeDomain {
		t.Errorf("2 contexts/1 domain: got %q, want %q", got, ScopeDomain)
	}
}

// TestComputeScope_TwoContextsTwoDomainsStillDomain: spec ambiguity decided
// in scope.go's godoc — 2 contexts spanning 2 domains is not enough for
// universal (the universal rule needs ≥3 contexts). It is at least domain.
func TestComputeScope_TwoContextsTwoDomainsStillDomain(t *testing.T) {
	r := mkRecipe([]string{"csv", "ecommerce"}, "ctx-a", "ctx-b")
	if got := ComputeScope(r); got != ScopeDomain {
		t.Errorf("2 contexts/2 domains: got %q, want %q (universal needs ≥3 contexts)", got, ScopeDomain)
	}
}

// TestComputeScope_Universal: ≥3 contexts AND ≥2 domains ⇒ universal.
func TestComputeScope_Universal(t *testing.T) {
	r := mkRecipe([]string{"csv", "ecommerce"}, "ctx-a", "ctx-b", "ctx-c")
	if got := ComputeScope(r); got != ScopeUniversal {
		t.Errorf("3 contexts/2 domains: got %q, want %q", got, ScopeUniversal)
	}
}

// TestComputeScope_UniversalNeedsTwoDomains: ≥3 contexts but only 1 domain
// stays at domain — universal requires spanning ≥2 domains.
func TestComputeScope_UniversalNeedsTwoDomains(t *testing.T) {
	r := mkRecipe([]string{"csv"}, "ctx-a", "ctx-b", "ctx-c")
	if got := ComputeScope(r); got != ScopeDomain {
		t.Errorf("3 contexts/1 domain: got %q, want %q (universal needs ≥2 domains)", got, ScopeDomain)
	}
}

// TestComputeScope_NoDomainsCapsAtProject: without ≥1 domain label, even
// many contexts cannot earn domain or universal.
func TestComputeScope_NoDomainsCapsAtProject(t *testing.T) {
	r := mkRecipe(nil, "ctx-a", "ctx-b", "ctx-c", "ctx-d")
	if got := ComputeScope(r); got != ScopeProject {
		t.Errorf("4 contexts/0 domains: got %q, want %q (no clustering signal)", got, ScopeProject)
	}
}

// TestComputeScope_DedupesContexts: duplicate Context values should NOT count
// twice — promotion is by distinct contexts.
func TestComputeScope_DedupesContexts(t *testing.T) {
	r := mkRecipe([]string{"csv"}, "ctx-a", "ctx-a", "ctx-a")
	if got := ComputeScope(r); got != ScopeProject {
		t.Errorf("same context 3×: got %q, want %q (must dedupe)", got, ScopeProject)
	}
}

// TestComputeScope_IgnoresEmptyContexts: blank Context strings are skipped so
// a corrupt verified_in entry can't inflate the score.
func TestComputeScope_IgnoresEmptyContexts(t *testing.T) {
	r := mkRecipe([]string{"csv"}, "", "  ", "ctx-a")
	if got := ComputeScope(r); got != ScopeProject {
		t.Errorf("only 1 real context: got %q, want %q", got, ScopeProject)
	}
}

// TestComputeScope_NilRecipe: a nil recipe returns project (no panic).
func TestComputeScope_NilRecipe(t *testing.T) {
	if got := ComputeScope(nil); got != ScopeProject {
		t.Errorf("nil recipe: got %q, want %q", got, ScopeProject)
	}
}

// TestRecordVerifiedContext_AppendsAndRefreshes: a new context is appended; a
// repeat refreshes the timestamp without duplicating the entry.
func TestRecordVerifiedContext_AppendsAndRefreshes(t *testing.T) {
	r := &Recipe{}
	t1 := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	if !r.RecordVerifiedContext("ctx-a", t1) {
		t.Error("first record should report change=true")
	}
	if len(r.VerifiedIn) != 1 || r.VerifiedIn[0].Context != "ctx-a" {
		t.Fatalf("after first record: %+v", r.VerifiedIn)
	}

	// Re-record same context with a newer timestamp: list stays length 1.
	t2 := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	r.RecordVerifiedContext("ctx-a", t2)
	if len(r.VerifiedIn) != 1 {
		t.Errorf("re-record duplicated entry: %+v", r.VerifiedIn)
	}
	if r.VerifiedIn[0].At != "2026-05-28T12:00:00Z" {
		t.Errorf("re-record did not refresh timestamp: %q", r.VerifiedIn[0].At)
	}

	// New context appends.
	r.RecordVerifiedContext("ctx-b", t2)
	if len(r.VerifiedIn) != 2 {
		t.Errorf("new context did not append: %+v", r.VerifiedIn)
	}
}

// TestRecordVerifiedContext_IgnoresEmpty: blank context strings are dropped.
func TestRecordVerifiedContext_IgnoresEmpty(t *testing.T) {
	r := &Recipe{}
	if r.RecordVerifiedContext("", time.Now()) {
		t.Error("empty context should report no-op")
	}
	if r.RecordVerifiedContext("   ", time.Now()) {
		t.Error("whitespace-only context should report no-op")
	}
	if len(r.VerifiedIn) != 0 {
		t.Errorf("empty contexts polluted VerifiedIn: %+v", r.VerifiedIn)
	}
}

// TestMaxScope: returns the higher of two scopes; unknown ⇒ project floor.
func TestMaxScope(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{ScopeProject, ScopeDomain, ScopeDomain},
		{ScopeDomain, ScopeProject, ScopeDomain},
		{ScopeUniversal, ScopeDomain, ScopeUniversal},
		{ScopeProject, ScopeUniversal, ScopeUniversal},
		{ScopeDomain, ScopeDomain, ScopeDomain},
		{"", ScopeDomain, ScopeDomain},
		{"weird", "alsoweird", ScopeProject},
	}
	for _, tt := range cases {
		if got := MaxScope(tt.a, tt.b); got != tt.want {
			t.Errorf("MaxScope(%q,%q) = %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}
