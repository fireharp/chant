package recipe

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ContextEnv is the environment variable that overrides the detected context
// for the current process. Tests set this so a verifier passing in two
// distinct "contexts" can be simulated without standing up two real git
// remotes. See DetectContext.
const ContextEnv = "CHANT_CONTEXT"

// DetectContext returns a stable identifier for the current repo so a passing
// verifier can be recorded against a distinct context (spec §5 — verified_in).
//
// Resolution order:
//
//  1. The CHANT_CONTEXT env var (test override; makes contexts hermetic).
//  2. `git config --get remote.origin.url` for repoRoot, normalized to the
//     stable `host/owner/repo` form (matching cmdCapture's provenance origin).
//  3. The absolute repoRoot path as a last-resort fallback.
//
// The function is best-effort and never fails: callers treat an empty string
// as "no context to record" (which means scope promotion doesn't advance).
func DetectContext(repoRoot string) string {
	if v := strings.TrimSpace(os.Getenv(ContextEnv)); v != "" {
		return v
	}
	if repoRoot != "" {
		// `git -C <root> config --get remote.origin.url` works on bare/non-bare
		// repos alike. We don't run `git remote get-url` to keep the call shape
		// identical to what cmdCapture used historically.
		out, err := exec.Command("git", "-C", repoRoot, "config", "--get", "remote.origin.url").Output()
		if err == nil {
			if u := NormalizeRemoteURL(strings.TrimSpace(string(out))); u != "" {
				return u
			}
		}
		if abs, err := filepath.Abs(repoRoot); err == nil {
			return abs
		}
		return repoRoot
	}
	return ""
}

// NormalizeRemoteURL turns a git remote URL into a stable `host/owner/repo`
// form (e.g. "github.com/fireharp/chant"), dropping scheme, credentials, and
// the trailing ".git". Returns "" if no useful form can be derived.
//
// This intentionally mirrors cmd/chant.normalizeRemoteURL so the context
// recorded in verified_in matches the provenance origin written at capture.
// Replicated rather than shared because internal/recipe must not import
// cmd/chant; future work could move the helper here and have cmd/chant call
// recipe.NormalizeRemoteURL.
func NormalizeRemoteURL(raw string) string {
	if raw == "" {
		return ""
	}
	s := raw
	// scp-like form: git@github.com:owner/repo.git
	if strings.HasPrefix(s, "git@") {
		s = strings.TrimPrefix(s, "git@")
		s = strings.Replace(s, ":", "/", 1)
	} else {
		// strip scheme://[user@]
		if i := strings.Index(s, "://"); i >= 0 {
			s = s[i+3:]
		}
		if i := strings.Index(s, "@"); i >= 0 {
			s = s[i+1:]
		}
	}
	s = strings.TrimSuffix(s, ".git")
	s = strings.Trim(s, "/")
	return s
}
