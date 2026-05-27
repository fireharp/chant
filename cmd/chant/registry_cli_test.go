package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runReg is like run() but injects a hermetic CHANT_REGISTRY pointing at a
// per-test temp file, so cross-package discovery never touches the real $HOME.
func runReg(t *testing.T, bin, dir, registryPath string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "CHANT_REGISTRY="+registryPath)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running %v: %v", args, err)
	}
	return string(out), code
}

// TestCLI_CrossPackageRegistryFlow exercises the full MVP loop (spec §6):
// capture+index in repo A, then in a SEPARATE repo B discover A's enchantment
// via `suggest --global`, then `import` it and confirm it lands locally and is
// NOT trusted (verifier-first).
func TestCLI_CrossPackageRegistryFlow(t *testing.T) {
	bin := buildBinary(t)
	registryPath := filepath.Join(t.TempDir(), "registry", "index.json")

	repoA := newRepo(t)
	repoB := newRepo(t)

	// ── repo A: capture a recipe, then index (which upserts into registry) ──
	if _, code := runReg(t, bin, repoA, registryPath,
		"capture", "--id", "csv-revenue", "--task", "compute revenue by channel from csv",
		"--command", "echo run", "--verifier", "sh -c true",
		"--tags", "csv,revenue", "--file-signals", "*.csv", "--json"); code != 0 {
		t.Fatal("repo A capture failed")
	}
	out, code := runReg(t, bin, repoA, registryPath, "index", "--json")
	if code != 0 {
		t.Fatalf("repo A index exit %d:\n%s", code, out)
	}
	var idxOut struct {
		RegistryUpserted int    `json:"registry_upserted"`
		RegistryWarning  string `json:"registry_warning"`
	}
	if err := json.Unmarshal([]byte(out), &idxOut); err != nil {
		t.Fatalf("index JSON parse: %v\n%s", err, out)
	}
	if idxOut.RegistryWarning != "" {
		t.Fatalf("index reported registry warning: %s", idxOut.RegistryWarning)
	}
	if idxOut.RegistryUpserted != 1 {
		t.Errorf("registry_upserted = %d, want 1", idxOut.RegistryUpserted)
	}

	// The registry file exists and records the spell_hash + absolute path.
	regBytes, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("registry not written: %v", err)
	}
	var reg struct {
		Entries []struct {
			SpellHash  string `json:"spell_hash"`
			ID         string `json:"id"`
			RecipePath string `json:"recipe_path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(regBytes, &reg); err != nil {
		t.Fatalf("registry JSON parse: %v\n%s", err, regBytes)
	}
	if len(reg.Entries) != 1 {
		t.Fatalf("registry has %d entries, want 1", len(reg.Entries))
	}
	spellHash := reg.Entries[0].SpellHash
	if spellHash == "" {
		t.Fatal("registry entry has empty spell_hash")
	}
	if !filepath.IsAbs(reg.Entries[0].RecipePath) {
		t.Errorf("recipe_path %q is not absolute", reg.Entries[0].RecipePath)
	}

	// ── repo B (no local recipes): suggest --global finds A's as FOREIGN ──
	// Local suggest must NOT find it (different repo, empty library).
	out, code = runReg(t, bin, repoB, registryPath,
		"suggest", "--task", "compute revenue by channel from this csv", "--json")
	if code != 0 {
		t.Fatalf("repo B local suggest exit %d:\n%s", code, out)
	}
	var localOnly struct {
		MatchFound bool `json:"match_found"`
	}
	if err := json.Unmarshal([]byte(out), &localOnly); err != nil {
		t.Fatalf("local suggest JSON parse: %v\n%s", err, out)
	}
	if localOnly.MatchFound {
		t.Errorf("local-only suggest in empty repo B found a match (registry leaked into local):\n%s", out)
	}

	// With --global it should surface a foreign hit.
	out, code = runReg(t, bin, repoB, registryPath,
		"suggest", "--task", "compute revenue by channel from this csv", "--global", "--json")
	if code != 0 {
		t.Fatalf("repo B global suggest exit %d:\n%s", code, out)
	}
	var global struct {
		MatchFound bool `json:"match_found"`
		Hits       []struct {
			ID             string `json:"id"`
			Global         bool   `json:"global"`
			Origin         string `json:"origin"`
			Scope          string `json:"scope"`
			SpellHash      string `json:"spell_hash"`
			VerifierExists bool   `json:"verifier_exists"`
			ReuseCommand   string `json:"reuse_command"`
		} `json:"hits"`
		RecommendedNextCommand string `json:"recommended_next_command"`
	}
	if err := json.Unmarshal([]byte(out), &global); err != nil {
		t.Fatalf("global suggest JSON parse: %v\n%s", err, out)
	}
	if !global.MatchFound || len(global.Hits) == 0 {
		t.Fatalf("global suggest found no foreign hit:\n%s", out)
	}
	h := global.Hits[0]
	if !h.Global {
		t.Errorf("foreign hit not marked global: %+v", h)
	}
	if h.SpellHash != spellHash {
		t.Errorf("foreign hit spell_hash = %q, want %q", h.SpellHash, spellHash)
	}
	if h.Scope != "project" {
		t.Errorf("foreign hit scope = %q, want project", h.Scope)
	}
	if !strings.Contains(h.ReuseCommand, "chant import") {
		t.Errorf("foreign hit reuse_command should be import-then-verify, got %q", h.ReuseCommand)
	}
	if !strings.Contains(global.RecommendedNextCommand, "chant import") {
		t.Errorf("recommended_next_command should be import for a top foreign hit, got %q", global.RecommendedNextCommand)
	}

	// ── repo B: import by spell_hash, confirm NOT trusted and on disk ──
	out, code = runReg(t, bin, repoB, registryPath, "import", spellHash, "--json")
	if code != 0 {
		t.Fatalf("import exit %d:\n%s", code, out)
	}
	var imp struct {
		Subcommand             string `json:"subcommand"`
		RecipeID               string `json:"recipe_id"`
		Trusted                bool   `json:"trusted"`
		RecommendedNextCommand string `json:"recommended_next_command"`
	}
	if err := json.Unmarshal([]byte(out), &imp); err != nil {
		t.Fatalf("import JSON parse: %v\n%s", err, out)
	}
	if imp.Trusted {
		t.Error("import marked the recipe trusted — verifier-first violated")
	}
	if imp.RecipeID != "csv-revenue" {
		t.Errorf("import recipe_id = %q, want csv-revenue", imp.RecipeID)
	}
	if !strings.Contains(imp.RecommendedNextCommand, "chant verify") {
		t.Errorf("import next step should be verify, got %q", imp.RecommendedNextCommand)
	}
	// The card now exists in repo B's local library.
	if _, err := os.Stat(filepath.Join(repoB, "recipes", "csv-revenue", "recipe.yaml")); err != nil {
		t.Fatalf("imported recipe card not in repo B: %v", err)
	}

	// Re-import without --force must refuse (no clobber).
	if _, code := runReg(t, bin, repoB, registryPath, "import", spellHash, "--json"); code == 0 {
		t.Error("re-import without --force should fail, but exited 0")
	}

	// After import, a LOCAL suggest in repo B now finds it (it is local now).
	out, code = runReg(t, bin, repoB, registryPath,
		"suggest", "--task", "compute revenue by channel from this csv", "--json")
	if code != 0 {
		t.Fatalf("post-import local suggest exit %d:\n%s", code, out)
	}
	var postLocal struct {
		MatchFound bool `json:"match_found"`
		Hits       []struct {
			ID     string `json:"id"`
			Global bool   `json:"global"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(out), &postLocal); err != nil {
		t.Fatalf("post-import suggest JSON parse: %v\n%s", err, out)
	}
	if !postLocal.MatchFound || len(postLocal.Hits) == 0 || postLocal.Hits[0].ID != "csv-revenue" || postLocal.Hits[0].Global {
		t.Errorf("post-import local suggest did not surface the imported recipe as a local hit: %+v\n%s", postLocal, out)
	}
}

