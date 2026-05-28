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
	return runEnv(t, bin, dir, nil, args...)
}

// runEnv executes the chant binary in dir with extra env vars (in K=V form)
// merged onto the current environment, returning combined stdout+stderr +
// exit code. Used by scope-promotion tests that need to set CHANT_CONTEXT.
func runEnv(t *testing.T, bin, dir string, extraEnv []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	// Hermetic: a clean-ish env, no network needed by any tested command.
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	cmd.Env = append(cmd.Env, extraEnv...)
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

// TestCLI_JSONErrorEmitsBlockingError verifies the error path honors --json:
// agents get a machine-readable blocking_error, not prose, and exit 1.
func TestCLI_JSONErrorEmitsBlockingError(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)
	out, code := run(t, bin, repo, "verify", "no-such-recipe", "--json")
	if code != 1 {
		t.Errorf("expected exit 1 for unknown recipe, got %d\n%s", code, out)
	}
	var e struct {
		BlockingError bool   `json:"blocking_error"`
		Message       string `json:"message"`
		Subcommand    string `json:"subcommand"`
	}
	if err := json.Unmarshal([]byte(out), &e); err != nil {
		t.Fatalf("error path did not emit JSON under --json: %v\n%s", err, out)
	}
	if !e.BlockingError || e.Subcommand != "verify" {
		t.Errorf("unexpected error JSON: %+v\n%s", e, out)
	}
}

// TestCLI_SuggestEmptyLibraryMatchFound verifies match_found is always present
// (false), even with no recipes, so agents can gate on it unconditionally.
func TestCLI_SuggestEmptyLibraryMatchFound(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)
	out, code := run(t, bin, repo, "suggest", "--task", "anything at all", "--json")
	if code != 0 {
		t.Fatalf("suggest exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, `"match_found"`) {
		t.Errorf("match_found missing from empty-library suggest --json:\n%s", out)
	}
}

