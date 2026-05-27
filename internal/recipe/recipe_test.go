package recipe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-recipe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A minimal card: leave id/version/kind/status unset to exercise defaults.
	card := "description: do a thing\nwhat_to_do:\n  command: echo hi\n"
	if err := os.WriteFile(filepath.Join(dir, CardFile), []byte(card), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.ID != "my-recipe" {
		t.Errorf("ID = %q, want directory name %q", r.ID, "my-recipe")
	}
	if r.Version != 1 {
		t.Errorf("Version = %d, want 1", r.Version)
	}
	if r.Kind != "executable_recipe" {
		t.Errorf("Kind = %q, want executable_recipe", r.Kind)
	}
	if r.Status != "active" {
		t.Errorf("Status = %q, want active", r.Status)
	}
	if r.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", r.Dir(), dir)
	}
	if r.IsStale() {
		t.Error("IsStale() = true, want false for active recipe")
	}
}

func TestLoadPreservesExplicitFields(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir-name")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	card := "id: explicit-id\nversion: 3\nkind: patch_recipe\nstatus: stale\ndescription: x\nwhat_to_do:\n  command: echo hi\n"
	if err := os.WriteFile(filepath.Join(dir, CardFile), []byte(card), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r.ID != "explicit-id" {
		t.Errorf("ID = %q, want explicit-id (should not be overridden by dir)", r.ID)
	}
	if r.Version != 3 {
		t.Errorf("Version = %d, want 3", r.Version)
	}
	if r.Kind != "patch_recipe" {
		t.Errorf("Kind = %q, want patch_recipe", r.Kind)
	}
	if !r.IsStale() {
		t.Error("IsStale() = false, want true for status=stale")
	}
}

func TestLoadMissingCardErrors(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("expected error loading a dir with no recipe.yaml")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "round-trip")
	orig := &Recipe{
		ID:          "round-trip",
		Version:     2,
		Kind:        "executable_recipe",
		Description: "round trip recipe",
		Status:      "active",
		WhenToUse: WhenToUse{
			TaskPatterns: []string{"do the thing", "do another thing"},
			Tags:         []string{"alpha", "beta"},
			InputSignals: InputSignals{
				Files:      []string{"*.csv"},
				ColumnsAny: [][]string{{"channel", "source"}},
			},
		},
		WhatToDo:     WhatToDo{Command: "echo {{name}}", Language: "bash"},
		Verification: Verification{Command: "test -f out.txt", ExpectedArtifacts: []string{"out.txt"}},
		Metrics:      Metrics{Runs: 4, Failures: 1},
	}
	orig.SetDir(dir)
	if err := orig.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if got.ID != orig.ID || got.Version != orig.Version || got.Description != orig.Description {
		t.Errorf("scalar fields not round-tripped: %+v", got)
	}
	if got.WhatToDo.Command != "echo {{name}}" {
		t.Errorf("Command = %q, want %q", got.WhatToDo.Command, "echo {{name}}")
	}
	if len(got.WhenToUse.TaskPatterns) != 2 || got.WhenToUse.TaskPatterns[0] != "do the thing" {
		t.Errorf("TaskPatterns not round-tripped: %v", got.WhenToUse.TaskPatterns)
	}
	if len(got.WhenToUse.InputSignals.ColumnsAny) != 1 || got.WhenToUse.InputSignals.ColumnsAny[0][1] != "source" {
		t.Errorf("ColumnsAny not round-tripped: %v", got.WhenToUse.InputSignals.ColumnsAny)
	}
	if got.Verification.Command != "test -f out.txt" {
		t.Errorf("Verification.Command = %q", got.Verification.Command)
	}
	if got.Metrics.Runs != 4 || got.Metrics.Failures != 1 {
		t.Errorf("Metrics not round-tripped: %+v", got.Metrics)
	}
}

