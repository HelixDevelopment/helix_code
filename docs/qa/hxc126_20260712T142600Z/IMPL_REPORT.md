# HXC-126 DB-owner implementation report

Status: DONE (data fix + permanent guard landed, uncommitted, ready for conductor review/commit/push/close).

## STEP 1 — Reproduce + characterize

Query used (per task brief):
```sql
SELECT atm_id,status,current_location FROM items
WHERE current_location='Issues'
  AND (status LIKE '%→ Fixed.md%' OR status LIKE 'Obsolete%');
```
Result: **41 items** (not 11 as the ticket text estimated), all genuinely
terminal (`Fixed (→ Fixed.md)`, `Implemented (→ Fixed.md)`, `Completed
(→ Fixed.md)` — no `Obsolete` items were in the leaked set). None were
`Reopened`/`Queued`/`Operator-blocked` (confirmed via a negative-filter query).

Full id list (41): HXA-001, HXA-002, HXA-003, HXC-001, HXC-002, HXC-003,
HXC-004, HXC-005, HXC-006, HXC-007, HXC-008, HXC-009, HXC-010, HXC-013,
HXC-014, HXC-014b, HXC-015, HXC-016, HXC-017, HXC-018, HXC-019, HXC-022,
HXC-023, HXC-024, HXC-025, HXC-026, HXC-027, HXC-028, HXC-029, HXC-030,
HXC-032, HXC-033, HXC-034, HXC-035, HXC-036, HXL-001, HXL-002, HXQ-001,
HXV-002, PAN-001, VEN-001.

Leak confirmed at the MD layer: `grep -c "^## <id> " docs/Issues.md` > 0 for
all 41 (RED baseline: 41/41 leaked), 0/41 present as an H2 heading in
docs/Fixed.md (`grep -c "^## <id> "` — but see the important nuance below).

**Forensic nuance discovered mid-investigation**: 30 of the 41 items already
had a *second*, pre-existing DB row for the same `atm_id` at
`current_location='Fixed', representation='table'` — a legacy pipe-table
closure row already rendered in `docs/Fixed.md` (confirmed by direct grep,
e.g. `docs/Fixed.md:69` for HXA-001). Their leaked Issues.md entry was a
`representation='section'` H2 "tombstone" (some, e.g. HXC-003, explicitly
labelled themselves "CLOSED (migrated to docs/Fixed.md) ... Section retained
as a migration tombstone per §11.4.19" in their own prose — a *deliberate*
historical practice that is exactly the HXC-126 defect). The other 11
(HXC-013/014/014b/015/018/019/024–028) had no pre-existing Fixed-side row at
all — for those the `section` representation was the item's *only* record.
Both classes are `current_location='Issues'` with terminal status and both
leak identically; the fix (below) is identical for both and is a
**precedented, schema-supported state**: `HXC-044` already lives in the DB
with BOTH `section` and `table` representations *co-located in Fixed*
(verified: `HXC-044|Fixed|section|...` and `HXC-044|Fixed|table|...`), proving
dual-representation-both-in-Fixed is a normal, already-existing system state,
not a novel one this fix invents.

Pre-fix DB blob (git HEAD): `6ef8a93d028be9e6c0cab2596a2c3822652d087f`.
Pre-fix `validate` baseline: `validate: OK — 335 items, all invariants
satisfied` (confirms `validate` does NOT cross-check status↔location, exactly
as briefed — it stayed green while the leak existed).

## STEP 2 — Data fix

**Tool-mechanism investigation (why raw SQL was used, justified):**
- `update --id <ID> --location Fixed` was tried first per the brief's suggested
  invocation, but reading `mutate.go` (`updateCmd`) shows `--location` is used
  **only as the item's current/source location to load AND to scope the
  `WHERE current_location=?` of the UPDATE** — it cannot *move* an item; a
  `--location Fixed` on an Issues-resident item returns `item not found in
  Fixed` (exit 1). Not usable.
- `close <id> --status <s> --evidence <p>` (the "atomic Issues→Fixed closure"
  subcommand) *does* move location, but `closeCmd` **regenerates body_md from
  columns via `renderItemBody`**, discarding the original freeform authored
  body — the exact truncation class the tool's own comments warn about
  (SPK-481 10 KB → 294 bytes). Given the brief's explicit "do NOT alter
  status, title, body" constraint and that several of these items carry
  multi-paragraph investigative narratives, `close` was unsafe to use here.