// TestCLI_ScopePromotion: a recipe captured with --domains earns "domain"
// scope after a passing verify in two distinct CHANT_CONTEXT values, and the
// second verify reports the promotion in its JSON outcome (spec §5).
func TestCLI_ScopePromotion(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)

	// Capture with two domain labels so cluster signals exist. Use a verifier
	// that always passes so verify can establish trust.
	if _, code := run(t, bin, repo,
		"capture", "--id", "scopey", "--task", "promote me",
		"--command", "echo run", "--verifier", "sh -c true",
		"--domains", "csv,ecommerce", "--json"); code != 0 {
		t.Fatal("capture failed")
	}

	// First verify in context "ctx-a": still project (1 distinct context).
	out, code := runEnv(t, bin, repo, []string{"CHANT_CONTEXT=ctx-a"}, "verify", "--json", "scopey")
	if code != 0 {
		t.Fatalf("verify(ctx-a) exit %d:\n%s", code, out)
	}
	var v1 struct {
		Trusted        bool `json:"trusted"`
		ScopePromotion *struct {
			Old, New string
			Contexts int
		} `json:"scope_promotion,omitempty"`
	}
	if err := json.Unmarshal([]byte(out), &v1); err != nil {
		t.Fatalf("verify(ctx-a) JSON parse: %v\n%s", err, out)
	}
	if !v1.Trusted {
		t.Fatal("first verify not trusted")
	}
	if v1.ScopePromotion != nil {
		t.Errorf("first verify reported scope_promotion %+v; want none (still project)", v1.ScopePromotion)
	}

	// Second verify in context "ctx-b": should promote project → domain.
	out, code = runEnv(t, bin, repo, []string{"CHANT_CONTEXT=ctx-b"}, "verify", "--json", "scopey")
	if code != 0 {
		t.Fatalf("verify(ctx-b) exit %d:\n%s", code, out)
	}
	var v2 struct {
		Trusted        bool `json:"trusted"`
		ScopePromotion *struct {
			Old      string `json:"old"`
			New      string `json:"new"`
			Contexts int    `json:"contexts"`
		} `json:"scope_promotion,omitempty"`
	}
	if err := json.Unmarshal([]byte(out), &v2); err != nil {
		t.Fatalf("verify(ctx-b) JSON parse: %v\n%s", err, out)
	}
	if !v2.Trusted {
		t.Fatal("second verify not trusted")
	}
	if v2.ScopePromotion == nil {
		t.Fatalf("second verify did NOT report scope_promotion:\n%s", out)
	}
	if v2.ScopePromotion.Old != "project" || v2.ScopePromotion.New != "domain" {
		t.Errorf("promotion = %s → %s, want project → domain", v2.ScopePromotion.Old, v2.ScopePromotion.New)
	}
	if v2.ScopePromotion.Contexts != 2 {
		t.Errorf("promotion contexts = %d, want 2", v2.ScopePromotion.Contexts)
	}

	// The recipe.yaml on disk records the new scope and both contexts.
	b, err := os.ReadFile(filepath.Join(repo, "recipes", "scopey", "recipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	card := string(b)
	if !strings.Contains(card, "scope: domain") {
		t.Errorf("recipe card did not record scope=domain:\n%s", card)
	}
	if !strings.Contains(card, "ctx-a") || !strings.Contains(card, "ctx-b") {
		t.Errorf("recipe card missing one of the verified contexts:\n%s", card)
	}

	// Third verify in the SAME context as the second: no further promotion
	// (already domain; needs 3 contexts AND 2 domains for universal).
	out, code = runEnv(t, bin, repo, []string{"CHANT_CONTEXT=ctx-b"}, "verify", "--json", "scopey")
	if code != 0 {
		t.Fatalf("verify(ctx-b dup) exit %d:\n%s", code, out)
	}
	var v3 struct {
		ScopePromotion *json.RawMessage `json:"scope_promotion,omitempty"`
	}
	if err := json.Unmarshal([]byte(out), &v3); err != nil {
		t.Fatalf("verify(ctx-b dup) JSON parse: %v\n%s", err, out)
	}
	if v3.ScopePromotion != nil {
		t.Errorf("re-verify in same context spuriously promoted again: %s", string(*v3.ScopePromotion))
	}

	// And `chant promote` reports the current scope/old_scope as equal.
	out, code = run(t, bin, repo, "promote", "scopey", "--json")
	if code != 0 {
		t.Fatalf("promote exit %d:\n%s", code, out)
	}
	var p struct {
		Subcommand    string `json:"subcommand"`
		Scope         string `json:"scope"`
		OldScope      string `json:"old_scope"`
		ContextsCount int    `json:"contexts_count"`
	}
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("promote JSON parse: %v\n%s", err, out)
	}
	if p.Subcommand != "promote" || p.Scope != "domain" || p.OldScope != "domain" {
		t.Errorf("promote outcome = %+v, want subcommand=promote scope=domain old_scope=domain", p)
	}
	if p.ContextsCount != 2 {
		t.Errorf("promote contexts_count = %d, want 2", p.ContextsCount)
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

// TestCLI_ImportResetsMetrics locks in the NEW-1 fix from the naive-user v0.2
// pass: an imported recipe must NOT inherit the origin's metrics.runs /
// last_success_at. Without the reset, `chant list` shows "N run(s) 100% ok"
// right after `chant import`, directly contradicting the import command's own
// "NOT trusted yet" message and breaking the verifier-first invariant
// visually.
func TestCLI_ImportResetsMetrics(t *testing.T) {
	bin := buildBinary(t)
	regPath := filepath.Join(t.TempDir(), "reg.json")
	extraEnv := []string{"CHANT_REGISTRY=" + regPath}

	// Repo A: capture, verify (bumps metrics), index → push into the registry.
	repoA := newRepo(t)
	if _, code := runEnv(t, bin, repoA, extraEnv,
		"capture", "--id", "n1-greet", "--task", "greet someone",
		"--command", "echo hi", "--verifier", "sh -c true", "--json"); code != 0 {
		t.Fatal("capture in A failed")
	}
	if _, code := runEnv(t, bin, repoA, extraEnv, "verify", "n1-greet"); code != 0 {
		t.Fatal("verify in A failed")
	}
	if _, code := runEnv(t, bin, repoA, extraEnv, "index"); code != 0 {
		t.Fatal("index in A failed")
	}

	// Repo B: empty. Import by id.
	repoB := newRepo(t)
	if out, code := runEnv(t, bin, repoB, extraEnv, "import", "n1-greet"); code != 0 {
		t.Fatalf("import into B failed: %d\n%s", code, out)
	}

	// Read the imported card directly — metrics + verified_in must be empty.
	b, err := os.ReadFile(filepath.Join(repoB, "recipes", "n1-greet", "recipe.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	card := string(b)
	if strings.Contains(card, "last_success_at") {
		t.Errorf("imported card carries origin last_success_at — verifier-first leak:\n%s", card)
	}
	// runs: 0 / failures: 0 / no inherited verified_in are the local-evidence
	// invariants. We tolerate the field being absent (omitempty) or zero.
	if strings.Contains(card, "runs: 1") || strings.Contains(card, "runs: 2") {
		t.Errorf("imported card inherits non-zero runs:\n%s", card)
	}
	if strings.Contains(card, "verified_in:") {
		t.Errorf("imported card carries origin verified_in — should accumulate locally:\n%s", card)
	}

	// `chant list --json` in B must NOT report success_rate from origin
	// before any local verify.
	out, code := runEnv(t, bin, repoB, extraEnv, "list", "--json")
	if code != 0 {
		t.Fatalf("list exit %d:\n%s", code, out)
	}
	var idx struct {
		Recipes []struct {
			ID   string `json:"id"`
			Runs int    `json:"runs"`
		} `json:"recipes"`
	}
	if err := json.Unmarshal([]byte(out), &idx); err != nil {
		t.Fatalf("list JSON parse: %v\n%s", err, out)
	}
	if len(idx.Recipes) != 1 || idx.Recipes[0].ID != "n1-greet" {
		t.Fatalf("expected one recipe n1-greet in B, got: %+v", idx)
	}
	if idx.Recipes[0].Runs != 0 {
		t.Errorf("imported recipe shows runs=%d in `list --json` before any local verify (want 0)", idx.Recipes[0].Runs)
	}
}

// TestCLI_IndexNoRegistryOmitsWarning locks the NEW-2 fix: --no-registry
// --json must not emit a stray `registry_warning: ""` field.
func TestCLI_IndexNoRegistryOmitsWarning(t *testing.T) {
	bin := buildBinary(t)
	repo := newRepo(t)
	out, code := runEnv(t, bin, repo, nil,
		"capture", "--id", "x", "--task", "x", "--command", "echo x", "--verifier", "sh -c true", "--json")
	if code != 0 {
		t.Fatalf("capture exit %d:\n%s", code, out)
	}
	out, code = runEnv(t, bin, repo, nil, "index", "--no-registry", "--json")
	if code != 0 {
		t.Fatalf("index exit %d:\n%s", code, out)
	}
	if strings.Contains(out, `"registry_warning"`) {
		t.Errorf("--no-registry --json should omit registry_warning when empty, got:\n%s", out)
	}
}
