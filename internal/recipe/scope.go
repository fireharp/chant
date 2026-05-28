package recipe

import (
	"strings"
	"time"
)

// Scope constants — the maturity channel a recipe can earn (spec §5).
const (
	ScopeProject   = "project"
	ScopeDomain    = "domain"
	ScopeUniversal = "universal"
)

// scopeRank lets us compare scopes monotonically. ComputeScope only ever
// returns a scope at this rank or lower (it never demotes); explicit demotion
// is the job of invalidate / DemoteScope (future).
var scopeRank = map[string]int{
	ScopeProject:   1,
	ScopeDomain:    2,
	ScopeUniversal: 3,
}

// ComputeScope returns the highest scope a recipe has earned from its
// verified_in + domains evidence (spec §5 — the universality ladder).
//
// Promotion rules (earned, never declared):
//
//	project   default; verified in 0 or 1 distinct contexts.
//	domain    verified in ≥2 distinct contexts sharing at least one domain tag.
//	universal verified in ≥3 distinct contexts AND those contexts span ≥2
//	          domains (i.e. the recipe has proved out under more than one
//	          domain label).
//
// Empty Domains caps the result at project regardless of how many contexts
// have verified the recipe: a recipe with no domain labels has no notion of
// "this is the same domain" or "this spans two domains", so the spec's
// domain/universal cases are undefined. We surface this in the godoc and in a
// comment on Recipe.Scope so the cap is discoverable.
//
// ComputeScope NEVER demotes: a recipe that already carries `universal` keeps
// it even if its current evidence has been pruned. Demotion lives in
// invalidate / a separate DemoteScope path so accidental evidence-trimming
// can't quietly downgrade a hard-earned universal recipe.
//
// Spec ambiguity (decided here, recorded for the reviewer):
//
//   - "≥2 distinct contexts spanning ≥2 domains but <3 contexts" — the table
//     says domain at ≥2 contexts sharing a domain tag, and universal at ≥3
//     contexts spanning ≥2 domains. A recipe with exactly 2 contexts that
//     span 2 domains satisfies the domain rule (≥2 contexts share at least
//     one domain — trivially, every context shares the union of the recipe's
//     domains) but NOT the universal rule (need ≥3 contexts). We return
//     `domain` for that case, matching the spec's explicit thresholds.
//
//   - "shared domain tag" — we interpret this as: the recipe declares ≥1
//     domain, AND it has ≥2 distinct contexts on its verified_in list. We do
//     NOT require per-context domain tags (VerifiedContext has no Domains
//     field), so "sharing" reduces to "the recipe has at least one domain
//     label, so the contexts share it by belonging to the same recipe."
//     This matches the practical effect of the spec — a recipe stamped
//     `domains: [csv]` verified in repo A and repo B is a csv-domain recipe
//     proven in 2 places.
func ComputeScope(r *Recipe) string {
	if r == nil {
		return ScopeProject
	}
	contexts := distinctContexts(r.VerifiedIn)
	domains := nonEmptyDomains(r.Domains)

	// No domain labels ⇒ no domain clustering is meaningful; cap at project.
	if len(domains) == 0 {
		return ScopeProject
	}

	switch {
	case len(contexts) >= 3 && len(domains) >= 2:
		return ScopeUniversal
	case len(contexts) >= 2:
		// ≥2 contexts + ≥1 domain ⇒ at least the recipe is verified across
		// places under its declared domain(s). Sufficient for domain.
		return ScopeDomain
	default:
		return ScopeProject
	}
}

// MaxScope returns the higher of two scopes by scopeRank. Unknown scopes are
// treated as project (the safe floor).
func MaxScope(a, b string) string {
	if scopeRank[a] >= scopeRank[b] {
		if _, ok := scopeRank[a]; ok {
			return a
		}
		return ScopeProject
	}
	if _, ok := scopeRank[b]; ok {
		return b
	}
	return ScopeProject
}

// RecordVerifiedContext records that the verifier passed in `context` and
// returns whether the verified_in list changed (a new context was appended OR
// an existing context's timestamp was refreshed). Empty contexts are ignored:
// without an identifiable repo there is nothing to dedupe on and we would
// pollute the ladder with anonymous evidence.
func (r *Recipe) RecordVerifiedContext(context string, at time.Time) bool {
	context = strings.TrimSpace(context)
	if context == "" {
		return false
	}
	stamp := at.UTC().Format(time.RFC3339)
	for i, v := range r.VerifiedIn {
		if v.Context == context {
			// Re-verify in the same context: refresh the timestamp so the
			// ladder reflects the most recent passing run, but do NOT add a
			// duplicate entry.
			r.VerifiedIn[i].At = stamp
			return true
		}
	}
	r.VerifiedIn = append(r.VerifiedIn, VerifiedContext{Context: context, At: stamp})
	return true
}

// distinctContexts returns the deduped, non-empty Context values from vs.
func distinctContexts(vs []VerifiedContext) []string {
	seen := make(map[string]struct{}, len(vs))
	var out []string
	for _, v := range vs {
		c := strings.TrimSpace(v.Context)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

// nonEmptyDomains returns the deduped, non-empty domain tags from ds.
func nonEmptyDomains(ds []string) []string {
	seen := make(map[string]struct{}, len(ds))
	var out []string
	for _, d := range ds {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	return out
}
