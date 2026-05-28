// Package outcome defines chant's stable JSON outcome vocabulary. Hooks and
// agents read these fields instead of parsing human prose, mirroring
// coherence's outcome contract so the two harnesses feel consistent.
package outcome

// Hit is one retrieved recipe candidate surfaced to an agent.
type Hit struct {
	ID             string   `json:"id"`
	Version        int      `json:"version"`
	Description    string   `json:"description,omitempty"`
	Confidence     float64  `json:"confidence"`
	Status         string   `json:"status,omitempty"`
	VerifierExists bool     `json:"verifier_exists"`
	Reasons        []string `json:"reasons,omitempty"`
	// ReuseCommand is the exact command an agent should run to reuse this
	// recipe (verifier-first): it runs then verifies.
	ReuseCommand string `json:"reuse_command,omitempty"`

	// ── cross-package discovery (spec §6) ────────────────────────────────
	// These fields are only set on FOREIGN hits from `suggest --global`: a
	// recipe whose source lives outside the current repo. They are additive
	// and omitempty so a local-only hit serializes exactly as before.
	//
	// Global marks a hit as coming from the per-machine registry rather than
	// the local library.
	Global bool `json:"global,omitempty"`
	// Origin is the provenance origin (repo/package) of a foreign hit.
	Origin string `json:"origin,omitempty"`
	// Scope is the foreign hit's maturity channel (project|domain|universal).
	Scope string `json:"scope,omitempty"`
	// SpellHash is the portable identity used to import a foreign hit.
	SpellHash string `json:"spell_hash,omitempty"`
}

// ScopePromotion is emitted on a verify (or promote) call that earned the
// recipe a higher scope (spec §5 — the universality ladder). It is purely
// informational: agents that don't care about scope can ignore it.
type ScopePromotion struct {
	Old      string `json:"old"`
	New      string `json:"new"`
	Contexts int    `json:"contexts"`
}

// Outcome is the union shape emitted by chant subcommands with --json. Unset
// fields are omitted so each subcommand's payload stays focused.
type Outcome struct {
	Subcommand string `json:"subcommand"`

	// suggest. match_found has no omitempty: agents gate on it, so it must be
	// present (false) even when the library is empty or nothing matched.
	MatchFound bool  `json:"match_found"`
	Hits       []Hit `json:"hits,omitempty"`

	// run / verify
	RecipeID    string `json:"recipe_id,omitempty"`
	Version     int    `json:"version,omitempty"`
	Executed    bool   `json:"executed,omitempty"`
	ExitCode    int    `json:"exit_code"`
	VerifierRan bool   `json:"verifier_ran,omitempty"`
	// Trusted is the verifier-first verdict: true ONLY after a passing
	// verifier. A cache hit alone never sets this.
	Trusted   bool  `json:"trusted"`
	RuntimeMS int64 `json:"runtime_ms,omitempty"`
	// LLMCallsAvoided is a reuse-win estimate carried on the recipe.
	LLMCallsAvoided int `json:"llm_calls_avoided,omitempty"`

	// ScopePromotion is set only when a verify (or promote) earned the
	// recipe a higher scope. Optional; absent by default.
	ScopePromotion *ScopePromotion `json:"scope_promotion,omitempty"`

	// promote (recompute-from-evidence)
	Scope         string `json:"scope,omitempty"`
	OldScope      string `json:"old_scope,omitempty"`
	ContextsCount int    `json:"contexts_count,omitempty"`

	// capture
	Captured bool `json:"captured,omitempty"`

	// invalidate / status
	Stale bool `json:"stale,omitempty"`

	// shared
	BlockingError          bool     `json:"blocking_error,omitempty"`
	Message                string   `json:"message,omitempty"`
	SuggestedCommands      []string `json:"suggested_commands,omitempty"`
	RecommendedNextCommand string   `json:"recommended_next_command,omitempty"`
}
