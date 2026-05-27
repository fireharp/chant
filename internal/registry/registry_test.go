package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fireharp/chant/internal/config"
	"github.com/fireharp/chant/internal/retrieve"
)

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "registry", "index.json")
}

func TestDefaultPathEnvOverride(t *testing.T) {
	want := "/tmp/some/registry.json"
	t.Setenv(EnvPath, want)
	if got := DefaultPath(); got != want {
		t.Errorf("DefaultPath with %s set = %q, want %q", EnvPath, got, want)
	}
}

func TestDefaultPathFallsBackToHome(t *testing.T) {
	t.Setenv(EnvPath, "")
	// Unset entirely so os.Getenv returns "".
	os.Unsetenv(EnvPath)
	got := DefaultPath()
	if filepath.Base(got) != "index.json" {
		t.Errorf("DefaultPath = %q, want a .../index.json path", got)
	}
}

func TestUpsertDedupsBySpellHash(t *testing.T) {
	reg := &Registry{path: tempPath(t)}

	n := reg.Upsert(
		Entry{SpellHash: "aaa", ID: "alpha", Version: 1, RecipePath: "/A/recipes/alpha"},
		Entry{SpellHash: "bbb", ID: "beta", Version: 1, RecipePath: "/A/recipes/beta"},
	)
	if n != 2 || len(reg.Entries) != 2 {
		t.Fatalf("first upsert: n=%d entries=%d, want 2/2", n, len(reg.Entries))
	}

	// Same spell_hash from another repo: replace, do not duplicate.
	reg.Upsert(Entry{
		SpellHash: "aaa", ID: "alpha-renamed", Version: 2,
		RecipePath: "/B/recipes/alpha", Origin: "github.com/x/b",
	})
	if len(reg.Entries) != 2 {
		t.Fatalf("after collision upsert entries=%d, want 2 (dedup by spell_hash)", len(reg.Entries))
	}
	got, ok := reg.Find("aaa")
	if !ok {
		t.Fatal("Find(aaa) not found")
	}
	if got.ID != "alpha-renamed" || got.Version != 2 || got.RecipePath != "/B/recipes/alpha" {
		t.Errorf("newest did not win: %+v", got)
	}

	// Empty spell_hash is skipped (no portable identity to dedup on).
	if reg.Upsert(Entry{ID: "no-hash"}) != 0 {
		t.Error("upsert of empty spell_hash should report 0")
	}
	if len(reg.Entries) != 2 {
		t.Errorf("empty spell_hash added an entry: %d", len(reg.Entries))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := tempPath(t)
	reg := &Registry{path: path}
	reg.Upsert(
		Entry{SpellHash: "h1", ID: "one", Version: 1, RecipePath: "/repo/recipes/one", HasVerifier: true, Tags: []string{"csv"}},
		Entry{SpellHash: "h2", ID: "two", Version: 3, RecipePath: "/repo/recipes/two"},
	)
	if err := reg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(reloaded.Entries) != 2 {
		t.Fatalf("round-trip entries=%d, want 2", len(reloaded.Entries))
	}
	if reloaded.Path() != path {
		t.Errorf("Load did not bind path: %q", reloaded.Path())
	}
	one, ok := reloaded.Find("one")
	if !ok || !one.HasVerifier || len(one.Tags) != 1 || one.Tags[0] != "csv" {
		t.Errorf("round-trip entry mismatch: %+v ok=%v", one, ok)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	path := tempPath(t) // never written
	reg, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing file should not error: %v", err)
	}
	if len(reg.Entries) != 0 {
		t.Errorf("missing-file registry has %d entries, want 0", len(reg.Entries))
	}
	if reg.Path() != path {
		t.Errorf("missing-file registry path = %q, want %q", reg.Path(), path)
	}
}

func TestSearchRanksByRelevance(t *testing.T) {
	reg := &Registry{path: tempPath(t)}
	reg.Upsert(
		Entry{
			SpellHash: "csv1", ID: "csv-revenue", Description: "compute revenue by channel from csv",
			TaskPatterns: []string{"compute revenue by channel from csv"}, Tags: []string{"csv", "revenue"},
			RecipePath: "/A/recipes/csv-revenue",
		},
		Entry{
			SpellHash: "k8s1", ID: "rotate-certs", Description: "rotate kubernetes TLS certificates",
			TaskPatterns: []string{"rotate kubernetes TLS certificates in staging"}, Tags: []string{"k8s"},
			RecipePath: "/A/recipes/rotate-certs",
		},
	)

	cfg := config.Default().Retrieval
	results := reg.Search(retrieve.Query{Task: "compute revenue by channel from this csv"}, cfg)
	if len(results) == 0 {
		t.Fatal("Search returned no results for a strongly matching query")
	}
	if results[0].Entry.ID != "csv-revenue" {
		t.Errorf("top result = %q, want csv-revenue", results[0].Entry.ID)
	}
	// The k8s entry should not clear the threshold for a revenue query.
	for _, r := range results {
		if r.Entry.ID == "rotate-certs" {
			t.Errorf("unrelated rotate-certs scored above threshold (%.2f) for a revenue query", r.Score)
		}
	}

	// True negative: an unrelated query returns nothing above threshold.
	none := reg.Search(retrieve.Query{Task: "deploy a helm chart to production"}, cfg)
	if len(none) != 0 {
		t.Errorf("unrelated query returned %d results, want 0: %+v", len(none), none)
	}
}
