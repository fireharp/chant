// Package runner executes a recipe's procedure and verifier. It enforces
// chant's central rule: a retrieved recipe is a *candidate*, and only a
// passing verifier promotes a result to "trusted". Reuse is therefore
//
//	retrieve → adapt → execute → verify → accept
//
// not
//
//	retrieve → trust
//
// which is the difference between a useful cache and a wrong-answer amplifier.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fireharp/chant/internal/recipe"
)

// Result captures one command execution.
type Result struct {
	Command    string
	ExitCode   int
	Stdout     string
	Stderr     string
	DurationMS int64
	Err        error
}

// OK reports a clean exit.
func (r Result) OK() bool { return r.Err == nil && r.ExitCode == 0 }

var placeholderRE = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

// Adapt substitutes {{var}} placeholders in the recipe command from inputs.
// Unresolved placeholders are reported so the caller can fail fast rather than
// run a half-formed command.
func Adapt(command string, inputs map[string]string) (string, []string) {
	var missing []string
	out := placeholderRE.ReplaceAllStringFunc(command, func(m string) string {
		key := strings.TrimSpace(placeholderRE.FindStringSubmatch(m)[1])
		if v, ok := inputs[key]; ok {
			return v
		}
		missing = append(missing, key)
		return m
	})
	return out, missing
}

// Run executes the recipe's procedure in its own directory after adapting
// {{var}} placeholders from inputs. It does NOT decide trust — call Verify for
// that.
func Run(r *recipe.Recipe, inputs map[string]string, timeout time.Duration) (Result, error) {
	cmd := r.WhatToDo.Command
	if cmd == "" {
		return Result{}, fmt.Errorf("recipe %q has no what_to_do.command", r.ID)
	}
	adapted, missing := Adapt(cmd, inputs)
	if len(missing) > 0 {
		return Result{Command: adapted}, fmt.Errorf("recipe %q is missing inputs: %s", r.ID, strings.Join(missing, ", "))
	}
	return execShell(adapted, r.Dir(), inputs, timeout), nil
}

// Verify runs the recipe's verifier and applies the trust gate. The boolean
// return is the trust decision: true only when the verifier command exits 0
// (or there is no verifier command but all expected artifacts exist).
func Verify(r *recipe.Recipe, inputs map[string]string, timeout time.Duration) (Result, bool, error) {
	v := r.Verification
	if v.Command == "" && len(v.ExpectedArtifacts) == 0 {
		return Result{}, false, fmt.Errorf("recipe %q has no verifier (verification.command or expected_artifacts) — cannot establish trust", r.ID)
	}

	var res Result
	if v.Command != "" {
		adapted, missing := Adapt(v.Command, inputs)
		if len(missing) > 0 {
			return Result{Command: adapted}, false, fmt.Errorf("verifier for %q is missing inputs: %s", r.ID, strings.Join(missing, ", "))
		}
		res = execShell(adapted, r.Dir(), inputs, timeout)
		if !res.OK() {
			return res, false, nil // verifier failed → not trusted
		}
	}

	// All declared artifacts must exist for the result to be trusted.
	for _, art := range v.ExpectedArtifacts {
		p := art
		if !filepath.IsAbs(p) {
			p = filepath.Join(r.Dir(), art)
		}
		if _, err := os.Stat(p); err != nil {
			res.Stderr += fmt.Sprintf("\nexpected artifact missing: %s", art)
			return res, false, nil
		}
	}
	return res, true, nil
}

func execShell(command, dir string, inputs map[string]string, timeout time.Duration) Result {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	c := exec.CommandContext(ctx, "sh", "-c", command)
	c.Dir = dir
	c.Env = os.Environ()
	// Expose inputs as CHANT_INPUT_<KEY> env vars too, so recipes can read
	// them without shell interpolation if they prefer.
	for k, v := range inputs {
		c.Env = append(c.Env, "CHANT_INPUT_"+strings.ToUpper(k)+"="+v)
	}
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()

	res := Result{
		Command:    command,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMS: time.Since(start).Milliseconds(),
		Err:        err,
	}
	if c.ProcessState != nil {
		res.ExitCode = c.ProcessState.ExitCode()
	}
	if ctx.Err() == context.DeadlineExceeded {
		res.Stderr += fmt.Sprintf("\nverifier/command timed out after %s", timeout)
	}
	return res
}
