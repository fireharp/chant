package bench

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunProperties runs the opt-in Hegel property suite. Hegel imports live behind
// the "hegel" build tag; this wrapper keeps normal `go test ./...` and
// `chant bench --suite=all` free of Hegel runtime startup.
func RunProperties(root string) Summary {
	sum := Summary{Suite: "properties"}
	res := Result{
		ID:    "PROP-HEGEL",
		Name:  "Hegel generative verifier properties",
		Suite: "properties",
		Pass:  true,
	}

	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			res.Pass = false
			res.Detail = "could not determine working directory: " + err.Error()
			record(&sum, res)
			return sum
		}
	}

	hegelDir := filepath.Join(root, ".chant", "hegel", ".hegel")
	if err := os.MkdirAll(hegelDir, 0o755); err != nil {
		res.Pass = false
		res.Detail = "could not create Hegel state dir: " + err.Error()
		record(&sum, res)
		return sum
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-tags", "hegel", "./internal/bench", "-run", "TestHegelProperties", "-count=1")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"CHANT_REPO_ROOT="+root,
		"CHANT_HEGEL_DIR="+hegelDir,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		res.Pass = false
		res.Detail = "Hegel property suite timed out after 5m"
	} else if err != nil {
		res.Pass = false
		res.Detail = trimDetail(out.String())
		if res.Detail == "" {
			res.Detail = err.Error()
		}
	} else {
		res.Detail = "Hegel property tests passed"
	}
	record(&sum, res)
	return sum
}

func trimDetail(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) == 0 {
		return ""
	}
	const keep = 12
	if len(lines) > keep {
		lines = lines[len(lines)-keep:]
	}
	return fmt.Sprintf("%s", strings.Join(lines, "\n"))
}
