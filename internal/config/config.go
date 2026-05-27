// Package config loads chant.yml — the committed configuration that controls
// where recipes live and how retrieval ranks them.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FileName is the default config filename at the repo root.
const FileName = "chant.yml"

// Config is the chant.yml shape.
type Config struct {
	Version int `yaml:"version"`

	// RecipesDir is the committed recipe library, relative to repo root.
	RecipesDir string `yaml:"recipes_dir"`

	// Retrieval tunes the hybrid scorer.
	Retrieval Retrieval `yaml:"retrieval"`
}

// Retrieval holds the scoring weights and the match threshold. The weights
// encode chant's ranking thesis: a recipe is more reusable when its task text
// matches, its structural signals match, and it has a good verifier track
// record. A hit above Threshold is a *candidate* — never trusted until a
// verifier re-confirms it.
type Retrieval struct {
	WeightLexical     float64 `yaml:"weight_lexical"`
	WeightTags        float64 `yaml:"weight_tags"`
	WeightSuccessRate float64 `yaml:"weight_success_rate"`
	// Threshold is the minimum score for `suggest` to report a match.
	Threshold float64 `yaml:"threshold"`
}

// Default returns the built-in configuration used when chant.yml is absent.
func Default() Config {
	return Config{
		Version:    1,
		RecipesDir: "recipes",
		Retrieval: Retrieval{
			WeightLexical:     0.5,
			WeightTags:        0.3,
			WeightSuccessRate: 0.2,
			Threshold:         0.25,
		},
	}
}

// Load reads chant.yml from repoRoot, falling back to defaults for any unset
// field. A missing file is not an error — chant works with zero config.
func Load(repoRoot string) (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(filepath.Join(repoRoot, FileName))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	// Re-apply defaults for any field the file left at zero.
	d := Default()
	if cfg.RecipesDir == "" {
		cfg.RecipesDir = d.RecipesDir
	}
	if cfg.Retrieval.WeightLexical == 0 && cfg.Retrieval.WeightTags == 0 && cfg.Retrieval.WeightSuccessRate == 0 {
		cfg.Retrieval = d.Retrieval
	}
	if cfg.Retrieval.Threshold == 0 {
		cfg.Retrieval.Threshold = d.Retrieval.Threshold
	}
	return cfg, nil
}
