// Command chant turns successful agent work into reusable, versioned, verified
// recipes — and reuses them verifier-first. It is the recipe-cache complement
// to coherence (a drift detector); both are harnesses for agent-edited repos.
//
// chant caches the tested *way* of solving a recurring task, not a cached
// answer. A retrieved recipe is a candidate; only a passing verifier makes a
// reuse result trusted.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fireharp/chant/internal/store"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "suggest":
		err = cmdSuggest(args)
	case "capture":
		err = cmdCapture(args)
	case "list":
		err = cmdList(args)
	case "search":
		err = cmdSearch(args)
	case "explain":
		err = cmdExplain(args)
	case "run":
		err = cmdRun(args)
	case "verify":
		err = cmdVerify(args)
	case "invalidate":
		err = cmdInvalidate(args)
	case "promote":
		err = cmdPromote(args)
	case "index":
		err = cmdIndex(args)
	case "import":
		err = cmdImport(args)
	case "doctor":
		err = cmdDoctor(args)
	case "status":
		err = cmdStatus(args)
	case "bench":
		err = cmdBench(args)
	case "version", "--version", "-v":
		fmt.Printf("chant %s\n", version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "chant: unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		// Honor the JSON contract on the error path too: agents asked for
		// --json should get a machine-readable blocking_error, not prose.
		if hasFlag(args, "json") {
			b, _ := json.MarshalIndent(map[string]any{
				"subcommand":     cmd,
				"blocking_error": true,
				"message":        err.Error(),
			}, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Fprintf(os.Stderr, "chant: %v\n", err)
		}
		os.Exit(1)
	}
}

// hasFlag reports whether a --name (or -name) flag appears anywhere in args.
func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == "--"+name || a == "-"+name {
			return true
		}
	}
	return false
}

func usage() {
	fmt.Print(`chant — a recipe cache for coding agents: cache the tested method, not the answer.

Usage: chant <command> [flags]

Lifecycle (the agent hook surface):
  suggest   --task "..." [--files a,b] [--columns a,b] [--global]   find a reusable recipe before writing new code
  capture   --id <id> --task "..." --command "..."        distill solved work into a verified recipe
  run       <id> [--input k=v ...]                        execute a recipe (adapts {{vars}})
  verify    <id> [--input k=v ...]                        run the verifier; only a pass is "trusted"

Library:
  list                       list recipes
  search    "<query>"        rank recipes against a query
  explain   <id>             print a recipe card
  invalidate <id>            mark a recipe stale
  promote   <id>             recompute scope (project→domain→universal) from verified_in evidence
  import    <id|spell_hash> [--as <id>]   copy a foreign enchantment locally (then chant verify)

Repo:
  init                       scaffold chant.yml, recipes/, skill, gitignore
  index    [--no-registry]   rebuild .chant/index.json (and upsert into the per-machine registry)
  status                     rewrite .chant/STATUS.md
  doctor                     validate config + store
  bench [--suite=retrieval|e2e|all]   run the validation suite
  version

Most commands accept --json for a stable machine-readable outcome contract.
`)
}

// ---- flag helpers ----

// kvFlag collects repeatable --input k=v values.
type kvFlag map[string]string

func (k kvFlag) String() string { return "" }
func (k kvFlag) Set(v string) error {
	parts := strings.SplitN(v, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("expected k=v, got %q", v)
	}
	k[parts[0]] = parts[1]
	return nil
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func emitJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func openStore() (*store.Store, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return store.Open(wd)
}
