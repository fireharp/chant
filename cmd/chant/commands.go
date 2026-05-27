package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fireharp/chant/internal/bench"
	"github.com/fireharp/chant/internal/config"
	"github.com/fireharp/chant/internal/outcome"
	"github.com/fireharp/chant/internal/recipe"
	"github.com/fireharp/chant/internal/retrieve"
	"github.com/fireharp/chant/internal/runner"
	"github.com/fireharp/chant/internal/status"
	"github.com/fireharp/chant/internal/store"
	"gopkg.in/yaml.v3"
)

// reuseCommand is the verifier-first reuse hint surfaced for a hit.
func reuseCommand(id string) string {
	return fmt.Sprintf("chant verify %s   # run + verify before trusting", id)
}

// parseFlags parses args allowing flags and positionals to be interspersed.
// Go's flag package stops at the first positional, so `chant verify <id> --json`
// would otherwise silently drop --json. We re-parse around each positional and
// return the collected positionals.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positionals, nil
		}
		positionals = append(positionals, rest[0])
		args = rest[1:]
	}
}

func toHits(matches []retrieve.Match) []outcome.Hit {
	var hits []outcome.Hit
	for _, m := range matches {
		hits = append(hits, outcome.Hit{
			ID:             m.Recipe.ID,
			Version:        m.Recipe.Version,
			Description:    m.Recipe.Description,
			Confidence:     round2(m.Score),
			Status:         m.Recipe.Status,
			VerifierExists: m.Recipe.Verification.Command != "" || len(m.Recipe.Verification.ExpectedArtifacts) > 0,
			Reasons:        m.Reasons,
			ReuseCommand:   reuseCommand(m.Recipe.ID),
		})
	}
	return hits
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

// ---- suggest ----

func cmdSuggest(args []string) error {
	fs := flag.NewFlagSet("suggest", flag.ContinueOnError)
	task := fs.String("task", "", "natural-language task description")
	files := fs.String("files", "", "comma-separated input file names/paths")
	columns := fs.String("columns", "", "comma-separated available column names")
	asJSON := fs.Bool("json", false, "emit JSON outcome contract")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*task) == "" {
		return fmt.Errorf("suggest requires --task")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	recs, err := s.LoadAll()
	if err != nil {
		return err
	}
	q := retrieve.Query{Task: *task, Files: splitCSV(*files), Columns: splitCSV(*columns)}
	matches := retrieve.Suggest(recs, q, s.Config.Retrieval)
	hits := toHits(matches)

	out := outcome.Outcome{Subcommand: "suggest", MatchFound: len(hits) > 0, Hits: hits}
	if len(hits) > 0 {
		out.RecommendedNextCommand = reuseCommand(hits[0].ID)
	} else {
		out.RecommendedNextCommand = "no recipe matched — solve the task, then `chant capture` it"
	}
	if *asJSON {
		return emitJSON(out)
	}
	if len(hits) == 0 {
		fmt.Printf("no recipe matched %q above threshold %.2f\n", *task, s.Config.Retrieval.Threshold)
		fmt.Println("→ solve the task, then capture it with `chant capture`.")
		return nil
	}
	fmt.Printf("%d recipe candidate(s) for %q:\n", len(hits), *task)
	for _, h := range hits {
		ver := "no verifier"
		if h.VerifierExists {
			ver = "verifier available"
		}
		fmt.Printf("  • %-28s v%d  confidence %.2f  [%s]\n", h.ID, h.Version, h.Confidence, ver)
		fmt.Printf("      reuse: %s\n", h.ReuseCommand)
	}
	return nil
}

// ---- capture ----

func cmdCapture(args []string) error {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	id := fs.String("id", "", "recipe id (slug). Defaults to a slug of --task")
	task := fs.String("task", "", "task description (becomes a task pattern)")
	desc := fs.String("description", "", "recipe description")
	command := fs.String("command", "", "what_to_do command (may contain {{vars}})")
	language := fs.String("language", "", "informational language tag")
	entrypoint := fs.String("entrypoint", "", "entrypoint filename inside the recipe dir")
	entrypointSrc := fs.String("entrypoint-src", "", "copy this file into the recipe dir as the entrypoint")
	verifier := fs.String("verifier", "", "verification command (exit 0 == verified)")
	artifacts := fs.String("expect-artifacts", "", "comma-separated expected output artifacts")
	tags := fs.String("tags", "", "comma-separated tags")
	patterns := fs.String("patterns", "", "comma-separated extra task patterns")
	fileSignals := fs.String("file-signals", "", "comma-separated input file globs")
	columns := fs.String("columns", "", "comma-separated required column aliases (one alias group)")
	author := fs.String("author", "", "provenance author (defaults to agent:capture)")
	force := fs.Bool("force", false, "overwrite an existing recipe")
	asJSON := fs.Bool("json", false, "emit JSON outcome contract")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*task) == "" && strings.TrimSpace(*id) == "" {
		return fmt.Errorf("capture requires --task or --id")
	}
	if strings.TrimSpace(*command) == "" {
		return fmt.Errorf("capture requires --command (the procedure to reuse)")
	}
	rid := *id
	if rid == "" {
		rid = recipe.Slug(*task)
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	if s.Exists(rid) && !*force {
		return fmt.Errorf("recipe %q already exists (use --force to overwrite, or invalidate + recapture for a new version)", rid)
	}

	description := *desc
	if description == "" {
		description = *task
	}
	taskPatterns := splitCSV(*patterns)
	if strings.TrimSpace(*task) != "" {
		taskPatterns = append([]string{*task}, taskPatterns...)
	}

	// Column signals, if supplied, become one alias group: they feed both the
	// retrieval input_signals and the portability input contract below.
	var columnsAny [][]string
	if cols := splitCSV(*columns); len(cols) > 0 {
		columnsAny = [][]string{cols}
	}

	r := &recipe.Recipe{
		ID:          rid,
		Version:     1,
		Kind:        "executable_recipe",
		Description: description,
		Status:      "active",
		WhenToUse: recipe.WhenToUse{
			TaskPatterns: taskPatterns,
			Tags:         splitCSV(*tags),
			InputSignals: recipe.InputSignals{
				Files:      splitCSV(*fileSignals),
				ColumnsAny: columnsAny,
			},
		},
		WhatToDo: recipe.WhatToDo{
			Entrypoint: *entrypoint,
			Command:    *command,
			Language:   *language,
		},
		Verification: recipe.Verification{
			Command:           *verifier,
			ExpectedArtifacts: splitCSV(*artifacts),
		},
	}
	r.SetDir(s.DirFor(rid))

	// Optionally copy the entrypoint source into the recipe dir.
	if *entrypointSrc != "" {
		if err := os.MkdirAll(r.Dir(), 0o755); err != nil {
			return err
		}
		dst := *entrypoint
		if dst == "" {
			dst = filepath.Base(*entrypointSrc)
			r.WhatToDo.Entrypoint = dst
		}
		b, err := os.ReadFile(*entrypointSrc)
		if err != nil {
			return fmt.Errorf("read entrypoint-src: %w", err)
		}
		if err := os.WriteFile(filepath.Join(r.Dir(), dst), b, 0o755); err != nil {
			return err
		}
	}

	r.ComputeFingerprints()

	// ── enchantment metadata (spec §8 step 2) ────────────────────────────
	// Everything here is best-effort and optional: a minimal capture still
	// produces a working recipe. spell_hash is computed after fingerprints so
	// the entrypoint source (if copied above) is on disk for the content hash.
	r.SpellHash = r.ComputeSpellHash()

	authorName := strings.TrimSpace(*author)
	if authorName == "" {
		authorName = "agent:capture"
	}
	r.Provenance = recipe.Provenance{
		Origin:     detectOrigin(s.Root),
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Author:     authorName,
	}

	// Default to project scope; promotion is earned via verified_in (spec §5).
	if r.Scope == "" {
		r.Scope = "project"
	}

	// Portability contract: determinism is best-effort (a recipe with a
	// verifier and no recorded side effects is treated as deterministic), the
	// runtime is reused from --language / dependencies, and the input contract
	// carries any column signals and schema fingerprint we have.
	r.Portability.Determinism = "deterministic"
	r.Portability.Requires.Runtime = captureRuntime(r)
	if r.Fingerprints.SchemaFingerprint != "" {
		r.Portability.InputContract.SchemaFingerprint = r.Fingerprints.SchemaFingerprint
	}
	if len(columnsAny) > 0 {
		r.Portability.InputContract.RequiredColumnsAny = columnsAny
	}

	if err := r.Save(); err != nil {
		return err
	}
	if _, err := s.WriteIndex(); err != nil {
		return err
	}

	hasVerifier := r.Verification.Command != "" || len(r.Verification.ExpectedArtifacts) > 0
	out := outcome.Outcome{
		Subcommand: "capture", Captured: true, RecipeID: rid, Version: 1,
		RecommendedNextCommand: fmt.Sprintf("chant verify %s", rid),
	}
	if hasVerifier {
		out.Message = "recipe captured — verify it to establish trust"
	} else {
		// Surface the no-verifier warning in the machine-readable contract too,
		// so an agent knows it wrote an un-trustable recipe.
		out.Message = "captured WITHOUT a verifier — reuse cannot be trusted until you add one"
		out.SuggestedCommands = []string{fmt.Sprintf("chant capture --id %s --force --verifier \"<cmd>\" ...", rid)}
	}
	if *asJSON {
		return emitJSON(out)
	}
	fmt.Printf("captured recipe %q (v1) at %s\n", rid, r.Dir())
	if hasVerifier {
		fmt.Printf("→ run `chant verify %s` to confirm the verifier passes.\n", rid)
	} else {
		fmt.Println("⚠ no verifier set — add one so reuse can be trusted (a hit without a verifier is just a guess).")
	}
	return nil
}

// detectOrigin best-effort determines the provenance origin: the git remote
// "origin" URL (normalized to host/path) if available, else the repo root
// path. Never fails — provenance fields are optional.
func detectOrigin(root string) string {
	out, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output()
	if err == nil {
		if u := normalizeRemoteURL(strings.TrimSpace(string(out))); u != "" {
			return u
		}
	}
	return root
}

// normalizeRemoteURL turns a git remote URL into a stable host/path form
// (e.g. "github.com/fireharp/chant"), dropping scheme, credentials, and the
// trailing ".git". Returns "" if it can't parse anything useful.
func normalizeRemoteURL(raw string) string {
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

// captureRuntime resolves the portability runtime from the recipe's pinned
// dependencies, falling back to the informational language tag.
func captureRuntime(r *recipe.Recipe) string {
	if r.Dependencies.Runtime != "" {
		return r.Dependencies.Runtime
	}
	if r.WhatToDo.Language != "" {
		return r.WhatToDo.Language
	}
	return ""
}

// ---- list ----

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	idx, err := s.WriteIndex()
	if err != nil {
		return err
	}
	if *asJSON {
		return emitJSON(idx)
	}
	if idx.Count == 0 {
		fmt.Println("no recipes yet — capture one with `chant capture`.")
		return nil
	}
	fmt.Printf("%d recipe(s):\n", idx.Count)
	for _, e := range idx.Recipes {
		flag := ""
		if e.Status == "stale" {
			flag = " (stale)"
		}
		fmt.Printf("  %-30s v%d  %d run(s) %.0f%% ok%s\n", e.ID, e.Version, e.Runs, e.SuccessRate*100, flag)
		if e.Description != "" {
			fmt.Printf("      %s\n", e.Description)
		}
	}
	return nil
}

// ---- search ----

func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit JSON")
	positionals, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	query := strings.Join(positionals, " ")
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("search requires a query")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	recs, err := s.LoadAll()
	if err != nil {
		return err
	}
	matches := retrieve.Rank(recs, retrieve.Query{Task: query}, s.Config.Retrieval)
	if *asJSON {
		return emitJSON(outcome.Outcome{Subcommand: "search", Hits: toHits(matches)})
	}
	if len(matches) == 0 {
		fmt.Println("no recipes to search.")
		return nil
	}
	fmt.Printf("ranked recipes for %q:\n", query)
	for _, m := range matches {
		fmt.Printf("  %.2f  %-30s (lex %.2f, signal %.2f, ok %.2f)\n",
			round2(m.Score), m.Recipe.ID, m.Lexical, m.SignalMatch, m.SuccessRate)
	}
	return nil
}

