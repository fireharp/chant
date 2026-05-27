package runner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/fireharp/chant/internal/recipe"
)

func TestAdaptSubstitutes(t *testing.T) {
	out, missing := Adapt("echo {{name}} {{greeting}}", map[string]string{"name": "world", "greeting": "hi"})
	if out != "echo world hi" {
		t.Errorf("Adapt = %q, want %q", out, "echo world hi")
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
}

func TestAdaptReportsMissing(t *testing.T) {
	out, missing := Adapt("echo {{name}} {{absent}}", map[string]string{"name": "world"})
	if !reflect.DeepEqual(missing, []string{"absent"}) {
		t.Errorf("missing = %v, want [absent]", missing)
	}
	// Unresolved placeholder is left intact in the output.
	if out != "echo world {{absent}}" {
		t.Errorf("Adapt left output = %q, want unresolved placeholder kept", out)
	}
}

func TestAdaptWhitespaceInPlaceholder(t *testing.T) {
	out, missing := Adapt("run {{ name }}", map[string]string{"name": "x"})
	if out != "run x" {
		t.Errorf("Adapt with spaces = %q, want %q", out, "run x")
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
}

func TestAdaptNoPlaceholders(t *testing.T) {
	out, missing := Adapt("plain command", nil)
	if out != "plain command" || len(missing) != 0 {
		t.Errorf("Adapt(plain) = %q, %v", out, missing)
	}
}

// newRecipe writes a recipe.yaml into a temp dir and loads it so r.Dir() is set.
func newRecipe(t *testing.T, command, verifier string, artifacts []string) *recipe.Recipe {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "rec")
	r := &recipe.Recipe{
		ID:           "rec",
		Version:      1,
		WhatToDo:     recipe.WhatToDo{Command: command},
		Verification: recipe.Verification{Command: verifier, ExpectedArtifacts: artifacts},
	}
	r.SetDir(dir)
	if err := r.Save(); err != nil {
		t.Fatalf("save recipe: %v", err)
	}
	return r
}

func TestRunExecutesEcho(t *testing.T) {
	r := newRecipe(t, "echo hello", "", nil)
	res, err := Run(r, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.OK() {
		t.Errorf("Run not OK: exit=%d err=%v", res.ExitCode, res.Err)
	}
	if res.Stdout != "hello\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "hello\n")
	}
}

func TestRunSubstitutesInput(t *testing.T) {
	r := newRecipe(t, "echo {{word}}", "", nil)
	res, err := Run(r, map[string]string{"word": "chant"}, 5*time.Second)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != "chant\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "chant\n")
	}
}

func TestRunMissingInputErrors(t *testing.T) {
	r := newRecipe(t, "echo {{word}}", "", nil)
	if _, err := Run(r, nil, 5*time.Second); err == nil {
		t.Fatal("expected error for missing input, got nil")
	}
}

func TestRunNoCommandErrors(t *testing.T) {
	r := newRecipe(t, "", "", nil)
	if _, err := Run(r, nil, 5*time.Second); err == nil {
		t.Fatal("expected error for recipe with no command")
	}
}

func TestRunExposesInputEnv(t *testing.T) {
	// Inputs are exposed as CHANT_INPUT_<KEY> uppercased.
	r := newRecipe(t, `echo "$CHANT_INPUT_NAME"`, "", nil)
	res, err := Run(r, map[string]string{"name": "fromenv"}, 5*time.Second)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != "fromenv\n" {
		t.Errorf("env-exposed input Stdout = %q, want %q", res.Stdout, "fromenv\n")
	}
}

