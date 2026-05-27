package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	d := Default()
	if d.Version != 1 {
		t.Errorf("Version = %d, want 1", d.Version)
	}
	if d.RecipesDir != "recipes" {
		t.Errorf("RecipesDir = %q, want %q", d.RecipesDir, "recipes")
	}
	if d.Retrieval.WeightLexical != 0.5 {
		t.Errorf("WeightLexical = %v, want 0.5", d.Retrieval.WeightLexical)
	}
	if d.Retrieval.WeightTags != 0.3 {
		t.Errorf("WeightTags = %v, want 0.3", d.Retrieval.WeightTags)
	}
	if d.Retrieval.WeightSuccessRate != 0.2 {
		t.Errorf("WeightSuccessRate = %v, want 0.2", d.Retrieval.WeightSuccessRate)
	}
	if d.Retrieval.Threshold != 0.25 {
		t.Errorf("Threshold = %v, want 0.25", d.Retrieval.Threshold)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir() // no chant.yml present
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error for missing file: %v", err)
	}
	if cfg != Default() {
		t.Errorf("Load(missing) = %+v, want defaults %+v", cfg, Default())
	}
}

func TestLoadPartialReappliesDefaults(t *testing.T) {
	dir := t.TempDir()
	// Only set the threshold; everything else should fall back to defaults.
	yml := "version: 1\nretrieval:\n  threshold: 0.4\n"
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RecipesDir != "recipes" {
		t.Errorf("RecipesDir = %q, want default %q", cfg.RecipesDir, "recipes")
	}
	// Weights were all zero in the file → defaults re-applied as a group.
	if cfg.Retrieval.WeightLexical != 0.5 || cfg.Retrieval.WeightTags != 0.3 || cfg.Retrieval.WeightSuccessRate != 0.2 {
		t.Errorf("weights = %+v, want defaults", cfg.Retrieval)
	}
	// Threshold was explicitly set and must be preserved.
	if cfg.Retrieval.Threshold != 0.4 {
		t.Errorf("Threshold = %v, want 0.4 (explicit value preserved)", cfg.Retrieval.Threshold)
	}
}

func TestLoadFullOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	yml := "" +
		"version: 2\n" +
		"recipes_dir: cookbook\n" +
		"retrieval:\n" +
		"  weight_lexical: 0.7\n" +
		"  weight_tags: 0.2\n" +
		"  weight_success_rate: 0.1\n" +
		"  threshold: 0.6\n"
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RecipesDir != "cookbook" {
		t.Errorf("RecipesDir = %q, want %q", cfg.RecipesDir, "cookbook")
	}
	if cfg.Retrieval.WeightLexical != 0.7 || cfg.Retrieval.Threshold != 0.6 {
		t.Errorf("retrieval = %+v, want explicit values", cfg.Retrieval)
	}
}

func TestLoadInvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("::: not yaml :::\n  - ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}
