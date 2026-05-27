package recipe

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSpellHashDeterministic: the same recipe hashes the same across calls.
func TestSpellHashDeterministic(t *testing.T) {
	r := &Recipe{
		ID:       "rev",
		WhatToDo: WhatToDo{Command: "python3 run.py {{input}}"},
		Portability: Portability{
			InputContract: InputContract{
				RequiredColumnsAny: [][]string{{"channel", "source"}, {"revenue", "amount"}},
			},
		},
	}
	h1 := r.ComputeSpellHash()
	h2 := r.ComputeSpellHash()
	if h1 == "" {
		t.Fatal("SpellHash returned empty string")
	}
	if len(h1) != 16 {
		t.Errorf("SpellHash length = %d, want 16 (short hex prefix)", len(h1))
	}
	if h1 != h2 {
		t.Errorf("SpellHash not deterministic: %q != %q", h1, h2)
	}
}

// TestSpellHashStableAcrossEquivalentRecipes: two recipes with different ids,
// descriptions, placeholder names, and column ordering — but the same
// underlying procedure — share a spell_hash (spec §4).
func TestSpellHashStableAcrossEquivalentRecipes(t *testing.T) {
	a := &Recipe{
		ID:          "csv-revenue-a",
		Description: "compute revenue",
		WhatToDo:    WhatToDo{Command: "python3 run.py {{input}}"},
		Portability: Portability{
			InputContract: InputContract{
				RequiredColumnsAny: [][]string{{"channel", "source"}, {"revenue", "amount"}},
			},
		},
	}
	b := &Recipe{
		ID:          "csv-revenue-b",           // different id
		Description: "totally different words", // different description
		// different placeholder name + extra incidental whitespace
		WhatToDo: WhatToDo{Command: "python3   run.py   {{file}}"},
		Portability: Portability{
			InputContract: InputContract{
				// same logical columns, listed in a different order
				RequiredColumnsAny: [][]string{{"amount", "revenue"}, {"source", "channel"}},
			},
		},
	}
	if a.ComputeSpellHash() != b.ComputeSpellHash() {
		t.Errorf("equivalent procedures hashed differently:\n a=%s\n b=%s", a.ComputeSpellHash(), b.ComputeSpellHash())
	}
}

// TestSpellHashColumnsFallBackToInputSignals: when the portability input
// contract is empty, capture-time WhenToUse.InputSignals.ColumnsAny feeds the
// hash, so the two sources are interchangeable.
func TestSpellHashColumnsFallBackToInputSignals(t *testing.T) {
	viaContract := &Recipe{
		WhatToDo: WhatToDo{Command: "go run main.go {{in}}"},
		Portability: Portability{
			InputContract: InputContract{RequiredColumnsAny: [][]string{{"a", "b"}}},
		},
	}
	viaSignals := &Recipe{
		WhatToDo:  WhatToDo{Command: "go run main.go {{in}}"},
		WhenToUse: WhenToUse{InputSignals: InputSignals{ColumnsAny: [][]string{{"a", "b"}}}},
	}
	if viaContract.ComputeSpellHash() != viaSignals.ComputeSpellHash() {
		t.Errorf("columns_any source should not affect hash: contract=%s signals=%s",
			viaContract.ComputeSpellHash(), viaSignals.ComputeSpellHash())
	}
}

// TestSpellHashDiffersOnCommand: a different command ⇒ a different hash.
func TestSpellHashDiffersOnCommand(t *testing.T) {
	a := &Recipe{WhatToDo: WhatToDo{Command: "python3 run.py {{input}}"}}
	b := &Recipe{WhatToDo: WhatToDo{Command: "node run.js {{input}}"}}
	if a.ComputeSpellHash() == b.ComputeSpellHash() {
		t.Errorf("different commands hashed the same: %s", a.ComputeSpellHash())
	}
}

// TestSpellHashDiffersOnColumns: different required columns ⇒ different hash.
func TestSpellHashDiffersOnColumns(t *testing.T) {
	a := &Recipe{
		WhatToDo:    WhatToDo{Command: "x"},
		Portability: Portability{InputContract: InputContract{RequiredColumnsAny: [][]string{{"channel"}}}},
	}
	b := &Recipe{
		WhatToDo:    WhatToDo{Command: "x"},
		Portability: Portability{InputContract: InputContract{RequiredColumnsAny: [][]string{{"region"}}}},
	}
	if a.ComputeSpellHash() == b.ComputeSpellHash() {
		t.Errorf("different columns hashed the same: %s", a.ComputeSpellHash())
	}
}

