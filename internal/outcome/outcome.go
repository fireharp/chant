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
}

// Outcome is the union shape emitted by chant subcommands with --json. Unset
// fields are omitted so each subcommand's payload stays focused.
type Outcome struct {
	Subcommand string `json:"subcommand"`

	// suggest
	MatchFound bool  `json:"match_found,omitempty"`
	Hits       []Hit `json:"hits,omitempty"`

	// run / verify
	RecipeID    string `json:"recipe_id,omitempty"`
	Version     int    `json:"version,omitempty"`
	Executed    bool   `json:"executed,omitempty"`
	ExitCode    int    `json:"exit_code"`
	VerifierRan bool   `json:"verifier_ran,omitempty"`
	// Trusted is the verifier-first verdict: true ONLY after a passing
	// verifier. A cache hit alone never sets this.
	Trusted    bool  `json:"trusted"`
	RuntimeMS  int64 `json:"runtime_ms,omitempty"`
	// LLMCallsAvoided is a reuse-win estimate carried on the recipe.
	LLMCallsAvoided int `json:"llm_calls_avoided,omitempty"`

	// capture
	Captured bool `json:"captured,omitempty"`

	// invalidate / status
	Stale bool `json:"stale,omitempty"`

	// shared
	BlockingError           bool     `json:"blocking_error,omitempty"`
	Message                 string   `json:"message,omitempty"`
	SuggestedCommands       []string `json:"suggested_commands,omitempty"`
	RecommendedNextCommand  string   `json:"recommended_next_command,omitempty"`
}
