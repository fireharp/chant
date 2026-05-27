package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fireharp/chant/internal/config"
	"github.com/fireharp/chant/internal/recipe"
)

// writeRecipe creates recipes/<id>/recipe.yaml under root with the given body.
func writeRecipe(t *testing.T, root, id, body string) {
	t.Helper()
	dir := filepath.Join(root, "recipes", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, recipe.CardFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newStore(root string) *Store {
	return &Store{Root: root, Config: config.Default()}
}

func TestFindRootWalksUpToGit(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(deep)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	// Resolve symlinks (macOS /var → /private/var) before comparing.
	wantR, _ := filepath.EvalSymlinks(root)
	gotR, _ := filepath.EvalSymlinks(got)
	if gotR != wantR {
		t.Errorf("FindRoot = %q, want %q", gotR, wantR)
	}
}

func TestFindRootWalksUpToChantYML(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, config.FileName), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "sub")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(deep)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	wantR, _ := filepath.EvalSymlinks(root)
	gotR, _ := filepath.EvalSymlinks(got)
	if gotR != wantR {
		t.Errorf("FindRoot = %q, want %q", gotR, wantR)
	}
}

func TestFindRootFallsBackToStart(t *testing.T) {
	// No marker anywhere up the tree → falls back to the start dir.
	start := t.TempDir()
	got, err := FindRoot(start)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	wantR, _ := filepath.EvalSymlinks(start)
	gotR, _ := filepath.EvalSymlinks(got)
	if gotR != wantR {
		t.Errorf("FindRoot fallback = %q, want start %q", gotR, wantR)
	}
}

