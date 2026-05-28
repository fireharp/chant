// Package recipe defines the chant Recipe: a versioned, verified procedure
// card produced from successful agent work. A recipe caches the tested *way*
// of solving a recurring task, not a cached answer.
package recipe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// CardFile is the canonical recipe descriptor filename inside a recipe dir.
const CardFile = "recipe.yaml"

// Recipe is the on-disk recipe card (recipes/<id>/recipe.yaml).
//
// The field groups mirror the lifecycle: when_to_use answers "should this
// recipe even be considered?", what_to_do is the executable procedure,
// verification is how we decide the result is trustworthy, invalidation is
// when to stop trusting it, and metrics records the reuse track record.
type Recipe struct {
	ID          string `yaml:"id"`
	Version     int    `yaml:"version"`
	Kind        string `yaml:"kind,omitempty"` // executable_recipe (default), patch_recipe, workflow_recipe
	Description string `yaml:"description"`

	WhenToUse    WhenToUse    `yaml:"when_to_use"`
	WhatToDo     WhatToDo     `yaml:"what_to_do"`
	Verification Verification `yaml:"verification"`
	Invalidation Invalidation `yaml:"invalidation,omitempty"`
	Dependencies Dependencies `yaml:"dependencies,omitempty"`
	Fingerprints Fingerprints `yaml:"fingerprints,omitempty"`
	Examples     []Example    `yaml:"examples,omitempty"`
	Metrics      Metrics      `yaml:"metrics,omitempty"`

	// Status is "active" (default) or "stale". Stale recipes are still
	// retrievable but flagged: a hit is a candidate, never trusted, until a
	// verifier re-confirms it.
	Status string `yaml:"status,omitempty"`

	// ── enchantment metadata (optional; see enchantment.go and
	// docs/specs/enchantment-metadata.md §3) ──────────────────────────────
	// Every field below is optional: a recipe.yaml carrying none of them
	// behaves exactly as before. They power cross-package recognition,
	// provenance, a portability contract, scope/universality, and lineage.

	// SpellHash is the content-addressed identity of the procedure (spec §4).
	// Same procedure ⇒ same hash across repos. Filled by `chant capture`.
	SpellHash string `yaml:"spell_hash,omitempty"`
	// LineageID is a stable family id shared across versions/forks.
	LineageID string `yaml:"lineage_id,omitempty"`
	// Provenance records where the enchantment came from.
	Provenance Provenance `yaml:"provenance,omitempty"`
	// Scope is the maturity channel: project | domain | universal (spec §5).
	// Promotion is earned, never declared — driven by VerifiedIn evidence and
	// Domains via ComputeScope (see scope.go). A recipe with no Domains is
	// capped at project regardless of VerifiedIn count: without a domain
	// label the "same domain" / "spans ≥2 domains" rules are undefined.
	Scope string `yaml:"scope,omitempty"`
	// Domains are discovery labels broader than tags. They are also the
	// clustering signal for scope promotion: without ≥1 domain tag a recipe
	// can never reach `domain` or `universal` scope (see ComputeScope).
	Domains []string `yaml:"domains,omitempty"`
	// VerifiedIn lists distinct contexts where the verifier passed (drives
	// scope promotion; computed, not hand-set). chant verify appends to this
	// list on a passing verifier; ComputeScope reads it.
	VerifiedIn []VerifiedContext `yaml:"verified_in,omitempty"`
	// Portability is the contract for moving the enchantment to another package.
	Portability Portability `yaml:"portability,omitempty"`
	// Relations are typed lineage edges (coherence edge vocabulary).
	Relations Relations `yaml:"relations,omitempty"`

	// dir is the absolute directory this recipe was loaded from. Not serialized.
	dir string `yaml:"-"`
}

// WhenToUse is the applicability gate used during retrieval.
type WhenToUse struct {
	// TaskPatterns are natural-language descriptions of the tasks this recipe
	// solves. Matched lexically (and optionally semantically) against a query.
	TaskPatterns []string `yaml:"task_patterns,omitempty"`
	// Tags are free-form labels for structured filtering.
	Tags []string `yaml:"tags,omitempty"`
	// InputSignals are structural conditions on the inputs.
	InputSignals InputSignals `yaml:"input_signals,omitempty"`
}

// InputSignals are structural preconditions on the task's inputs.
type InputSignals struct {
	// Files are globs the input file set should match (e.g. "*.csv").
	Files []string `yaml:"files,omitempty"`
	// ColumnsAny is a list of alternative column-name groups. Each inner
	// group is a set of acceptable aliases for one logical column.
	ColumnsAny [][]string `yaml:"columns_any,omitempty"`
}

// WhatToDo is the executable procedure.
type WhatToDo struct {
	// Entrypoint is the script file inside the recipe dir (e.g. "run.py").
	Entrypoint string `yaml:"entrypoint,omitempty"`
	// Command is the templated shell command to run the recipe. {{var}}
	// placeholders are substituted from `chant run --input k=v` values.
	Command string `yaml:"command"`
	// Language is informational (python, go, node, bash, ...).
	Language string `yaml:"language,omitempty"`
}