// ---- explain ----

func cmdExplain(args []string) error {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit JSON")
	positionals, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(positionals) == 0 {
		return fmt.Errorf("explain requires a recipe id")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	r, err := s.Get(positionals[0])
	if err != nil {
		return err
	}
	if *asJSON {
		// Emit the card with its recipe.yaml (snake_case) field names rather
		// than Go field names, so explain matches the rest of the JSON contract.
		yb, err := yaml.Marshal(r)
		if err != nil {
			return err
		}
		var m map[string]any
		if err := yaml.Unmarshal(yb, &m); err != nil {
			return err
		}
		return emitJSON(m)
	}
	fmt.Printf("# %s (v%d) — %s\n\n", r.ID, r.Version, r.Status)
	fmt.Printf("%s\n\n", r.Description)
	if len(r.WhenToUse.TaskPatterns) > 0 {
		fmt.Println("when to use:")
		for _, p := range r.WhenToUse.TaskPatterns {
			fmt.Printf("  - %s\n", p)
		}
	}
	if len(r.WhenToUse.Tags) > 0 {
		fmt.Printf("tags: %s\n", strings.Join(r.WhenToUse.Tags, ", "))
	}
	fmt.Printf("\nwhat to do:\n  %s\n", r.WhatToDo.Command)
	if r.Verification.Command != "" {
		fmt.Printf("\nverify with:\n  %s\n", r.Verification.Command)
	} else {
		fmt.Printf("\n⚠ no verifier — reuse cannot be trusted.\n")
	}
	fmt.Printf("\nmetrics: %d run(s), %d failure(s), %.0f%% success\n",
		r.Metrics.Runs, r.Metrics.Failures, r.Metrics.SuccessRate()*100)
	return nil
}

// ---- run ----

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	inputs := kvFlag{}
	fs.Var(inputs, "input", "k=v input (repeatable); fills {{k}} placeholders")
	timeout := fs.Duration("timeout", 60*time.Second, "command timeout")
	asJSON := fs.Bool("json", false, "emit JSON")
	positionals, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(positionals) == 0 {
		return fmt.Errorf("run requires a recipe id")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	r, err := s.Get(positionals[0])
	if err != nil {
		return err
	}
	res, err := runner.Run(r, inputs, *timeout)
	if err != nil {
		return err
	}
	logRun(s, r, inputs, res, false, false)

	out := outcome.Outcome{
		Subcommand: "run", RecipeID: r.ID, Version: r.Version,
		Executed: true, ExitCode: res.ExitCode, RuntimeMS: res.DurationMS,
		Trusted:                false, // run alone never establishes trust
		RecommendedNextCommand: fmt.Sprintf("chant verify %s", r.ID),
	}
	if *asJSON {
		if err := emitJSON(out); err != nil {
			return err
		}
	} else {
		fmt.Print(res.Stdout)
		if res.Stderr != "" {
			fmt.Fprint(os.Stderr, res.Stderr)
		}
		fmt.Printf("\n[ran %s in %dms, exit %d] — run `chant verify %s` to establish trust.\n",
			r.ID, res.DurationMS, res.ExitCode, r.ID)
	}
	if !res.OK() {
		os.Exit(1)
	}
	return nil
}

// ---- verify ----

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	inputs := kvFlag{}
	fs.Var(inputs, "input", "k=v input (repeatable)")
	timeout := fs.Duration("timeout", 60*time.Second, "verifier timeout")
	run := fs.Bool("run", true, "run the procedure before verifying")
	asJSON := fs.Bool("json", false, "emit JSON")
	positionals, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(positionals) == 0 {
		return fmt.Errorf("verify requires a recipe id")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	r, err := s.Get(positionals[0])
	if err != nil {
		return err
	}

	// Default input convenience: map the first example as {{input}} when set.
	if len(r.Examples) > 0 {
		if _, ok := inputs["input"]; !ok {
			inputs["input"] = r.Examples[0].Input
		}
		if _, ok := inputs["input_file"]; !ok {
			inputs["input_file"] = r.Examples[0].Input
		}
	}

	if *run {
		if _, err := runner.Run(r, inputs, *timeout); err != nil {
			return err
		}
	}
	res, trusted, err := runner.Verify(r, inputs, *timeout)
	if err != nil {
		return err
	}

	// Record the verification as the trust event.
	r.RecordRun(trusted)
	if trusted && r.IsStale() {
		r.Status = "active" // a passing verifier re-blesses a stale recipe
	}
	_ = r.Save()
	_, _ = s.WriteIndex()
	logRun(s, r, inputs, res, true, trusted)

	out := outcome.Outcome{
		Subcommand: "verify", RecipeID: r.ID, Version: r.Version,
		Executed: *run, VerifierRan: true, ExitCode: res.ExitCode,
		Trusted: trusted, RuntimeMS: res.DurationMS,
	}
	if trusted {
		out.Message = "verifier passed — result is trusted"
	} else {
		out.Message = "verifier did NOT pass — result is NOT trusted; repair or invalidate"
		out.RecommendedNextCommand = fmt.Sprintf("chant invalidate %s", r.ID)
	}
	if *asJSON {
		if err := emitJSON(out); err != nil {
			return err
		}
	} else {
		if res.Stdout != "" {
			fmt.Print(res.Stdout)
		}
		if trusted {
			fmt.Printf("✓ %s verified — trusted (%dms)\n", r.ID, res.DurationMS)
		} else {
			fmt.Printf("✗ %s NOT verified — do not trust this result.\n", r.ID)
			if res.Stderr != "" {
				fmt.Fprintln(os.Stderr, strings.TrimSpace(res.Stderr))
			}
		}
	}
	// Verifier-first: a non-trusted result is a failure regardless of output
	// mode, so CI/hooks keying on the exit code behave correctly.
	if !trusted {
		os.Exit(1)
	}
	return nil
}

// ---- invalidate ----

func cmdInvalidate(args []string) error {
	fs := flag.NewFlagSet("invalidate", flag.ContinueOnError)
	reason := fs.String("reason", "", "why the recipe is being invalidated")
	asJSON := fs.Bool("json", false, "emit JSON")
	positionals, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(positionals) == 0 {
		return fmt.Errorf("invalidate requires a recipe id")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	r, err := s.Get(positionals[0])
	if err != nil {
		return err
	}
	r.MarkStale()
	if err := r.Save(); err != nil {
		return err
	}
	_, _ = s.WriteIndex()
	out := outcome.Outcome{
		Subcommand: "invalidate", RecipeID: r.ID, Stale: true,
		Message: "recipe marked stale — a passing `chant verify` will re-bless it",
	}
	if *reason != "" {
		out.Message += " (" + *reason + ")"
	}
	if *asJSON {
		return emitJSON(out)
	}
	fmt.Printf("marked %s stale. %s\n", r.ID, out.Message)
	return nil
}

// ---- index ----

func cmdIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	idx, err := s.WriteIndex()
	if err != nil {
		return err
	}
	if *asJSON {
		return emitJSON(idx)
	}
	fmt.Printf("indexed %d recipe(s) → %s\n", idx.Count, s.StatePath("index.json"))
	return nil
}

// ---- status ----

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	rep, err := status.Build(s)
	if err != nil {
		return err
	}
	if err := status.Write(s, rep); err != nil {
		return err
	}
	if *asJSON {
		return emitJSON(rep)
	}
	fmt.Printf("wrote %s (%d recipe(s), %d active, %d stale)\n",
		s.StatePath("STATUS.md"), rep.RecipeCount, rep.ActiveCount, rep.StaleCount)
	return nil
}

