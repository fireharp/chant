// Package store is chant's filesystem layout: the committed recipe library
// under recipes/ and the gitignored runtime state under .chant/ (index + run
// logs). It mirrors coherence's split of committed config vs .coherence/
// runtime state.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fireharp/chant/internal/config"
	"github.com/fireharp/chant/internal/recipe"
)

// StateDir is the gitignored runtime directory.
const StateDir = ".chant"

// Store binds a repo root to its config and recipe library.
type Store struct {
	Root   string
	Config config.Config
}

// Open discovers the repo root from start and loads chant.yml.
func Open(start string) (*Store, error) {
	root, err := FindRoot(start)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	return &Store{Root: root, Config: cfg}, nil
}

// FindRoot walks up from start looking for a .git directory or a chant.yml.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if isRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Hit filesystem root: fall back to the original start so chant
			// still works in a non-git scratch directory.
			abs, _ := filepath.Abs(start)
			return abs, nil
		}
		dir = parent
	}
}

func isRoot(dir string) bool {
	for _, marker := range []string{".git", config.FileName} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

// RecipesDir returns the absolute path to the recipe library.
func (s *Store) RecipesDir() string {
	return filepath.Join(s.Root, s.Config.RecipesDir)
}

// StatePath returns an absolute path inside the .chant runtime directory.
func (s *Store) StatePath(parts ...string) string {
	return filepath.Join(append([]string{s.Root, StateDir}, parts...)...)
}

// LoadAll loads every recipe under the recipe library, sorted by id.
func (s *Store) LoadAll() ([]*recipe.Recipe, error) {
	dir := s.RecipesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*recipe.Recipe
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		recDir := filepath.Join(dir, e.Name())
		if _, err := os.Stat(filepath.Join(recDir, recipe.CardFile)); err != nil {
			continue // not a recipe dir
		}
		r, err := recipe.Load(recDir)
		if err != nil {
			return nil, fmt.Errorf("load recipe %q: %w", e.Name(), err)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Get loads a single recipe by id.
func (s *Store) Get(id string) (*recipe.Recipe, error) {
	recDir := filepath.Join(s.RecipesDir(), id)
	if _, err := os.Stat(filepath.Join(recDir, recipe.CardFile)); err != nil {
		return nil, fmt.Errorf("recipe %q not found", id)
	}
	return recipe.Load(recDir)
}

// DirFor returns the directory a recipe with the given id should live in.
func (s *Store) DirFor(id string) string {
	return filepath.Join(s.RecipesDir(), id)
}

// Exists reports whether a recipe id already has a card on disk.
func (s *Store) Exists(id string) bool {
	_, err := os.Stat(filepath.Join(s.DirFor(id), recipe.CardFile))
	return err == nil
}

// Index is the rebuilt-on-demand retrieval index written to .chant/index.json.
type Index struct {
	GeneratedAt string         `json:"generated_at"`
	Count       int            `json:"count"`
	Recipes     []IndexEntry   `json:"recipes"`
}

// IndexEntry is a flattened recipe summary for fast listing.
type IndexEntry struct {
	ID          string   `json:"id"`
	Version     int      `json:"version"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags,omitempty"`
	Runs        int      `json:"runs"`
	Failures    int      `json:"failures"`
	SuccessRate float64  `json:"success_rate"`
}

// WriteIndex rebuilds .chant/index.json from the recipe library.
func (s *Store) WriteIndex() (*Index, error) {
	recs, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	idx := &Index{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Count:       len(recs),
	}
	for _, r := range recs {
		idx.Recipes = append(idx.Recipes, IndexEntry{
			ID:          r.ID,
			Version:     r.Version,
			Description: r.Description,
			Status:      r.Status,
			Tags:        r.WhenToUse.Tags,
			Runs:        r.Metrics.Runs,
			Failures:    r.Metrics.Failures,
			SuccessRate: r.Metrics.SuccessRate(),
		})
	}
	if err := os.MkdirAll(s.StatePath(), 0o755); err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(idx, "", "  ")
	if err := os.WriteFile(s.StatePath("index.json"), b, 0o644); err != nil {
		return nil, err
	}
	return idx, nil
}

// RunRecord is one execution logged under .chant/runs/.
type RunRecord struct {
	RecipeID    string            `json:"recipe_id"`
	Version     int               `json:"version"`
	StartedAt   string            `json:"started_at"`
	DurationMS  int64             `json:"duration_ms"`
	Command     string            `json:"command"`
	Inputs      map[string]string `json:"inputs,omitempty"`
	ExitCode    int               `json:"exit_code"`
	VerifierRan bool              `json:"verifier_ran"`
	Verified    bool              `json:"verified"`
	Stdout      string            `json:"stdout,omitempty"`
	Stderr      string            `json:"stderr,omitempty"`
}

// WriteRun persists a run record under .chant/runs/<timestamp>/run.json.
func (s *Store) WriteRun(rec RunRecord) (string, error) {
	stamp := time.Now().UTC().Format("20060102T150405.000Z")
	dir := s.StatePath("runs", stamp)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	b, _ := json.MarshalIndent(rec, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "run.json"), b, 0o644); err != nil {
		return "", err
	}
	return dir, nil
}

// ErrNoRecipes signals an empty library.
var ErrNoRecipes = errors.New("no recipes in library")
