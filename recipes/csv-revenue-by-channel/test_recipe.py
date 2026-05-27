#!/usr/bin/env python3
"""Verifier for csv-revenue-by-channel.

A recipe hit is only a *candidate*. This verifier is what promotes a reuse
result to "trusted": it runs the procedure on a known example and asserts the
computed revenue-by-channel totals exactly. Exit 0 == verified.
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))

# Independent ground truth for examples/orders_shopify.csv.
EXPECTED = {"direct": 25.5, "facebook": 200.0, "google": 150.0}


def main():
    example = os.path.join(HERE, "examples", "orders_shopify.csv")
    subprocess.run([sys.executable, os.path.join(HERE, "run.py"), example], check=True)

    with open(os.path.join(HERE, "revenue_by_channel.json"), encoding="utf-8") as f:
        got = json.load(f)

    if got != EXPECTED:
        print(f"FAIL: expected {EXPECTED}, got {got}", file=sys.stderr)
        sys.exit(1)
    print("OK: revenue-by-channel totals match expected ground truth")


if __name__ == "__main__":
    main()