func TestRunInRecipeDir(t *testing.T) {
	r := newRecipe(t, "pwd", "", nil)
	res, err := Run(r, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// macOS /tmp is a symlink to /private/tmp, so resolve before comparing.
	wantDir, _ := filepath.EvalSymlinks(r.Dir())
	gotDir, _ := filepath.EvalSymlinks(string([]byte(res.Stdout[:len(res.Stdout)-1]))) // strip trailing newline
	if gotDir != wantDir {
		t.Errorf("command ran in %q, want recipe dir %q", gotDir, wantDir)
	}
}

// ---- the verifier-first trust gate ----

func TestVerifyPassingVerifierTrusted(t *testing.T) {
	r := newRecipe(t, "true", "sh -c true", nil)
	res, trusted, err := Verify(r, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if !trusted {
		t.Errorf("trusted = false for a passing verifier, want true (res=%+v)", res)
	}
}

func TestVerifyFailingVerifierNotTrustedNoError(t *testing.T) {
	r := newRecipe(t, "true", `sh -c "exit 1"`, nil)
	res, trusted, err := Verify(r, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("Verify on a failing verifier should NOT error, got: %v", err)
	}
	if trusted {
		t.Error("trusted = true for a failing verifier, want false")
	}
	if res.OK() {
		t.Error("failing verifier reported Result.OK() == true")
	}
}

func TestVerifyMissingArtifactNotTrusted(t *testing.T) {
	// Verifier passes, but a declared expected artifact does not exist.
	r := newRecipe(t, "true", "sh -c true", []string{"does-not-exist.txt"})
	res, trusted, err := Verify(r, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("Verify should not error on a missing artifact, got: %v", err)
	}
	if trusted {
		t.Error("trusted = true despite a missing expected artifact, want false")
	}
	if res.Stderr == "" {
		t.Error("expected stderr note about the missing artifact")
	}
}

func TestVerifyPresentArtifactTrusted(t *testing.T) {
	r := newRecipe(t, "true", "sh -c true", []string{"out.txt"})
	// Create the expected artifact inside the recipe dir.
	if err := os.WriteFile(filepath.Join(r.Dir(), "out.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, trusted, err := Verify(r, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !trusted {
		t.Error("trusted = false despite passing verifier and present artifact")
	}
}

func TestVerifyArtifactOnlyTrusted(t *testing.T) {
	// No verifier command, only an expected artifact that the run produces.
	r := newRecipe(t, "echo hi > produced.txt", "", []string{"produced.txt"})
	if _, err := Run(r, nil, 5*time.Second); err != nil {
		t.Fatalf("Run: %v", err)
	}
	_, trusted, err := Verify(r, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !trusted {
		t.Error("artifact-only verifier: trusted = false, want true once the artifact exists")
	}
}

func TestVerifyNoVerifierErrors(t *testing.T) {
	// No verifier command AND no expected artifacts → cannot establish trust → error.
	r := newRecipe(t, "true", "", nil)
	_, trusted, err := Verify(r, nil, 5*time.Second)
	if err == nil {
		t.Fatal("expected error when a recipe has no verifier at all")
	}
	if trusted {
		t.Error("trusted = true when there is no verifier")
	}
}

func TestVerifyMissingInputErrors(t *testing.T) {
	r := newRecipe(t, "true", "test {{flag}}", nil)
	_, trusted, err := Verify(r, nil, 5*time.Second)
	if err == nil {
		t.Fatal("expected error: verifier references a missing input")
	}
	if trusted {
		t.Error("trusted = true despite missing verifier input")
	}
}

func TestVerifyArtifactAbsolutePath(t *testing.T) {
	r := newRecipe(t, "true", "sh -c true", nil)
	abs := filepath.Join(t.TempDir(), "abs-artifact.txt")
	r.Verification.ExpectedArtifacts = []string{abs}
	// Missing first.
	if _, trusted, _ := Verify(r, nil, 5*time.Second); trusted {
		t.Error("trusted with a missing absolute-path artifact")
	}
	if err := os.WriteFile(abs, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, trusted, err := Verify(r, nil, 5*time.Second); err != nil || !trusted {
		t.Errorf("absolute artifact present: trusted=%v err=%v, want trusted/no error", trusted, err)
	}
}

func TestResultOK(t *testing.T) {
	if !(Result{ExitCode: 0}).OK() {
		t.Error("clean Result.OK() = false")
	}
	if (Result{ExitCode: 1}).OK() {
		t.Error("nonzero exit Result.OK() = true")
	}
}