// Verification is how chant decides a reuse result is trustworthy.
type Verification struct {
	// Command runs the verifier. Exit 0 == passed.
	Command string `yaml:"command,omitempty"`
	// ExpectedArtifacts are paths that must exist after a successful run.
	ExpectedArtifacts []string `yaml:"expected_artifacts,omitempty"`
}

// Invalidation captures when a recipe should be marked stale.
type Invalidation struct {
	IfTestsFail        bool `yaml:"if_tests_fail,omitempty"`
	IfColumnsMissing   bool `yaml:"if_columns_missing,omitempty"`
	IfDependencyChange bool `yaml:"if_dependency_changed,omitempty"`
}

// Dependencies pins the runtime the recipe was verified against.
type Dependencies struct {
	Runtime  string            `yaml:"runtime,omitempty"` // e.g. "python: >=3.11"
	Packages map[string]string `yaml:"packages,omitempty"`
}

// Fingerprints are content hashes used to detect drift in the recipe itself.
type Fingerprints struct {
	RecipeCodeHash    string `yaml:"recipe_code_hash,omitempty"`
	VerifierHash      string `yaml:"verifier_hash,omitempty"`
	SchemaFingerprint string `yaml:"schema_fingerprint,omitempty"`
}

// Example is a recorded input/output pair.
type Example struct {
	Input  string `yaml:"input,omitempty"`
	Output string `yaml:"output,omitempty"`
}

// Metrics is the reuse track record feeding retrieval ranking.
type Metrics struct {
	Runs          int    `yaml:"runs,omitempty"`
	Failures      int    `yaml:"failures,omitempty"`
	LastSuccessAt string `yaml:"last_success_at,omitempty"`
	LastFailureAt string `yaml:"last_failure_at,omitempty"`
}

// SuccessRate returns the fraction of runs that succeeded. A recipe with no
// recorded runs is given the benefit of the doubt (1.0) so a freshly captured
// recipe is not penalized before it has a track record.
func (m Metrics) SuccessRate() float64 {
	if m.Runs <= 0 {
		return 1.0
	}
	ok := m.Runs - m.Failures
	if ok < 0 {
		ok = 0
	}
	return float64(ok) / float64(m.Runs)
}

// Dir returns the directory the recipe was loaded from.
func (r *Recipe) Dir() string { return r.dir }

// IsStale reports whether the recipe is flagged stale.
func (r *Recipe) IsStale() bool { return r.Status == "stale" }

// EntrypointPath returns the absolute path to the recipe's entrypoint script.
func (r *Recipe) EntrypointPath() string {
	if r.WhatToDo.Entrypoint == "" {
		return ""
	}
	return filepath.Join(r.dir, r.WhatToDo.Entrypoint)
}

// Load reads a recipe.yaml from a recipe directory.
func Load(dir string) (*Recipe, error) {
	path := filepath.Join(dir, CardFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read recipe card: %w", err)
	}
	var r Recipe
	if err := yaml.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if r.ID == "" {
		// Default the id to the directory name when unset.
		r.ID = filepath.Base(dir)
	}
	if r.Version == 0 {
		r.Version = 1
	}
	if r.Kind == "" {
		r.Kind = "executable_recipe"
	}
	if r.Status == "" {
		r.Status = "active"
	}
	r.dir = dir
	return &r, nil
}

// Save writes the recipe card back to its directory, creating it if needed.
func (r *Recipe) Save() error {
	if r.dir == "" {
		return fmt.Errorf("recipe %q has no directory set", r.ID)
	}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	header := "# chant recipe card — a verified, reusable procedure.\n" +
		"# Edit by hand or let `chant capture` regenerate it.\n"
	return os.WriteFile(filepath.Join(r.dir, CardFile), append([]byte(header), b...), 0o644)
}

// SetDir assigns the directory a recipe will be saved to.
func (r *Recipe) SetDir(dir string) { r.dir = dir }

// ComputeFingerprints (re)computes content hashes for the recipe's code and
// verifier files. Missing files hash to the empty marker.
func (r *Recipe) ComputeFingerprints() {
	r.Fingerprints.RecipeCodeHash = hashFile(r.EntrypointPath())
	// The verifier hash covers any explicit verifier file referenced by the
	// command; for the MVP we hash the entrypoint dir's *_test / test_* files.
	r.Fingerprints.VerifierHash = hashGlob(r.dir, verifierGlobs)
}

var verifierGlobs = []string{"test_*.py", "*_test.py", "*_test.go", "*.test.*", "verifier.*"}

func hashFile(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

func hashGlob(dir string, globs []string) string {
	var paths []string
	for _, g := range globs {
		matches, _ := filepath.Glob(filepath.Join(dir, g))
		paths = append(paths, matches...)
	}
	if len(paths) == 0 {
		return ""
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		h.Write([]byte(filepath.Base(p)))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// RecordRun updates the metrics after an execution. ok=true marks success.
func (r *Recipe) RecordRun(ok bool) {
	r.Metrics.Runs++
	now := time.Now().UTC().Format(time.RFC3339)
	if ok {
		r.Metrics.LastSuccessAt = now
	} else {
		r.Metrics.Failures++
		r.Metrics.LastFailureAt = now
	}
}

// MarkStale flags the recipe stale.
func (r *Recipe) MarkStale() { r.Status = "stale" }

// Slug normalizes a free-form string into a recipe-id-safe slug.
func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteRune(c)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
