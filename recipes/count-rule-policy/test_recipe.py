#!/usr/bin/env python3
"""Verifier for count-rule-policy — the trust gate for count-rule reuse.

Mirrors BitGN's fixtures.test.ts style: it asserts the rule fires (and the
exact-format token + the policy ref) on the in-date city-scoped query, and
that it does NOT fire on the out-of-date query or the no-city query against
a city-scoped policy. The procedure is reused across all three cases — only
inputs and CHANT_INPUT_DATE change, which is the whole point of caching the
template instead of the answer.
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
RUN = os.path.join(HERE, "run.py")
POLICY_2026_05_28 = os.path.join(
    HERE, "fixtures", "policies", "2026-05-28", "inventory-hold-salzburg.md"
)

# Case design — covers the three branches of the count-rule pipeline:
#   A: in-date, city-scoped policy fires → exact-format token.
#   B: out-of-date → no policy → bare integer count from catalog.
#   C: in-date, but no city argument against a city-scoped policy → does
#      NOT fire (the demo policy is scoped to Salzburg, so a query without
#      a city is broader than the policy's scope). Returns the bare total.
CASES = [
    {
        "name": "A: in-date city-scoped policy fires (Salzburg hold)",
        "args": ["Cordless-Drill", "Salzburg"],
        "env": {"CHANT_INPUT_DATE": "2026-05-28"},
        "expect_message": "<COUNT:0>",
        "expect_count": 0,
        "require_ref": POLICY_2026_05_28,
    },
    {
        "name": "B: out-of-date → no policy → bare count",
        "args": ["Cordless-Drill", "Salzburg"],
        "env": {"CHANT_INPUT_DATE": "2026-04-01"},
        # Salzburg Cordless-Drill rows in the catalog: 4 + 3 = 7.
        "expect_message": "7",
        "expect_count": 7,
        "forbid_ref_substring": "2026-05-28",
    },
    {
        "name": "C: in-date, no city, policy is city-scoped → does NOT fire",
        "args": ["Cordless-Drill"],
        "env": {"CHANT_INPUT_DATE": "2026-05-28"},
        # All Cordless-Drill rows: 4 + 3 + 5 + 2 = 14.
        "expect_message": "14",
        "expect_count": 14,
        "forbid_ref_substring": "2026-05-28",
    },
]


def run_case(c):
    env = os.environ.copy()
    env.update(c["env"])
    out = subprocess.run(
        [sys.executable, RUN, *c["args"]],
        capture_output=True, text=True, env=env, check=True,
    )
    ans = json.loads(out.stdout)

    if ans.get("message") != c["expect_message"]:
        return (
            f"{c['name']}: expected message={c['expect_message']!r}, "
            f"got {ans.get('message')!r}"
        )
    if ans.get("count") != c["expect_count"]:
        return (
            f"{c['name']}: expected count={c['expect_count']}, "
            f"got {ans.get('count')}"
        )

    refs = ans.get("refs", [])
    if c.get("require_ref") and c["require_ref"] not in refs:
        return (
            f"{c['name']}: required ref missing from refs: {c['require_ref']}"
        )
    if c.get("forbid_ref_substring"):
        leaked = [r for r in refs if c["forbid_ref_substring"] in r]
        if leaked:
            return (
                f"{c['name']}: forbidden substring "
                f"{c['forbid_ref_substring']!r} appeared in refs: {leaked}"
            )

    # Exact-format invariant — when the rule fires, the message must match
    # the BitGN grader's `<COUNT:%d>` regex; when it does not fire, it must
    # be a bare integer with no angle brackets.
    if c["expect_message"].startswith("<COUNT:"):
        if not (ans["message"].startswith("<COUNT:") and ans["message"].endswith(">")):
            return f"{c['name']}: exact-format token missing in message {ans['message']!r}"
    else:
        if "<" in ans["message"] or ">" in ans["message"]:
            return f"{c['name']}: bare-integer case leaked exact-format token: {ans['message']!r}"

    return None


def main():
    failures = [msg for c in CASES if (msg := run_case(c))]
    if failures:
        for f in failures:
            print("FAIL:", f, file=sys.stderr)
        sys.exit(1)
    print(f"OK: all {len(CASES)} count-rule cases pass (exact-format + ref invariants)")


if __name__ == "__main__":
    main()