// TestSpellHashIncludesEntrypointSource: changing the entrypoint file content
// changes the hash (the entrypoint source contributes per spec §4).
func TestSpellHashIncludesEntrypointSource(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spell")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.py"), []byte("print('v1')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Recipe{WhatToDo: WhatToDo{Entrypoint: "run.py", Command: "python3 run.py {{in}}"}}
	r.SetDir(dir)
	before := r.ComputeSpellHash()

	if err := os.WriteFile(filepath.Join(dir, "run.py"), []byte("print('v2-changed')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if after := r.ComputeSpellHash(); after == before {
		t.Errorf("SpellHash did not change after entrypoint source changed: %s", before)
	}
}

// TestEnchantmentMetadataRoundTrip: the new optional fields survive a
// Save → Load cycle and a recipe carrying none of them still loads cleanly.
func TestEnchantmentMetadataRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ench")
	orig := &Recipe{
		ID:          "ench",
		Version:     1,
		Kind:        "executable_recipe",
		Description: "enchanted",
		Status:      "active",
		WhatToDo:    WhatToDo{Command: "echo {{x}}"},
		SpellHash:   "abc123def4567890",
		LineageID:   "ench-family",
		Provenance: Provenance{
			Origin:     "github.com/fireharp/chant",
			CapturedAt: "2026-05-27T21:09:37Z",
			Author:     "agent:capture",
		},
		Scope:   "project",
		Domains: []string{"csv", "ecommerce"},
		VerifiedIn: []VerifiedContext{
			{Context: "fireharp/bitgn", At: "2026-05-27T00:00:00Z"},
		},
		Portability: Portability{
			Determinism: "deterministic",
			SideEffects: []string{},
			InputContract: InputContract{
				SchemaFingerprint:  "cols:channel|revenue",
				RequiredColumnsAny: [][]string{{"channel", "source"}},
			},
			Requires: Requires{Runtime: "python: >=3.8"},
		},
		Relations: Relations{
			Supersedes: []string{"ench@1"},
			Mirrors:    []string{"ench-go"},
			Implements: []string{"US-014"},
		},
	}
	orig.SetDir(dir)
	if err := orig.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SpellHash != orig.SpellHash {
		t.Errorf("SpellHash not round-tripped: %q", got.SpellHash)
	}
	if got.LineageID != "ench-family" {
		t.Errorf("LineageID not round-tripped: %q", got.LineageID)
	}
	if got.Provenance.Origin != orig.Provenance.Origin || got.Provenance.Author != orig.Provenance.Author {
		t.Errorf("Provenance not round-tripped: %+v", got.Provenance)
	}
	if got.Scope != "project" {
		t.Errorf("Scope not round-tripped: %q", got.Scope)
	}
	if len(got.Domains) != 2 || got.Domains[1] != "ecommerce" {
		t.Errorf("Domains not round-tripped: %v", got.Domains)
	}
	if len(got.VerifiedIn) != 1 || got.VerifiedIn[0].Context != "fireharp/bitgn" {
		t.Errorf("VerifiedIn not round-tripped: %v", got.VerifiedIn)
	}
	if got.Portability.Determinism != "deterministic" ||
		got.Portability.InputContract.SchemaFingerprint != "cols:channel|revenue" ||
		got.Portability.Requires.Runtime != "python: >=3.8" {
		t.Errorf("Portability not round-tripped: %+v", got.Portability)
	}
	if len(got.Portability.InputContract.RequiredColumnsAny) != 1 ||
		got.Portability.InputContract.RequiredColumnsAny[0][0] != "channel" {
		t.Errorf("RequiredColumnsAny not round-tripped: %v", got.Portability.InputContract.RequiredColumnsAny)
	}
	if len(got.Relations.Supersedes) != 1 || got.Relations.Implements[0] != "US-014" {
		t.Errorf("Relations not round-tripped: %+v", got.Relations)
	}
}

// TestLoadWithoutMetadataIsClean: a recipe.yaml with no enchantment metadata
// loads with empty zero-value metadata (backward compatibility, spec §2/§8).
func TestLoadWithoutMetadataIsClean(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "legacy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	card := "id: legacy\nversion: 1\ndescription: old recipe\nwhat_to_do:\n  command: echo hi\n"
	if err := os.WriteFile(filepath.Join(dir, CardFile), []byte(card), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load legacy recipe: %v", err)
	}
	if r.SpellHash != "" || r.Scope != "" || r.LineageID != "" {
		t.Errorf("legacy recipe gained metadata on load: hash=%q scope=%q lineage=%q",
			r.SpellHash, r.Scope, r.LineageID)
	}
	if len(r.Domains) != 0 || len(r.VerifiedIn) != 0 {
		t.Error("legacy recipe gained domains/verified_in on load")
	}
	// And it can still compute a spell_hash on demand.
	if r.ComputeSpellHash() == "" {
		t.Error("SpellHash() empty for a legacy recipe with a command")
	}
}