func TestSaveWithoutDirErrors(t *testing.T) {
	r := &Recipe{ID: "no-dir"}
	if err := r.Save(); err == nil {
		t.Fatal("expected error saving a recipe with no dir set")
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Compute Revenue by Channel", "compute-revenue-by-channel"},
		{"  Trim   Spaces  ", "trim-spaces"},
		{"CSV → Normalize!!!", "csv-normalize"},
		{"already-a-slug", "already-a-slug"},
		{"UPPER_case_123", "upper-case-123"},
		{"---leading and trailing---", "leading-and-trailing"},
		{"multiple   spaces", "multiple-spaces"},
		{"", ""},
		{"!!!", ""},
		{"a", "a"},
		{"café crème", "caf-cr-me"}, // non-ASCII letters are non-alphanumeric here
	}
	for _, tt := range tests {
		if got := Slug(tt.in); got != tt.want {
			t.Errorf("Slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMetricsSuccessRate(t *testing.T) {
	tests := []struct {
		name     string
		runs     int
		failures int
		want     float64
	}{
		{"no runs gets benefit of the doubt", 0, 0, 1.0},
		{"negative-ish runs treated as no runs", -3, 0, 1.0},
		{"all success", 5, 0, 1.0},
		{"half fail", 4, 2, 0.5},
		{"all fail", 3, 3, 0.0},
		{"more failures than runs clamps to 0", 2, 5, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Metrics{Runs: tt.runs, Failures: tt.failures}
			if got := m.SuccessRate(); got != tt.want {
				t.Errorf("SuccessRate(runs=%d,fail=%d) = %v, want %v", tt.runs, tt.failures, got, tt.want)
			}
		})
	}
}

func TestRecordRun(t *testing.T) {
	r := &Recipe{ID: "rec"}

	r.RecordRun(true)
	if r.Metrics.Runs != 1 || r.Metrics.Failures != 0 {
		t.Errorf("after success: runs=%d failures=%d, want 1/0", r.Metrics.Runs, r.Metrics.Failures)
	}
	if r.Metrics.LastSuccessAt == "" {
		t.Error("LastSuccessAt not set after a successful run")
	}
	if r.Metrics.LastFailureAt != "" {
		t.Error("LastFailureAt should be empty after only a success")
	}

	r.RecordRun(false)
	if r.Metrics.Runs != 2 || r.Metrics.Failures != 1 {
		t.Errorf("after failure: runs=%d failures=%d, want 2/1", r.Metrics.Runs, r.Metrics.Failures)
	}
	if r.Metrics.LastFailureAt == "" {
		t.Error("LastFailureAt not set after a failed run")
	}
	if got := r.Metrics.SuccessRate(); got != 0.5 {
		t.Errorf("SuccessRate after 1/1 = %v, want 0.5", got)
	}
}

func TestComputeFingerprintsStability(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.py"), []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test_run.py"), []byte("assert True\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Recipe{ID: "fp", WhatToDo: WhatToDo{Entrypoint: "run.py"}}
	r.SetDir(dir)

	r.ComputeFingerprints()
	code1, verif1 := r.Fingerprints.RecipeCodeHash, r.Fingerprints.VerifierHash
	if code1 == "" {
		t.Error("RecipeCodeHash empty despite an existing entrypoint")
	}
	if verif1 == "" {
		t.Error("VerifierHash empty despite a matching test_*.py file")
	}

	// Stable: recomputing on unchanged files yields the same hashes.
	r.ComputeFingerprints()
	if r.Fingerprints.RecipeCodeHash != code1 || r.Fingerprints.VerifierHash != verif1 {
		t.Error("fingerprints not stable across recomputation on unchanged files")
	}

	// Changing the entrypoint content changes the code hash.
	if err := os.WriteFile(filepath.Join(dir, "run.py"), []byte("print('changed')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.ComputeFingerprints()
	if r.Fingerprints.RecipeCodeHash == code1 {
		t.Error("RecipeCodeHash did not change after entrypoint content changed")
	}
}

func TestComputeFingerprintsMissingFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Recipe{ID: "empty"} // no entrypoint, no verifier files
	r.SetDir(dir)
	r.ComputeFingerprints()
	if r.Fingerprints.RecipeCodeHash != "" {
		t.Errorf("RecipeCodeHash = %q, want empty for missing entrypoint", r.Fingerprints.RecipeCodeHash)
	}
	if r.Fingerprints.VerifierHash != "" {
		t.Errorf("VerifierHash = %q, want empty for no verifier files", r.Fingerprints.VerifierHash)
	}
}

func TestEntrypointPath(t *testing.T) {
	r := &Recipe{}
	r.SetDir("/tmp/x")
	if r.EntrypointPath() != "" {
		t.Errorf("EntrypointPath with no entrypoint = %q, want empty", r.EntrypointPath())
	}
	r.WhatToDo.Entrypoint = "run.py"
	if got := r.EntrypointPath(); got != filepath.Join("/tmp/x", "run.py") {
		t.Errorf("EntrypointPath = %q", got)
	}
}

func TestMarkStale(t *testing.T) {
	r := &Recipe{Status: "active"}
	r.MarkStale()
	if !r.IsStale() {
		t.Error("MarkStale did not set status to stale")
	}
}