// TestCLI_ImportAs verifies --as imports under a new id and rewrites the card.
func TestCLI_ImportAs(t *testing.T) {
	bin := buildBinary(t)
	registryPath := filepath.Join(t.TempDir(), "registry", "index.json")
	repoA := newRepo(t)
	repoB := newRepo(t)

	if _, code := runReg(t, bin, repoA, registryPath,
		"capture", "--id", "orig", "--task", "do a thing",
		"--command", "echo x", "--verifier", "sh -c true", "--json"); code != 0 {
		t.Fatal("capture failed")
	}
	if _, code := runReg(t, bin, repoA, registryPath, "index"); code != 0 {
		t.Fatal("index failed")
	}
	if _, code := runReg(t, bin, repoB, registryPath, "import", "orig", "--as", "renamed", "--json"); code != 0 {
		t.Fatal("import --as failed")
	}
	b, err := os.ReadFile(filepath.Join(repoB, "recipes", "renamed", "recipe.yaml"))
	if err != nil {
		t.Fatalf("renamed recipe not written: %v", err)
	}
	if !strings.Contains(string(b), "id: renamed") {
		t.Errorf("imported card did not adopt the new id:\n%s", b)
	}
}

// TestCLI_IndexNoRegistryFlag verifies --no-registry skips the registry upsert.
func TestCLI_IndexNoRegistryFlag(t *testing.T) {
	bin := buildBinary(t)
	registryPath := filepath.Join(t.TempDir(), "registry", "index.json")
	repo := newRepo(t)

	if _, code := runReg(t, bin, repo, registryPath,
		"capture", "--id", "r", "--task", "x", "--command", "echo x", "--verifier", "sh -c true", "--json"); code != 0 {
		t.Fatal("capture failed")
	}
	if _, code := runReg(t, bin, repo, registryPath, "index", "--no-registry", "--json"); code != 0 {
		t.Fatal("index --no-registry failed")
	}
	if _, err := os.Stat(registryPath); !os.IsNotExist(err) {
		t.Errorf("--no-registry still wrote the registry file (err=%v)", err)
	}
}
