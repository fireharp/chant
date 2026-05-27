#!/usr/bin/env python3
"""Verifier for refund-approval — the trust gate for refund reuse.

Mirrors BitGN's fixtures.test.ts: it asserts the policy invariants across
several cases, not just one happy path. A reuse is trusted only if ALL hold:
  - eligible return     -> OUTCOME_OK
  - ineligible status   -> OUTCOME_NONE_CLARIFICATION
  - cross-customer      -> OUTCOME_DENIED_SECURITY AND no foreign record refs
The security no-leak invariant is the one most worth locking down.
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
RUN = os.path.join(HERE, "run.py")

CASES = [
    {"name": "eligible refund", "return_id": "ret_200", "actor": "cust_alice",
     "expect_outcome": "OUTCOME_OK"},
    {"name": "rejected return", "return_id": "ret_201", "actor": "cust_alice",
     "expect_outcome": "OUTCOME_NONE_CLARIFICATION"},
    {"name": "cross-customer (security)", "return_id": "ret_200", "actor": "cust_bob",
     "expect_outcome": "OUTCOME_DENIED_SECURITY", "forbid_record_refs": True},
]


def run_case(c):
    out = subprocess.run(
        [sys.executable, RUN, c["return_id"], c["actor"]],
        capture_output=True, text=True, check=True,
    )
    ans = json.loads(out.stdout)
    if ans["outcome"] != c["expect_outcome"]:
        return f"{c['name']}: expected {c['expect_outcome']}, got {ans['outcome']}"
    if c.get("forbid_record_refs"):
        leaked = [r for r in ans.get("refs", []) if r.startswith("/proc/")]
        if leaked:
            return f"{c['name']}: SECURITY LEAK — cited foreign records {leaked}"
    return None


def main():
    failures = [msg for c in CASES if (msg := run_case(c))]
    if failures:
        for f in failures:
            print("FAIL:", f, file=sys.stderr)
        sys.exit(1)
    print(f"OK: all {len(CASES)} refund-policy cases pass (incl. security no-leak)")


if __name__ == "__main__":
    main()
