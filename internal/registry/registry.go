// Package registry is chant's per-machine, cross-package enchantment index
// (spec docs/specs/enchantment-metadata.md §6). Where the in-repo
// .chant/index.json lists one project's recipes, the registry aggregates
// enchantments across all of a user's projects, keyed by spell_hash, so a
// recipe captured in project A can be discovered (and imported, then verified)
// in project B.
//
// The registry is deliberately dependency-free (stdlib + the existing
// retrieve/recipe packages) and deterministic: the same set of entries always
// serializes the same way, and Search is reproducible. It never establishes
// trust — a foreign entry is only a *candidate*; trust comes from running its
// verifier locally (verifier-first).
package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fireharp/chant/internal/config"
	"github.com/fireharp/chant/internal/recipe"
	"github.com/fireharp/chant/internal/retrieve"
)

// EnvPath is the environment variable that overrides the registry location.
// Tests set this to a temp dir so they never touch the real $HOME.
const EnvPath = "CHANT_REGISTRY"

// Entry is one enchantment recorded in the registry, flattened for fast,
// cross-package discovery. It is keyed/deduped by SpellHash.
type Entry struct {
	SpellHash    string   `json:"spell_hash"`
	ID           string   `json:"id"`
	Version      int      `json:"version"`
	Description  string   `json:"description,omitempty"`
	Origin       string   `json:"origin,omitempty"`
	Scope        string   `json:"scope,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	TaskPatterns []string `json:"task_patterns,omitempty"`
	// RecipePath is the ABSOLUTE path to the recipe directory, so `chant
	// import` can copy it into the local library.
	RecipePath string `json:"recipe_path"`
	// HasVerifier records whether the source recipe carries a verifier. A
	// foreign recipe with no verifier can never be trusted on import.
	HasVerifier bool `json:"has_verifier"`
	// UpdatedAt is the RFC3339 timestamp this entry was last upserted; it
	// implements "newest wins" on a spell_hash collision.
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Registry is the on-disk per-machine index.
type Registry struct {
	GeneratedAt string  `json:"generated_at"`
	Entries     []Entry `json:"entries"`

	// path is where Save writes; not serialized.
	path string `json:"-"`
}

// DefaultPath resolves the registry file path: $CHANT_REGISTRY when set,
// otherwise $HOME/.chant/registry/index.json. It never reads HOME directly when
// the env override is present, keeping tests hermetic.
func DefaultPath() string {
	if p := os.Getenv(EnvPath); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Last-resort fallback so a HOME-less environment still has a path.
		home = "."
	}
	return filepath.Join(home, ".chant", "registry", "index.json")
}

// Load reads the registry at path. A missing file is not an error: it yields an
// empty registry bound to that path, ready to Upsert + Save.
func Load(path string) (*Registry, error) {
	reg := &Registry{path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, reg); err != nil {
		return nil, err
	}
	reg.path = path
	return reg, nil
}

// Path returns the file path the registry is bound to.
func (r *Registry) Path() string { return r.path }

// Upsert merges entries into the registry, replacing any existing entry with
// the same spell_hash (newest wins). Entries with an empty spell_hash are
// skipped — without portable identity there is nothing to dedupe on. Returns
// the number of entries inserted or replaced.
func (r *Registry) Upsert(entries ...Entry) int {
	byHash := make(map[string]int, len(r.Entries))
	for i, e := range r.Entries {
		byHash[e.SpellHash] = i
	}
	n := 0
	for _, e := range entries {
		if e.SpellHash == "" {
			continue
		}
		if e.UpdatedAt == "" {
			e.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if i, ok := byHash[e.SpellHash]; ok {
			r.Entries[i] = e // newest wins
		} else {
			byHash[e.SpellHash] = len(r.Entries)
			r.Entries = append(r.Entries, e)
		}
		n++
	}
	r.sortEntries()
	return n
}

// sortEntries keeps the registry deterministic on disk: ordered by spell_hash.
func (r *Registry) sortEntries() {
	sort.Slice(r.Entries, func(i, j int) bool {
		return r.Entries[i].SpellHash < r.Entries[j].SpellHash
	})
}

// Save writes the registry to its path, creating parent directories as needed.
func (r *Registry) Save() error {
	if r.path == "" {
		r.path = DefaultPath()
	}
	r.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	r.sortEntries()
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, b, 0o644)
}

// Find returns the entry whose spell_hash or id equals key, preferring an exact
// spell_hash match. The second return is false when nothing matches.
func (r *Registry) Find(key string) (Entry, bool) {
	for _, e := range r.Entries {
		if e.SpellHash == key {
			return e, true
		}
	}
	for _, e := range r.Entries {
		if e.ID == key {
			return e, true
		}
	}
	return Entry{}, false
}

// Result is one ranked registry entry from Search.
type Result struct {
	Entry Entry
	Score float64
}

// Search ranks registry entries against the query, reusing the retrieve scorer
// against each entry's task_patterns + description + tags. Only entries scoring
// at or above cfg.Threshold are returned, sorted by descending score (ties
// broken by spell_hash for determinism). The recipe scorer needs a *recipe, so
// each entry is projected onto a minimal synthetic recipe carrying just the
// retrieval-relevant text.
func (r *Registry) Search(q retrieve.Query, cfg config.Retrieval) []Result {
	recs := make([]*recipe.Recipe, 0, len(r.Entries))
	byID := make(map[string]Entry, len(r.Entries))
	for _, e := range r.Entries {
		// Use the spell_hash as the synthetic recipe id so ranking ties break
		// deterministically and we can map matches back to entries.
		rec := &recipe.Recipe{
			ID:          e.SpellHash,
			Version:     e.Version,
			Description: e.Description,
			WhenToUse: recipe.WhenToUse{
				TaskPatterns: e.TaskPatterns,
				Tags:         e.Tags,
			},
		}
		recs = append(recs, rec)
		byID[e.SpellHash] = e
	}
	var out []Result
	for _, m := range retrieve.Suggest(recs, q, cfg) {
		if e, ok := byID[m.Recipe.ID]; ok {
			out = append(out, Result{Entry: e, Score: m.Score})
		}
	}
	return out
}

// EntryFromRecipe projects a loaded recipe into a registry Entry. recipePath
// should be the absolute recipe directory. Returns the entry and false when the
// recipe has no spell_hash (nothing to register on).
func EntryFromRecipe(r *recipe.Recipe, recipePath string) (Entry, bool) {
	if r.SpellHash == "" {
		return Entry{}, false
	}
	return Entry{
		SpellHash:    r.SpellHash,
		ID:           r.ID,
		Version:      r.Version,
		Description:  r.Description,
		Origin:       r.Provenance.Origin,
		Scope:        r.Scope,
		Tags:         r.WhenToUse.Tags,
		TaskPatterns: r.WhenToUse.TaskPatterns,
		RecipePath:   recipePath,
		HasVerifier:  r.Verification.Command != "" || len(r.Verification.ExpectedArtifacts) > 0,
	}, true
}
