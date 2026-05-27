package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fireharp/chant/internal/store"
)

const chantYMLTemplate = `version: 1
# chant.yml — configuration for the recipe cache.
# Recipes live under recipes_dir (committed). Runtime state is in .chant/ (gitignored).

recipes_dir: recipes

retrieval:
  # Hybrid scorer weights. A hit above threshold is a *candidate*; only a
  # passing verifier makes a reuse result trusted.
  weight_lexical: 0.5        # task-text overlap with description/patterns
  weight_tags: 0.3           # structural file/column signal match
  weight_success_rate: 0.2   # verifier track record
  threshold: 0.25            # minimum score for 'chant suggest' to report a match
`

const chantSkillTemplate = `---
name: chant
description: Use before writing new code (check for a reusable verified recipe) and after solving a task (capture the procedure as a recipe). chant caches the tested method, not the answer.
---

# chant

chant is a recipe cache for agents. It stores successful work as **verified,
reusable procedures** and reuses them verifier-first.

## Before writing new code

` + "```bash" + `
chant suggest --task "<what you are about to do>" --files "<input files>" --json
` + "```" + `

Read the JSON: ` + "`match_found`, `hits[].id`, `hits[].verifier_exists`, `hits[].reuse_command`" + `.
If a hit has a verifier, reuse it instead of writing new code:

` + "```bash" + `
chant verify <id> --input file=<path> --json    # runs + verifies; trust only if "trusted": true
` + "```" + `

A cache hit is a *candidate*, never trusted until the verifier passes.

## After solving a task

If you wrote code that solved a recurring task and it passes a test/verifier,
capture it so the next similar task reuses it:

` + "```bash" + `
chant capture --id <slug> --task "<task>" \
  --command "<command to reuse, may contain {{vars}}>" \
  --verifier "<verifier command, exit 0 == passed>" \
  --tags "<comma,tags>" --json
chant verify <slug>     # confirm the verifier passes
` + "```" + `

## Maintenance

- ` + "`chant list`" + ` / ` + "`chant search \"<query>\"`" + ` — browse the library.
- ` + "`chant invalidate <id>`" + ` — mark a recipe stale when it stops working.
- ` + "`chant status`" + ` / ` + "`chant doctor`" + ` — health of the recipe library.

Treat ` + "`.chant/`" + ` as local runtime state. Recipes under ` + "`recipes/`" + ` are committed.
`

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	force := fs.Bool("force", false, "overwrite existing files")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, _ := store.FindRoot(wd)

	var created, skipped []string
	write := func(rel, content string, mode os.FileMode) error {
		p := filepath.Join(root, rel)
		if _, err := os.Stat(p); err == nil && !*force {
			skipped = append(skipped, rel)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(content), mode); err != nil {
			return err
		}
		created = append(created, rel)
		return nil
	}

	if err := write("chant.yml", chantYMLTemplate, 0o644); err != nil {
		return err
	}
	if err := write(filepath.Join("recipes", ".gitkeep"), "", 0o644); err != nil {
		return err
	}
	if err := write(filepath.Join(".agents", "skills", "chant", "SKILL.md"), chantSkillTemplate, 0o644); err != nil {
		return err
	}

	// Ensure .chant/ is gitignored (append, don't clobber).
	giPath := filepath.Join(root, ".gitignore")
	gi, _ := os.ReadFile(giPath)
	if !strings.Contains(string(gi), store.StateDir) {
		f, err := os.OpenFile(giPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if len(gi) > 0 && !strings.HasSuffix(string(gi), "\n") {
			_, _ = f.WriteString("\n")
		}
		_, _ = f.WriteString(store.StateDir + "/\n")
		_ = f.Close()
		created = append(created, ".gitignore (+"+store.StateDir+"/)")
	} else {
		skipped = append(skipped, ".gitignore (already ignores "+store.StateDir+"/)")
	}

	// Create the runtime state dir.
	if err := os.MkdirAll(filepath.Join(root, store.StateDir), 0o755); err != nil {
		return err
	}

	if *asJSON {
		return emitJSON(map[string]any{"created": created, "skipped": skipped, "root": root})
	}
	fmt.Printf("chant init in %s\n", root)
	for _, c := range created {
		fmt.Printf("  created  %s\n", c)
	}
	for _, sk := range skipped {
		fmt.Printf("  skipped  %s\n", sk)
	}
	fmt.Print(`
Next:
  $ chant capture --id <slug> --task "..." --command "..." --verifier "..."
  $ chant verify <slug>
  $ chant suggest --task "..." --json    # wire into your agent's pre-write hook

Pre-commit: chant pairs with coherence. If you want a chant gate too, add
'chant status' or a 'chant bench' line to .githooks/pre-commit.
`)
	return nil
}