func TestLoadAll(t *testing.T) {
	root := t.TempDir()
	writeRecipe(t, root, "bravo", "description: B\nwhat_to_do:\n  command: echo b\n")
	writeRecipe(t, root, "alpha", "description: A\nwhat_to_do:\n  command: echo a\n")
	// A non-recipe directory (no recipe.yaml) must be skipped.
	if err := os.MkdirAll(filepath.Join(root, "recipes", "not-a-recipe"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A stray file at the recipes top level must be skipped.
	if err := os.WriteFile(filepath.Join(root, "recipes", "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newStore(root)
	recs, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("LoadAll returned %d recipes, want 2", len(recs))
	}
	// Sorted by id.
	if recs[0].ID != "alpha" || recs[1].ID != "bravo" {
		t.Errorf("LoadAll order = [%s, %s], want [alpha, bravo]", recs[0].ID, recs[1].ID)
	}
}

func TestLoadAllMissingDir(t *testing.T) {
	s := newStore(t.TempDir()) // no recipes/ dir at all
	recs, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll on missing dir should not error: %v", err)
	}
	if recs != nil {
		t.Errorf("LoadAll on missing dir = %v, want nil", recs)
	}
}

func TestGetUnknownErrors(t *testing.T) {
	s := newStore(t.TempDir())
	if _, err := s.Get("nope"); err == nil {
		t.Fatal("Get on unknown id should error")
	}
}

func TestGetKnown(t *testing.T) {
	root := t.TempDir()
	writeRecipe(t, root, "known", "description: K\nwhat_to_do:\n  command: echo k\n")
	s := newStore(root)
	r, err := s.Get("known")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if r.ID != "known" || r.Description != "K" {
		t.Errorf("Get returned %+v", r)
	}
}

func TestExists(t *testing.T) {
	root := t.TempDir()
	writeRecipe(t, root, "here", "description: x\nwhat_to_do:\n  command: echo x\n")
	s := newStore(root)
	if !s.Exists("here") {
		t.Error("Exists(here) = false, want true")
	}
	if s.Exists("gone") {
		t.Error("Exists(gone) = true, want false")
	}
}

func TestWriteIndexContent(t *testing.T) {
	root := t.TempDir()
	writeRecipe(t, root, "alpha",
		"description: Alpha recipe\nwhat_to_do:\n  command: echo a\nwhen_to_use:\n  tags: [csv, data]\nmetrics:\n  runs: 4\n  failures: 1\n")
	writeRecipe(t, root, "stale-one",
		"description: Stale recipe\nstatus: stale\nwhat_to_do:\n  command: echo s\n")

	s := newStore(root)
	idx, err := s.WriteIndex()
	if err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}
	if idx.Count != 2 {
		t.Errorf("index Count = %d, want 2", idx.Count)
	}
	if idx.GeneratedAt == "" {
		t.Error("index GeneratedAt empty")
	}

	// The file on disk matches.
	b, err := os.ReadFile(s.StatePath("index.json"))
	if err != nil {
		t.Fatalf("read index.json: %v", err)
	}
	var onDisk Index
	if err := json.Unmarshal(b, &onDisk); err != nil {
		t.Fatalf("unmarshal index.json: %v", err)
	}
	if onDisk.Count != 2 || len(onDisk.Recipes) != 2 {
		t.Errorf("on-disk index = %+v", onDisk)
	}

	// Entries are sorted by id (alpha before stale-one) and carry derived fields.
	a := onDisk.Recipes[0]
	if a.ID != "alpha" {
		t.Fatalf("first entry = %q, want alpha", a.ID)
	}
	if a.Description != "Alpha recipe" {
		t.Errorf("alpha description = %q", a.Description)
	}
	if a.Status != "active" {
		t.Errorf("alpha status = %q, want active (defaulted)", a.Status)
	}
	if a.Runs != 4 || a.Failures != 1 {
		t.Errorf("alpha metrics = %d/%d, want 4/1", a.Runs, a.Failures)
	}
	if a.SuccessRate != 0.75 {
		t.Errorf("alpha success rate = %v, want 0.75", a.SuccessRate)
	}
	if len(a.Tags) != 2 || a.Tags[0] != "csv" {
		t.Errorf("alpha tags = %v, want [csv data]", a.Tags)
	}

	stale := onDisk.Recipes[1]
	if stale.Status != "stale" {
		t.Errorf("stale-one status = %q, want stale", stale.Status)
	}
	// No recorded runs → benefit-of-the-doubt success rate of 1.0.
	if stale.SuccessRate != 1.0 {
		t.Errorf("stale-one success rate = %v, want 1.0 (no runs)", stale.SuccessRate)
	}
}

func TestWriteRun(t *testing.T) {
	root := t.TempDir()
	s := newStore(root)
	rec := RunRecord{
		RecipeID:    "r1",
		Version:     1,
		Command:     "echo hi",
		ExitCode:    0,
		VerifierRan: true,
		Verified:    true,
		Stdout:      "hi\n",
	}
	dir, err := s.WriteRun(rec)
	if err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		t.Fatalf("read run.json: %v", err)
	}
	var got RunRecord
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal run.json: %v", err)
	}
	if got.RecipeID != "r1" || !got.Verified || !got.VerifierRan {
		t.Errorf("run record round-trip = %+v", got)
	}
}

func TestStatePathAndRecipesDir(t *testing.T) {
	s := newStore("/repo")
	if got := s.RecipesDir(); got != filepath.Join("/repo", "recipes") {
		t.Errorf("RecipesDir = %q", got)
	}
	if got := s.StatePath("runs", "x"); got != filepath.Join("/repo", StateDir, "runs", "x") {
		t.Errorf("StatePath = %q", got)
	}
	if got := s.DirFor("foo"); got != filepath.Join("/repo", "recipes", "foo") {
		t.Errorf("DirFor = %q", got)
	}
}

func TestOpenLoadsConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, config.FileName), []byte("version: 1\nrecipes_dir: cookbook\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if s.Config.RecipesDir != "cookbook" {
		t.Errorf("Open did not load config: RecipesDir = %q", s.Config.RecipesDir)
	}
}