- `reopen --location <dest>` (mutate.go `reopenCmd`) contains the **exact
  lossless pattern needed**: a single `UPDATE items SET current_location=?
  WHERE atm_id=? AND current_location=? AND representation=?` (the code
  comment states verbatim: *"an UPDATE that flips current_location is
  LOSSLESS (it preserves forensic_anchor / closure_criteria / composes_with /
  parent_atm_id / session_ref / version_tags — columns a delete+insert would
  drop)"*), paired with `removeItemSegment` + `appendSegment` to move the
  matching `doc_segments` row. It is not directly invocable without also
  forcing `status='Reopened'`, which is not acceptable here.

**Justified raw-SQL mechanism used**: replicated `reopen`'s own internal
lossless-relocate pattern verbatim (same UPDATE shape, same
`doc_segments` move via DELETE+INSERT mirroring `removeItemSegment`/
`appendSegment`, same `item_history` audit-row shape used by `update`'s
`recordHistory(tx, id, "Updated", "AI", "", "")` call) — via a generated SQL
script (`/tmp/hxc126_work/fix_location.sql`, one transaction, 41×4
statements), executed with `sqlite3 docs/workable_items.db < fix_location.sql`.
Per item:
```sql
UPDATE items SET current_location='Fixed', last_modified=datetime('now')
  WHERE atm_id='<ID>' AND current_location='Issues' AND representation='section';
DELETE FROM doc_segments WHERE document='Issues' AND kind='item' AND atm_id='<ID>' AND representation='section';
INSERT INTO doc_segments (document, seq, kind, atm_id, representation, raw)
  VALUES ('Fixed', (SELECT COALESCE(MAX(seq),-1)+1 FROM doc_segments WHERE document='Fixed'), 'item', '<ID>', 'section', NULL);
INSERT INTO item_history (atm_id, event_type, by, on_date, reason, evidence_path)
  VALUES ('<ID>', 'Updated', 'AI', date('now'),
          'HXC-126 location-desync repair: current_location Issues to Fixed (status already terminal; item was leaking into open-issues tracker)',
          '/tmp/hxc126_report.md');
```
ONLY `current_location`, `last_modified`, the moved `doc_segments` row, and
the append-only `item_history` audit trail were touched. `status`, `title`,
`description`, `body_md`, `type`, `severity`, `created_by`, `assigned_to`,
`forensic_anchor`, `closure_criteria`, `composes_with`, `destination`,
`logic_group` — all byte-identical (verified: `validate` still passes, and
the md⟷db round-trip sync gate — which is byte-diff-based — still PASSes).

Pre-op backup: `/tmp/hxc126_work/workable_items.db.pre-fix-backup` (md5sum
matched the tracked DB before the fix, per §9.2).

Then: `workable-items export --db docs/workable_items.db --out-dir docs`
(into `docs/`, per the documented tool gotcha — verified no stray root-level
Issues.md/Fixed.md were created) → `sqlite3 docs/workable_items.db "PRAGMA
wal_checkpoint(TRUNCATE);"`.

## STEP 3 — Permanent guard (§11.4.135)

- `scripts/gates/no_terminal_item_in_issues_gate.sh` (new, executable,
  `bash -n` clean). Two layers: (1) an MD-level, DB-independent oracle that
  parses every `## <ID> ...` heading in `docs/Issues.md` and its first
  `**Status:**` line, FAILing if any status matches the terminal closed-set
  (`Fixed|Implemented|Completed (→ Fixed.md)` or `Obsolete...`) — this is the
  authoritative check and needs no Go/sqlite3; (2) a defense-in-depth DB-level
  cross-check (builds the workable-items binary, reads a **temp copy** of the
  tracked DB — never opens it in place, mirroring
  `workable_items_sync_gate.sh`'s WAL-mutation discipline — and queries
  `current_location='Issues'` + terminal status directly), honestly
  `SKIP`-noting (not failing) when go/sqlite3/the binary source are
  unavailable.
- `scripts/tests/no_terminal_item_in_issues_meta_test.sh` (new, executable,
  `bash -n` clean) — the standing §1.1 paired-mutation proof, matching the
  existing `workable_items_sync_meta_test.sh` pattern exactly (backup →
  mutate → assert FAIL → restore via `trap` → assert PASS).

**§1.1 self-mutation proof (ad hoc run during development, then the permanent
meta-test re-ran it and is now in the tree for CI/manual re-runs):**
```
=== mutated run (should FAIL) ===
CM-NO-TERMINAL-ITEM-IN-ISSUES: FAIL — 1 item(s) with a terminal §11.4.15
status are still rendered as open headings in docs/Issues.md (leaked into the
open-issues tracker): HXC-META-MUTATION
exit=1
=== restore ===
restore: byte-identical
=== restored run (should PASS) ===
CM-NO-TERMINAL-ITEM-IN-ISSUES: PASS — zero terminal-status items leaked into docs/Issues.md
exit=0
SELF-MUTATION PROOF: PASS (mutated FAILED, restored PASSED)
```
The mutation was NOT left in the tree — `docs/Issues.md` was byte-restored
from the pre-mutation backup and re-verified identical
(`diff -q` clean) both in the ad hoc run and inside the permanent meta-test's
own `trap restore EXIT INT TERM`.

## STEP 4 — Verify (all captured)

```
=== FINAL VERIFY 1: workable-items validate ===
validate: OK — 335 items, all invariants satisfied

=== FINAL VERIFY 2: sync gate ===
CM-WORKABLE-ITEMS-MD-DB-IN-SYNC: PASS — committed DB validates + md⟷db byte-identical in sync

=== FINAL VERIFY 3: new gate ===
CM-NO-TERMINAL-ITEM-IN-ISSUES: PASS — zero terminal-status items leaked into docs/Issues.md

=== FINAL VERIFY 4: new gate meta-test ===
=== no_terminal_item_in_issues_gate §1.1 paired-mutation meta-test ===
  PASS: baseline gate PASS (zero terminal items leaked into Issues.md) (exit 0)
  PASS: mutated (terminal-status-in-Issues) gate FAILS (exit 1)
  PASS: restored gate PASS again (exit 0)
---
Passed: 3  Failed: 0
PASS: gate genuinely detects a terminal-status item leaked into Issues.md (§1.1 honoured)

=== FINAL VERIFY 5: reproduce query on live DB (must be 0) ===
0

=== FINAL VERIFY 6: no stray root Issues.md/Fixed.md ===
ls: cannot access 'Issues.md': No such file or directory
ls: cannot access 'Fixed.md': No such file or directory
```

Also individually confirmed: 0/41 ids remain in `docs/Issues.md` (`grep -c
"^## <id> "`), 41/41 now present as `## <id>` headings in `docs/Fixed.md`.
HXC-126 itself was NOT touched (`current_location='Issues'`, `status='Queued'`
still — confirmed via direct DB query and grep of its own heading/body).

## git status --short (working tree, uncommitted)

```
 M docs/Fixed.docx
 M docs/Fixed.html
 M docs/Fixed.md
 M docs/Fixed.pdf
 M docs/Fixed_Summary.docx
 M docs/Fixed_Summary.html
 M docs/Fixed_Summary.md
 M docs/Fixed_Summary.pdf
 M docs/Issues.docx
 M docs/Issues.html
 M docs/Issues.md
 M docs/Issues.pdf
 M docs/Issues_Summary.docx
 M docs/Issues_Summary.html
 M docs/Issues_Summary.md
 M docs/Issues_Summary.pdf
 M docs/workable_items.db
?? scripts/gates/no_terminal_item_in_issues_gate.sh
?? scripts/tests/no_terminal_item_in_issues_meta_test.sh
```
(All other `M`/`??` entries visible in a bare `git status --short` — e.g.
`dependencies/LLama_CPP`, `docs/audit/bypass_events.md`, `submodules/helix_qa`,
`.git-backups/`, `.superpowers/`, `docs/qa/*`, `scratchpad/*` — pre-existed
before this session started per the conductor's own pre-task `gitStatus`
snapshot and were NOT touched by this work.)

No source code, no `constitution/` files, and no push were touched. HXC-126
itself was left `Queued`/open in the DB for the conductor to close.

## Nothing flagged as unsafe

All 41 flips were mechanically uniform (same representation='section',
same current_location='Issues', same terminal-status shape, none
Reopened/Queued/Operator-blocked, none already present as a `section`-rep row
in Fixed) — no item required manual judgment or was skipped.
