package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles the chant CLI once into a temp dir and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "chant-test-bin")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

// run executes the chant binary in dir and returns combined stdout, exit code.
func run(t *testing.T, bin, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	// Hermetic: a clean-ish env, no network needed by any tested command.
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running %v: %v", args, err)
	}
	return string(out), code
}

// newRepo makes a t.TempDir that FindRoot will treat as a repo root.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// A chant.yml marker makes this the discovered root, isolating the test
	// from the developer's real repo above the temp dir.
	if err := os.WriteFile(filepath.Join(dir, "chant.yml"), []byte("version: 1\nrecipes_dir: recipes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCLI_CaptureListVerifyHappyPath(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)

	// capture: a trivial recipe whose verifier always passes.
	out, code := run(t, bin, repo,
		"capture", "--id", "hello", "--task", "say hello",
		"--command", "echo hi", "--verifier", "sh -c true", "--json")
	if code != 0 {
		t.Fatalf("capture exit %d:\n%s", code, out)
	}
	var cap struct {
		Subcommand string `json:"subcommand"`
		Captured   bool   `json:"captured"`
		RecipeID   string `json:"recipe_id"`
	}
	if err := json.Unmarshal([]byte(out), &cap); err != nil {
		t.Fatalf("capture JSON parse: %v\n%s", err, out)
	}
	if !cap.Captured || cap.RecipeID != "hello" {
		t.Errorf("capture outcome = %+v", cap)
	}
	// The card landed on disk.
	if _, err := os.Stat(filepath.Join(repo, "recipes", "hello", "recipe.yaml")); err != nil {
		t.Fatalf("recipe card not written: %v", err)
	}

	// list: shows exactly one recipe.
	out, code = run(t, bin, repo, "list", "--json")
	if code != 0 {
		t.Fatalf("list exit %d:\n%s", code, out)
	}
	var idx struct {
		Count   int `json:"count"`
		Recipes []struct {
			ID string `json:"id"`
		} `json:"recipes"`
	}
	if err := json.Unmarshal([]byte(out), &idx); err != nil {
		t.Fatalf("list JSON parse: %v\n%s", err, out)
	}
	if idx.Count != 1 || len(idx.Recipes) != 1 || idx.Recipes[0].ID != "hello" {
		t.Errorf("list outcome = %+v", idx)
	}

	// verify: passing verifier → trusted.
	// NOTE: flags MUST precede the positional id. Go's flag package stops
	// parsing at the first non-flag arg, so `verify hello --json` would
	// silently ignore --json. See BUG note in the test report.
	out, code = run(t, bin, repo, "verify", "--json", "hello")
	if code != 0 {
		t.Fatalf("verify exit %d:\n%s", code, out)
	}
	var ver struct {
		Trusted     bool `json:"trusted"`
		VerifierRan bool `json:"verifier_ran"`
		ExitCode    int  `json:"exit_code"`
	}
	if err := json.Unmarshal([]byte(out), &ver); err != nil {
		t.Fatalf("verify JSON parse: %v\n%s", err, out)
	}
	if !ver.Trusted {
		t.Errorf("verify trusted = false, want true:\n%s", out)
	}
	if !ver.VerifierRan {
		t.Error("verify verifier_ran = false, want true")
	}
}

func TestCLI_VerifyFailingNotTrustedExit1(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)

	if _, code := run(t, bin, repo,
		"capture", "--id", "broken", "--task", "always fails",
		"--command", "true", "--verifier", `sh -c "exit 1"`, "--json"); code != 0 {
		t.Fatalf("capture failed, exit %d", code)
	}

	out, code := run(t, bin, repo, "verify", "--json", "broken")
	var ver struct {
		Trusted  bool `json:"trusted"`
		ExitCode int  `json:"exit_code"`
	}
	if err := json.Unmarshal([]byte(out), &ver); err != nil {
		t.Fatalf("verify JSON parse: %v\n%s", err, out)
	}
	// The core trust signal must be correct: a failing verifier is NOT trusted.
	if ver.Trusted {
		t.Error("failing verifier reported trusted = true")
	}
	if ver.ExitCode != 1 {
		t.Errorf("verifier exit_code in JSON = %d, want 1", ver.ExitCode)
	}
	// Verifier-first: an untrusted result must exit nonzero even in --json mode,
	// so a CI/hook keying on the process exit code behaves correctly.
	if code != 1 {
		t.Errorf("verify --json of a failing recipe exited %d, want 1", code)
	}
}

