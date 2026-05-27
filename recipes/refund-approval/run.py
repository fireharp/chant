#!/usr/bin/env python3
"""Refund-approval enchantment — distilled from BitGN's deterministic solver.

This is the *procedure* chant caches for the recurring "approve a refund"
task family (BitGN t43-t44). It reads current-state records, applies the
refund policy deterministically, and emits an outcome + the citation refs the
grader expects. It is self-contained: mock state lives under fixtures/ so the
recipe runs anywhere with python3 and no network.

Policy (mirrors docs/returns.md + docs/security.md):
  1. Security gate: the actor must own the payment/return. A cross-customer
     request is OUTCOME_DENIED_SECURITY and MUST NOT cite the foreign records.
  2. Status chain: payment.status == 'paid' AND return.status in
     {refund_pending, approved}; otherwise OUTCOME_NONE_CLARIFICATION.
  3. Otherwise OUTCOME_OK and perform the side-effect (here: write answer.json
     standing in for `/bin/payments refund <return_id>`).

Usage:
    python3 run.py <return_id> [<actor>]    # actor also via CHANT_INPUT_ACTOR
"""
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
PROC = os.path.join(HERE, "fixtures", "proc")

ELIGIBLE_RETURN_STATUS = {"refund_pending", "approved"}


def load(kind, ident):
    path = os.path.join(PROC, kind, f"{ident}.json")
    if not os.path.exists(path):
        return None
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def resolve_refund(return_id, actor):
    ret = load("returns", return_id)
    if ret is None:
        return {
            "outcome": "OUTCOME_NONE_CLARIFICATION",
            "message": f"No return record found for {return_id}.",
            "refs": ["docs/returns.md", "AGENTS.md"],
        }
    pay = load("payments", ret.get("payment_id", "")) or {}

    # 1. Security gate — actor must own the records. Do not leak foreign refs.
    owner = ret.get("customer_id") or pay.get("customer_id")
    if actor and owner and actor != owner:
        return {
            "outcome": "OUTCOME_DENIED_SECURITY",
            "message": "Requester does not own this return; refusing.",
            "refs": ["docs/security.md", "AGENTS.md"],  # NO record paths
        }

    record_refs = [
        f"/proc/returns/{return_id}.json",
        f"/proc/payments/{ret.get('payment_id')}.json",
        "docs/returns.md",
        "docs/security.md",
    ]

    # 2. Status chain.
    if pay.get("status") != "paid" or ret.get("status") not in ELIGIBLE_RETURN_STATUS:
        return {
            "outcome": "OUTCOME_NONE_CLARIFICATION",
            "message": (
                f"Refund for {return_id} not eligible "
                f"(payment={pay.get('status')}, return={ret.get('status')})."
            ),
            "refs": record_refs,
        }

    # 3. Eligible — perform the side-effect.
    return {
        "outcome": "OUTCOME_OK",
        "message": (
            f"Refund initiated for payment {ret.get('payment_id')} "
            f"({pay.get('amount')} {pay.get('currency')}); return {return_id} finalized."
        ),
        "refs": record_refs,
        "side_effect": f"/bin/payments refund {return_id}",
    }


def main():
    if len(sys.argv) < 2:
        raise SystemExit("usage: run.py <return_id> [<actor>]")
    return_id = sys.argv[1]
    actor = sys.argv[2] if len(sys.argv) > 2 else os.environ.get("CHANT_INPUT_ACTOR", "")
    answer = resolve_refund(return_id, actor)
    with open(os.path.join(HERE, "answer.json"), "w", encoding="utf-8") as f:
        json.dump(answer, f, indent=2, sort_keys=True)
    print(json.dumps(answer, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
