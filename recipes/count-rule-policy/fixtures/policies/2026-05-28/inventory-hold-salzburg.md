# Inventory hold — Salzburg, 2026-05-28

**Effective date:** 2026-05-28
**Kind:** Cordless-Drill
**City:** Salzburg
**Status:** ON HOLD

The Cordless-Drill inventory at the Salzburg store is on hold pending an
audit by the regional ops team. While this notice is in effect, report **0**
for any count query that targets `kind=Cordless-Drill` AND `city=Salzburg`.

This rule is **scoped to Salzburg** — Cordless-Drill counts in other cities
follow the normal `fixtures/products.csv` totals. Count queries for other
kinds in Salzburg also follow the normal totals.

When this rule fires, the answer message MUST use the exact-format token
`<COUNT:%d>` (e.g. `<COUNT:0>`) so the grader's regex picks it up. When no
rule fires, return the bare integer.
