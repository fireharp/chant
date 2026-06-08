package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type PropertySpec struct {
	ID       string
	Name     string
	TestName string
	Cases    int
}

var propertySpecs = []PropertySpec{
	{ID: "PROP-retrieval-stale-penalty", Name: "retrieval stale penalty", TestName: "PROP-retrieval-stale-penalty", Cases: 50},
	{ID: "PROP-retrieval-signal-monotonicity", Name: "retrieval signal monotonicity", TestName: "PROP-retrieval-signal-monotonicity", Cases: 50},
	{ID: "PROP-runner-trust-gate", Name: "runner trust gate", TestName: "PROP-runner-trust-gate", Cases: 50},
	{ID: "PROP-spell-hash-stability", Name: "spell hash stability", TestName: "PROP-spell-hash-stability", Cases: 50},
	{ID: "PROP-csv-recipe-oracle", Name: "CSV recipe oracle", TestName: "PROP-csv-recipe-oracle", Cases: 25},
}

func PropertySpecs() []PropertySpec {
	return append([]PropertySpec(nil), propertySpecs...)
}

type propertyReport struct {
	GeneratedAt string   `json:"generated_at"`
	Suite       string   `json:"suite"`
	Results     []Result `json:"results"`
}

// RunProperties runs the opt-in Hegel property suite. Hegel imports live behind
// the "hegel" build tag; this wrapper keeps normal `go test ./...` and
// `chant bench --suite=all` free of Hegel runtime startup.
func RunProperties(root string) Summary {
	sum := Summary{Suite: "properties"}

	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			res := Result{
				ID:     "PROP-setup",
				Name:   "property suite setup",
				Suite:  "properties",
				Pass:   false,
				Detail: "could not determine working directory: " + err.Error(),
			}
			record(&sum, res)
			return sum
		}
	}

	hegelRoot := filepath.Join(root, ".chant", "hegel")
	hegelDir := filepath.Join(hegelRoot, ".hegel")
	failureDir := filepath.Join(hegelRoot, "failures")
	reportPath := filepath.Join(hegelRoot, "properties-report.json")
	for _, dir := range []string{hegelDir, failureDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			res := Result{
				ID:     "PROP-setup",
				Name:   "property suite setup",
				Suite:  "properties",
				Pass:   false,
				Detail: "could not create Hegel state dir: " + err.Error(),
			}
			record(&sum, res)
			return sum
		}
	}

	for _, spec := range propertySpecs {
		record(&sum, runProperty(root, hegelDir, failureDir, spec))
	}

	report := propertyReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Suite:       "properties",
		Results:     sum.Results,
	}
	if err := writeJSON(reportPath, report); err != nil {
		res := Result{
			ID:     "PROP-report",
			Name:   "property report",
			Suite:  "properties",
			Pass:   false,
			Detail: "could not write property report: " + err.Error(),
		}
		record(&sum, res)
	}
	return sum
}

func runProperty(root, hegelDir, failureDir string, spec PropertySpec) Result {
	res := Result{
		ID:     spec.ID,
		Name:   spec.Name,
		Suite:  "properties",
		Pass:   true,
		Cases:  spec.Cases,
		Detail: "Hegel property passed",
	}

	failurePath := filepath.Join(failureDir, spec.ID+".json")
	_ = os.Remove(failurePath)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	runPattern := fmt.Sprintf("^TestHegelProperties$/^%s$", regexp.QuoteMeta(spec.TestName))
	cmd := exec.CommandContext(ctx, "go", "test", "-tags", "hegel", "./internal/bench", "-run", runPattern, "-count=1", "-v")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"CHANT_REPO_ROOT="+root,
		"CHANT_HEGEL_DIR="+hegelDir,
		"CHANT_HEGEL_FAILURE_DIR="+failureDir,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		res.Pass = false
		res.Detail = "Hegel property timed out after 5m"
	} else if err != nil {
		res.Pass = false
		res.Detail = trimDetail(out.String())
		if res.Detail == "" {
			res.Detail = err.Error()
		}
	}
	if !res.Pass {
		if rel, ok := existingArtifact(root, failurePath); ok {
			res.FailureArtifact = rel
		}
	}
	return res
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func existingArtifact(root, path string) (string, bool) {
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel, true
	}
	return path, true
}

func recordPropertyFailure(failureDir, propertyID string, payload map[string]any) (string, error) {
	if failureDir == "" {
		return "", nil
	}
	if err := os.MkdirAll(failureDir, 0o755); err != nil {
		return "", err
	}
	record := make(map[string]any, len(payload)+2)
	for k, v := range payload {
		record[k] = v
	}
	record["property_id"] = propertyID
	record["recorded_at"] = time.Now().UTC().Format(time.RFC3339)
	path := filepath.Join(failureDir, propertyID+".json")
	if err := writeJSON(path, record); err != nil {
		return "", err
	}
	return path, nil
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
	return strings.Join(lines, "\n")
}