func TestCLI_SuggestTrueNegative(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)

	// Capture a CSV recipe, then suggest against an unrelated task.
	if _, code := run(t, bin, repo,
		"capture", "--id", "csv-revenue", "--task", "compute revenue by channel from csv",
		"--command", "echo run", "--verifier", "sh -c true",
		"--tags", "csv,revenue", "--file-signals", "*.csv", "--json"); code != 0 {
		t.Fatal("capture failed")
	}

	out, code := run(t, bin, repo,
		"suggest", "--task", "rotate the kubernetes TLS certificates in staging", "--json")
	if code != 0 {
		t.Fatalf("suggest exit %d:\n%s", code, out)
	}
	var sg struct {
		MatchFound bool `json:"match_found"`
	}
	if err := json.Unmarshal([]byte(out), &sg); err != nil {
		t.Fatalf("suggest JSON parse: %v\n%s", err, out)
	}
	if sg.MatchFound {
		t.Errorf("suggest match_found = true for an unrelated query (true-negative expected)\n%s", out)
	}
}

func TestCLI_SuggestPositiveMatch(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)

	if _, code := run(t, bin, repo,
		"capture", "--id", "csv-revenue", "--task", "compute revenue by channel from csv",
		"--command", "echo run", "--verifier", "sh -c true",
		"--tags", "csv,revenue", "--file-signals", "*.csv", "--json"); code != 0 {
		t.Fatal("capture failed")
	}

	out, code := run(t, bin, repo,
		"suggest", "--task", "compute revenue by channel from this csv", "--files", "orders.csv", "--json")
	if code != 0 {
		t.Fatalf("suggest exit %d:\n%s", code, out)
	}
	var sg struct {
		MatchFound bool `json:"match_found"`
		Hits       []struct {
			ID             string `json:"id"`
			VerifierExists bool   `json:"verifier_exists"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(out), &sg); err != nil {
		t.Fatalf("suggest JSON parse: %v\n%s", err, out)
	}
	if !sg.MatchFound || len(sg.Hits) == 0 || sg.Hits[0].ID != "csv-revenue" {
		t.Errorf("suggest positive match failed: %+v\n%s", sg, out)
	}
	if !sg.Hits[0].VerifierExists {
		t.Error("hit verifier_exists = false, want true")
	}
}

func TestCLI_InvalidateMarksStale(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)

	if _, code := run(t, bin, repo,
		"capture", "--id", "tostale", "--task", "x",
		"--command", "echo x", "--verifier", "sh -c true", "--json"); code != 0 {
		t.Fatal("capture failed")
	}
	out, code := run(t, bin, repo, "invalidate", "--json", "tostale")
	if code != 0 {
		t.Fatalf("invalidate exit %d:\n%s", code, out)
	}
	var inv struct {
		Stale bool `json:"stale"`
	}
	if err := json.Unmarshal([]byte(out), &inv); err != nil {
		t.Fatalf("invalidate JSON parse: %v\n%s", err, out)
	}
	if !inv.Stale {
		t.Error("invalidate did not report stale")
	}
	// The card on disk reflects the stale status.
	b, err := os.ReadFile(filepath.Join(repo, "recipes", "tostale", "recipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "stale") {
		t.Errorf("recipe card does not record stale status:\n%s", b)
	}
}

// TestCLI_FlagAfterPositional verifies flags are honored AFTER the positional
// recipe id (e.g. `chant verify <id> --json`), matching usage()/SKILL.md.
// parseFlags lets flags and positionals be interspersed, so Go flag's
// stop-at-first-positional behavior no longer silently drops the flag.
func TestCLI_FlagAfterPositional(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)
	if _, code := run(t, bin, repo,
		"capture", "--id", "quirk", "--task", "x",
		"--command", "echo x", "--verifier", "sh -c true", "--json"); code != 0 {
		t.Fatal("capture failed")
	}
	// Flag AFTER the id must be honored: --json should emit JSON.
	out, code := run(t, bin, repo, "verify", "quirk", "--json")
	if code != 0 {
		t.Fatalf("verify exit %d:\n%s", code, out)
	}
	var ver struct {
		Trusted bool `json:"trusted"`
	}
	if err := json.Unmarshal([]byte(out), &ver); err != nil {
		t.Fatalf("verify <id> --json did not emit JSON (flag-after-positional broken): %v\n%s", err, out)
	}
	if !ver.Trusted {
		t.Errorf("expected trusted=true for a passing verifier, got:\n%s", out)
	}
}

func TestCLI_UnknownCommandExit2(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)
	out, code := run(t, bin, repo, "frobnicate")
	if code != 2 {
		t.Errorf("unknown command exit = %d, want 2\n%s", code, out)
	}
}
