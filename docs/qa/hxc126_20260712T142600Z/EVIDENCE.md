# HXC-126 — QA evidence (§11.4.83)

**Item:** HXC-126 (Task / Medium) — closed items still appear in the open-issues tracker instead of the resolved tracker
**DB:** `docs/workable_items.db` (§11.4.93/.95 SSoT, git-tracked)
**Date (UTC):** 2026-07-12T14:26:00Z
**Closure vocab:** Completed (§11.4.33, Task)
**Discipline:** §11.4.102 systematic-debugging, §11.4.115 reproduce-first, §11.4.135 permanent guard, §11.4.142/§11.4.134 independent review, §11.4.34 audit, §11.4.60/.65 export-sync.

## Root cause (Phase 1 — FACT, reproduced)

**41 items** (the ticket estimated 11) had a TERMINAL status (`Fixed/Implemented/Completed (→ Fixed.md)`
or `Obsolete...`) but `current_location='Issues'`, so `workable-items export` rendered them into
`docs/Issues.md` (the OPEN tracker) — a leak. IDs: HXA-001/002/003, HXC-001–010, HXC-013–019,
HXC-022–030, HXC-032–036, HXL-001/002, HXQ-001, HXV-002, PAN-001, VEN-001. Legacy closures set the
terminal STATUS but never flipped `current_location`; `validate` had no status↔location cross-check,
so the inconsistency passed silently.

## Fix

**(1) Data (lossless location flip).** For exactly the 41 terminal items, `current_location` Issues→Fixed
+ `doc_segments` moved to `document='Fixed'` + one `item_history` audit row each — via a raw-SQL
transaction replicating the tool's own documented-LOSSLESS `reopen` pattern (the tool's
`update --location` only scopes reads and `close` would rewrite `body_md`, so neither was safe). NO
status/type/title/body/severity change; NO non-terminal item touched; the 6 legitimately-Queued items
in Issues left untouched.

**(2) Permanent guard (§11.4.135, project-level).** `scripts/gates/no_terminal_item_in_issues_gate.sh`
(two-layer: MD-heading oracle + DB cross-check with honest SKIP) + paired meta-test
`scripts/tests/no_terminal_item_in_issues_meta_test.sh` (§1.1: plants a terminal phantom → gate FAILs →
restore → PASS, zero residue). Prevents recurrence.

## Independent review (§11.4.142) — dump-diff verified, then 2 conductor-side fixes

The reviewer's rigorous `.dump` diff of `git show HEAD:docs/workable_items.db` vs the working DB
(keyed on the true composite PK `(atm_id,current_location,representation)`) confirmed **exactly 41 flips
with only `last_modified` changing on those rows**; every other column and all 294 other rows byte-
identical; `doc_segments` delete-41/insert-41 to `Fixed`; `item_history` +41 audit rows (170 prior
untouched); no other table changed; validate OK; sync gate PASS; the new gate genuinely FAILs on a
planted phantom (Layer-2 adversarially exercised by swapping the real pre-fix DB → gate named the exact
41 ids → restored byte-identical).

Two review findings, both conductor-remediated:
- **BLOCKING (export sync §11.4.60/.65):** `docs/Issues.{html,pdf,docx}` were stale vs `Issues.md`
  (10 of 41 still rendered). Fixed by re-running `export --db docs/workable_items.db --out-dir docs`
  so all formats regenerate atomically from the DB; verified every `Issues.*`/`Fixed.*` export mtime ≥
  its `.md`.
- **Nit (§11.4.34 durable evidence):** the 41 `item_history.evidence_path` pointed to non-committed
  `/tmp/hxc126_report.md`; updated to this durable committed path
  (`docs/qa/hxc126_20260712T142600Z/EVIDENCE.md`).

## Captured verification (post-remediation)

```
sqlite3 ... WHERE current_location='Issues' AND terminal-status  -> 0
0 of 41 in docs/Issues.md ; 41 of 41 in docs/Fixed.md
workable-items validate -> OK (335 items)
workable_items_sync_gate.sh -> PASS (md⟷db byte-identical)
no_terminal_item_in_issues_gate.sh -> PASS ; meta-test FAIL→restore→PASS (zero residue)
all docs/Issues.* and docs/Fixed.* export siblings mtime >= their .md
```

The impl subagent's full run report is at `IMPL_REPORT.md` in this directory.
