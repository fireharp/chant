package recipe

import (
	"path/filepath"
	"testing"
)

// TestDetectContextEnvOverride: CHANT_CONTEXT wins over any other detection.
// This is the hermetic-test escape hatch the verify path uses to simulate
// distinct contexts without standing up two real git remotes.
func TestDetectContextEnvOverride(t *testing.T) {
	t.Setenv(ContextEnv, "ctx-from-env")
	got := DetectContext("/tmp/some-fake-repo-root-that-does-not-exist")
	if got != "ctx-from-env" {
		t.Errorf("DetectContext with env override = %q, want %q", got, "ctx-from-env")
	}
}

// TestDetectContextEnvTrimmed: leading/trailing whitespace in the env override
// is trimmed before being returned.
func TestDetectContextEnvTrimmed(t *testing.T) {
	t.Setenv(ContextEnv, "   spacey-ctx   ")
	got := DetectContext("/tmp/x")
	if got != "spacey-ctx" {
		t.Errorf("DetectContext trimmed override = %q, want %q", got, "spacey-ctx")
	}
}

// TestDetectContextPathFallback: with no env override and a directory that has
// no git remote, the absolute path is returned. We point at the test's TempDir
// so the result is stable.
func TestDetectContextPathFallback(t *testing.T) {
	// Make sure no inherited CHANT_CONTEXT leaks in.
	t.Setenv(ContextEnv, "")
	dir := t.TempDir()
	got := DetectContext(dir)
	// Don't compare against `dir` directly: filepath.Abs may canonicalize the
	// path (e.g. /var → /private/var on macOS). Compare against the same
	// canonicalization.
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("DetectContext path fallback = %q, want %q", got, want)
	}
}

// TestNormalizeRemoteURL_Forms covers the URL shapes git emits.
func TestNormalizeRemoteURL_Forms(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"git@github.com:fireharp/chant.git", "github.com/fireharp/chant"},
		{"https://github.com/fireharp/chant.git", "github.com/fireharp/chant"},
		{"https://user:token@github.com/fireharp/chant", "github.com/fireharp/chant"},
		{"ssh://git@github.com/fireharp/chant", "github.com/fireharp/chant"},
		{"", ""},
	}
	for _, tt := range cases {
		if got := NormalizeRemoteURL(tt.in); got != tt.want {
			t.Errorf("NormalizeRemoteURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