// ---- doctor ----

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	type check struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	}
	var checks []check
	add := func(name, st, detail string) { checks = append(checks, check{name, st, detail}) }

	// config
	if _, err := os.Stat(filepath.Join(s.Root, config.FileName)); err == nil {
		add("config", "ok", "chant.yml present")
	} else {
		add("config", "warn", "chant.yml absent — using defaults (run `chant init`)")
	}
	// recipes dir
	if _, err := os.Stat(s.RecipesDir()); err == nil {
		add("recipes-dir", "ok", s.Config.RecipesDir+"/ present")
	} else {
		add("recipes-dir", "warn", s.Config.RecipesDir+"/ missing — run `chant init`")
	}
	// recipes load + verifier coverage
	recs, lerr := s.LoadAll()
	if lerr != nil {
		add("recipes", "fail", lerr.Error())
	} else {
		noVerifier := 0
		for _, r := range recs {
			if r.Verification.Command == "" && len(r.Verification.ExpectedArtifacts) == 0 {
				noVerifier++
			}
		}
		if noVerifier > 0 {
			add("verifiers", "warn", fmt.Sprintf("%d/%d recipe(s) have no verifier — reuse can't be trusted", noVerifier, len(recs)))
		} else {
			add("verifiers", "ok", fmt.Sprintf("all %d recipe(s) have a verifier", len(recs)))
		}
	}
	// gitignore
	if b, err := os.ReadFile(filepath.Join(s.Root, ".gitignore")); err == nil && strings.Contains(string(b), store.StateDir) {
		add("gitignore", "ok", store.StateDir+"/ is gitignored")
	} else {
		add("gitignore", "warn", store.StateDir+"/ not gitignored — add it")
	}

	fail := false
	for _, c := range checks {
		if c.Status == "fail" {
			fail = true
		}
	}
	if *asJSON {
		if err := emitJSON(map[string]any{"checks": checks, "ok": !fail}); err != nil {
			return err
		}
	} else {
		for _, c := range checks {
			fmt.Printf("[%s] %s: %s\n", c.Status, c.Name, c.Detail)
		}
		if fail {
			fmt.Println("doctor: blocking issues found.")
		} else {
			fmt.Println("doctor: no blocking issues.")
		}
	}
	if fail {
		os.Exit(1)
	}
	return nil
}

