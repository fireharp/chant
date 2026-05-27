#!/usr/bin/env python3
"""Compute ecommerce revenue by channel from a CSV-like export.

This is the *procedure* chant caches — not an answer. It is robust to the
common schema drift across exports: the channel and revenue columns are
discovered from a set of aliases, so the same recipe reuses across Shopify /
Stripe / custom exports without re-reasoning.

Usage:
    python3 run.py <orders.csv>        # path via argv, or CHANT_INPUT_INPUT env

Writes revenue_by_channel.json next to this script and prints it to stdout.
Stdlib only — no pandas, so `chant run`/`verify` work in any environment.
"""
import csv
import json
import os
import sys

CHANNEL_ALIASES = ["channel", "source", "utm_source"]
REVENUE_ALIASES = ["revenue", "amount", "price", "total"]


def pick(header, aliases):
    lower = {h.lower().strip(): h for h in header}
    for a in aliases:
        if a in lower:
            return lower[a]
    raise SystemExit(f"no column among {aliases} found in header {header}")


def compute(path):
    with open(path, newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        header = reader.fieldnames or []
        ch_col = pick(header, CHANNEL_ALIASES)
        rev_col = pick(header, REVENUE_ALIASES)
        totals = {}
        for row in reader:
            ch = (row.get(ch_col) or "").strip()
            if not ch:
                continue
            try:
                rev = float((row.get(rev_col) or "0").strip() or 0)
            except ValueError:
                rev = 0.0
            totals[ch] = round(totals.get(ch, 0.0) + rev, 2)
    return dict(sorted(totals.items()))


def main():
    path = sys.argv[1] if len(sys.argv) > 1 else os.environ.get("CHANT_INPUT_INPUT")
    if not path:
        raise SystemExit("usage: run.py <orders.csv>")
    if not os.path.isabs(path):
        # Resolve relative to this script so it works regardless of cwd.
        here = os.path.dirname(os.path.abspath(__file__))
        cand = os.path.join(here, path)
        path = cand if os.path.exists(cand) else path
    totals = compute(path)
    out_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "revenue_by_channel.json")
    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(totals, f, indent=2, sort_keys=True)
    print(json.dumps(totals, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
