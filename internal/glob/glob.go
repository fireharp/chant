// Package glob is a tiny path glob matcher supporting "*", "**", and "?".
// "*" matches within a path segment; "**" matches across segments. It is the
// same style used by coherence's ontology rules so recipe file signals behave
// predictably for users familiar with that tool.
package glob

import "strings"

// Match reports whether name matches the glob pattern.
func Match(pattern, name string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			// Collapse consecutive **.
			for len(pat) > 1 && pat[1] == "**" {
				pat = pat[1:]
			}
			if len(pat) == 1 {
				return true // trailing ** matches everything
			}
			// Try to match the remainder at each position.
			for i := 0; i <= len(name); i++ {
				if matchSegments(pat[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if !matchSegment(pat[0], name[0]) {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}

// matchSegment matches a single path segment with "*" and "?" wildcards.
func matchSegment(pat, s string) bool {
	// Iterative wildcard match with backtracking.
	var pi, si, star, mark int
	star = -1
	for si < len(s) {
		if pi < len(pat) && (pat[pi] == s[si] || pat[pi] == '?') {
			pi++
			si++
		} else if pi < len(pat) && pat[pi] == '*' {
			star = pi
			mark = si
			pi++
		} else if star != -1 {
			pi = star + 1
			mark++
			si = mark
		} else {
			return false
		}
	}
	for pi < len(pat) && pat[pi] == '*' {
		pi++
	}
	return pi == len(pat)
}
