package recipe

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"sort"
	"strings"
)

// This file holds the optional "enchantment" metadata (spec
// docs/specs/enchantment-metadata.md §3) layered onto a Recipe. Every type and
// field here is optional (omitempty): an existing recipe.yaml with none of
// these fields loads, saves, runs, and verifies exactly as before. The fields
// power cross-package recognition (spell_hash), provenance, a portability
// contract, scope/universality, and typed relations.

// Provenance records where an enchantment came from (spec §3). chant capture
// fills it best-effort; any field may be empty.
type Provenance struct {
	// Origin is the repo/package the enchantment was born in
	// (e.g. "github.com/fireharp/bitgn" or a local repo path).
	Origin string `yaml:"origin,omitempty"`
	// CapturedFrom is a chant run id / git commit / agent id, when known.
	CapturedFrom string `yaml:"captured_from,omitempty"`
	// CapturedAt is the RFC3339 capture timestamp.
	CapturedAt string `yaml:"captured_at,omitempty"`
	// Author is who/what captured it (e.g. "agent:capture").
	Author string `yaml:"author,omitempty"`
}

// VerifiedContext is one distinct context where the verifier passed (spec §3,
// §5). The set of these drives scope promotion (computed, never hand-set).
type VerifiedContext struct {
	// Context identifies the repo/package (e.g. "fireharp/bitgn").
	Context string `yaml:"context,omitempty"`
	// At is the RFC3339 timestamp the verifier passed there.
	At string `yaml:"at,omitempty"`
}

// InputContract describes what an enchantment's inputs must look like so a
// foreign package can decide whether its data fits (spec §3).
type InputContract struct {
	// SchemaFingerprint is a compact signature of the expected schema
	// (e.g. "cols:channel|revenue").
	SchemaFingerprint string `yaml:"schema_fingerprint,omitempty"`
	// RequiredColumnsAny mirrors WhenToUse.InputSignals.ColumnsAny: each inner
	// group is a set of acceptable aliases for one logical column.
	RequiredColumnsAny [][]string `yaml:"required_columns_any,omitempty"`
}

// Requires pins the runtime context the enchantment needs to move to another
// package (spec §3).
type Requires struct {
	// Runtime is e.g. "python: >=3.8".
	Runtime string `yaml:"runtime,omitempty"`
	// Packages maps package name to version constraint.
	Packages map[string]string `yaml:"packages,omitempty"`
	// Env lists required environment variables; empty ⇒ context-free.
	Env []string `yaml:"env,omitempty"`
}

// Portability answers "can this enchantment move to another package?" (spec §3).
type Portability struct {
	// Determinism is "deterministic" or "effectful".
	Determinism string `yaml:"determinism,omitempty"`
	// SideEffects lists observable side effects; empty ⇒ pure.
	SideEffects []string `yaml:"side_effects,omitempty"`
	// InputContract is what the inputs must look like.
	InputContract InputContract `yaml:"input_contract,omitempty"`
	// Requires is the runtime/packages/env the enchantment needs.
	Requires Requires `yaml:"requires,omitempty"`
}

// Relations are typed lineage edges reusing the coherence edge vocabulary
// (spec §3, §7). All are recipe-id (or resource) references.
type Relations struct {
	// Supersedes lists versions this one replaces.
	Supersedes []string `yaml:"supersedes,omitempty"`
	// Mirrors lists same-procedure enchantments in another language/package.
	Mirrors []string `yaml:"mirrors,omitempty"`
	// Generalizes lists narrower forms this one broadens.
	Generalizes []string `yaml:"generalizes,omitempty"`
	// Specializes lists broader forms this one narrows.
	Specializes []string `yaml:"specializes,omitempty"`
	// DependsOn lists data/config the enchantment depends on.
	DependsOn []string `yaml:"depends_on,omitempty"`
	// Implements lists the story/policy ids it fulfils.
	Implements []string `yaml:"implements,omitempty"`
}

// placeholderToken is the constant a {{var}} placeholder normalizes to so two
// enchantments that differ only in their placeholder names share a spell_hash.
const placeholderToken = "{{·}}"

// placeholderRE matches {{ ... }} template placeholders (the same form
// WhatToDo.Command uses).
var placeholderRE = regexp.MustCompile(`\{\{[^}]*\}\}`)

// whitespaceRE collapses runs of whitespace to a single space.
var whitespaceRE = regexp.MustCompile(`\s+`)

// normalizeCommand applies the spec §4 command normalization: placeholders are
// replaced with a constant token and whitespace is collapsed/trimmed. Two
// commands that differ only in placeholder names or incidental whitespace
// normalize identically.
func normalizeCommand(cmd string) string {
	cmd = placeholderRE.ReplaceAllString(cmd, placeholderToken)
	cmd = whitespaceRE.ReplaceAllString(cmd, " ")
	return strings.TrimSpace(cmd)
}

// ComputeSpellHash computes the portable, content-addressed identity of an
// enchantment (spec §4):
//
//	sha256(
//	  normalize(what_to_do.command)         // placeholders → {{·}}, whitespace-collapsed
//	  ⧺ canonical(entrypoint source)        // plain content hash for the MVP (see note)
//	  ⧺ sorted(input_contract.required_columns_any)
//	)
//
// returning the first 16 hex chars, matching the existing fingerprint helpers.
// Two enchantments with the same spell_hash are the same procedure even if
// their ids/descriptions differ.
//
// MVP deviation: the entrypoint contribution is a plain SHA-256 of the file's
// raw bytes (no comment-stripping / reformatting). The spec marks comment
// stripping a nice-to-have; a plain content hash is acceptable for the MVP and
// keeps the result deterministic and dependency-free. The required-columns
// input falls back to WhenToUse.InputSignals.ColumnsAny when
// Portability.InputContract.RequiredColumnsAny is unset, so capture-time
// signals contribute even before the portability contract is filled in.
func (r *Recipe) ComputeSpellHash() string {
	h := sha256.New()

	// 1. normalized command
	h.Write([]byte(normalizeCommand(r.WhatToDo.Command)))
	h.Write([]byte("\x00"))

	// 2. canonical entrypoint source (plain content hash, MVP)
	if path := r.EntrypointPath(); path != "" {
		if b, err := os.ReadFile(path); err == nil {
			sum := sha256.Sum256(b)
			h.Write(sum[:])
		}
	}
	h.Write([]byte("\x00"))

	// 3. sorted required_columns_any
	cols := r.Portability.InputContract.RequiredColumnsAny
	if len(cols) == 0 {
		cols = r.WhenToUse.InputSignals.ColumnsAny
	}
	for _, group := range sortedColumnGroups(cols) {
		h.Write([]byte(strings.Join(group, ",")))
		h.Write([]byte(";"))
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}

// sortedColumnGroups returns a deterministic ordering of a columns_any list:
// each inner alias group is sorted, then the groups are sorted. This makes the
// hash independent of the order columns were listed in. Input is not mutated.
func sortedColumnGroups(cols [][]string) [][]string {
	out := make([][]string, len(cols))
	for i, group := range cols {
		g := append([]string(nil), group...)
		sort.Strings(g)
		out[i] = g
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.Join(out[i], ",") < strings.Join(out[j], ",")
	})
	return out
}