// ---- bench ----

func cmdBench(args []string) error {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	suite := fs.String("suite", "all", "retrieval | e2e | all")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	var summaries []bench.Summary
	if *suite == "retrieval" || *suite == "all" {
		summaries = append(summaries, bench.RunRetrieval(s.Config.Retrieval))
	}
	if *suite == "e2e" || *suite == "all" {
		e2e, err := bench.RunE2E(s)
		if err != nil {
			return err
		}
		summaries = append(summaries, e2e)
	}
	failed := 0
	for _, sum := range summaries {
		failed += sum.Failed
	}
	if *asJSON {
		if err := emitJSON(map[string]any{"summaries": summaries, "failed": failed}); err != nil {
			return err
		}
	} else {
		for _, sum := range summaries {
			fmt.Printf("\n== suite: %s (%d/%d passed) ==\n", sum.Suite, sum.Passed, sum.Total)
			for _, r := range sum.Results {
				mark := "PASS"
				if !r.Pass {
					mark = "FAIL"
				}
				fmt.Printf("  [%s] %-8s %s — %s\n", mark, r.ID, r.Name, r.Detail)
			}
		}
		if failed > 0 {
			fmt.Printf("\nbench: %d scenario(s) failed.\n", failed)
		} else {
			fmt.Printf("\nbench: all scenarios passed.\n")
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
	return nil
}

// logRun persists a run record under .chant/runs/.
func logRun(s *store.Store, r *recipe.Recipe, inputs map[string]string, res runner.Result, verifierRan, verified bool) {
	_, _ = s.WriteRun(store.RunRecord{
		RecipeID:    r.ID,
		Version:     r.Version,
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
		DurationMS:  res.DurationMS,
		Command:     res.Command,
		Inputs:      inputs,
		ExitCode:    res.ExitCode,
		VerifierRan: verifierRan,
		Verified:    verified,
		Stdout:      truncate(res.Stdout, 4000),
		Stderr:      truncate(res.Stderr, 4000),
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…[truncated]"
}
