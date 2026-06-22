# §11.4.55 Sweep Remediation Recipes — 2026-06-22

**Scope:** READ-ONLY remediation report for the 6 failing gates of
`scripts/verify-all-constitution-rules.sh` (G2 already fixed this session).
No source/governance files were edited or committed. (One transient G12
in-place regen was performed to MEASURE the diff, then immediately reverted —
working tree is back to its pre-investigation state apart from the
pre-existing parallel-session semgrep changes.)

**Live sweep verdict (this run):** `Gates run: 14 · Failures: 6`
**G2 now PASS** ("zero production bluff markers") — disproves the stale
"7-fail" figure; the live count is **6/14 failing**.

| Gate | Anchor | One-line verdict | Pre-existing? | Safe to auto-fix? |
|------|--------|------------------|---------------|-------------------|
| G1  | §11.9 + CONST-047 cascade | 213 cascade failures (63 PASS); the queued 62/64-repo fleet 142→166 backfill | pre-existing (queued) | NO — operator-scoped fleet cascade (do not run inline) |
| G7  | §11.4.83 docs/qa | 117 feature commits lack `docs/qa/<run-id>/` | pre-existing debt (116) + 1 this-session HEAD | PARTLY — `[no-qa-evidence]` is not retro-applicable; needs operator decision |
| G8  | §11.4.90 Obsolete-Details | 1 violation: HXC-044 `Reason` out-of-vocab + `Triple-check` token malformed | pre-existing (2026-06-09) | NO — closed-vocab Reason needs operator decision |
| G12 | §11.4.12 summary freshness | 2 summaries stale by ONLY the `Last regenerated` date stamp | pre-existing | YES — pure regen, no content change |
| G13 | §11.4.99 Sources-verified | 22/72 operator docs lack a `## Sources verified` footer | pre-existing (docs dated 06-14..06-22) | NO — §11.4.99 requires live WebFetch cross-ref, not a blind append |
| G14 | §11.4.106 docs_chain | 3 contexts STALE (issues/fixed/governance + new semgrep_status) | pre-existing + parallel-session | YES (mostly downstream of G12) — `docs_chain sync --all` |

---

## G1 — Governance cascade (§11.9 + CONST-047) — CONFIRMED queued fleet cascade

**Violation count:** 213 FAIL / 63 PASS (`/tmp/g1-cascade.out`, 317 KB).
**Distinct owned submodules failing:** 64, plus 6 root carriers
(`root/CLAUDE.md` lags by 1 anchor = §11.4.166; `AGENTS/QWEN/GEMINI` lag from
§11.4.159; `CRUSH/CONSTITUTION` lag from §11.4.142).

**Root cause:** the constitution submodule was bumped to `4ff4985`
(adds §11.4.157 GEMINI-lockstep, §11.4.166 semgrep). Per CONST-047 the new
anchor band §11.4.142→166 must propagate (verbatim or by ID) into every owned
submodule's CLAUDE/AGENTS/CONSTITUTION trio + the root carriers. None of the
207 owned carrier files (64 submodules) carry the band yet.

**Pre-existing?** PRE-EXISTING / queued. This is the cross-cutting fleet
cascade explicitly out-of-scope for this task. **NOT re-investigated** per
instructions — confirmed it is the 62-repo (here: 64-submodule) fleet
142→166 backfill.

**Fix recipe (operator-scoped — do NOT run inline):**
```bash
# Cascade the new anchor band into every owned carrier + root mirrors:
bash scripts/propagate-governance.sh          # root carriers (CLAUDE/AGENTS/QWEN/GEMINI/CRUSH/CONSTITUTION)
bash scripts/backfill_anchor_cascade.sh       # owned-submodule trio backfill (142→166)
bash scripts/verify-governance-cascade.sh     # re-verify → expect 0 failures
# then commit+push EACH owned submodule (no force, §11.4.113) + bump pointers.
```
**Effort:** LARGE (64 submodules × 3 files, each committed+pushed to all
upstreams, then pointer-bumped). **Safe to auto-fix?** NO — multi-repo,
multi-upstream, operator-gated.

---

## G7 — §11.4.83 docs/qa end-user evidence — 117 violations

**Violation count:** 117 (`/tmp/g7-qa.out`). Scope =
`ed84f90e..HEAD` (baseline 2026-05-28; **453** commits in scope).
**Sample (newest 3):**
- `ab5d2ad2` chore(governance): G2 smoke false-positive fix … (← THIS session, HEAD)
- `1771c855` fix(cli): add Xiaomi MiMo models to --list-models output
- `759e48c8` feat(llm): add ASR and TTS endpoint support to Xiaomi provider
Span: 2026-06-10 (`bd3adb7f`) → 2026-06-22 (`ab5d2ad2`).

**Root cause:** the gate heuristic flags any `feat(`/`fix(` commit touching
shippable code that has no matching `docs/qa/<run-id>/` directory. 116 of the
117 are PRE-EXISTING feature commits (Xiaomi provider, §11.4.118 discovery
rounds, HXC-NNN fixes) that landed without a QA transcript dir; 1 (`ab5d2ad2`)
is this session's governance commit that tripped the `chore(`+code heuristic.
`docs/qa/` exists with 61 run-id dirs — the tree is real, just not populated
for these 117 commits.

**Pre-existing?** YES — 116/117 pre-existing debt across 453 commits.

**Fix recipe (two legitimate paths per the gate's own message):**
1. For genuine feature commits → author the missing evidence:
   `mkdir -p docs/qa/<run-id>/ && <add bidirectional transcript + materials>`
   (one dir per shipped feature; see `docs/qa/README.md`). This is the
   §11.4.83-compliant path but is heavy (117 transcripts).
2. For non-feature changes that tripped the heuristic (e.g. `ab5d2ad2`
   governance) → the gate is `git`-history-based and the annotation is
   `[no-qa-evidence]` in the COMMIT MESSAGE. It is **not retro-applicable**
   to already-pushed commits without history rewrite (forbidden, §11.4.113).
   Realistic remediation: a one-time baseline bump —
   `ed84f90e..HEAD` → set the gate baseline to a recent commit so the
   pre-baseline debt is exempted (same mechanism the gate already uses), OR
   add a `docs/qa/<run-id>/` for each future feature going forward.

**Effort:** LARGE if authoring 117 transcripts; SMALL if operator approves a
baseline bump for the pre-existing cohort + commit-message discipline forward.
**Safe to auto-fix?** NO — requires operator decision on baseline-bump vs
117-transcript backfill; cannot fabricate transcripts (would itself be a bluff).

---

## G8 — §11.4.90 Obsolete-Details — 1 violation (HXC-044)

**Violation (`/tmp/g8-obs.out`):**
```
FAIL §11.4.90 [docs/Fixed.md] ## HXC-044 — internal/cognee: AMD-GPU rocm-smi
  JSON parser ... — missing/invalid sub-fact(s): Reason(closed-vocab) Triple-check
```
**Current line (docs/Fixed.md:294):**
```
**Obsolete-Details:** Since: 2026-06-09; Reason: not-reproducible; Superseding-item: none; Triple-check evidence: docs/qa/HXC-044/evidence.md
```

**Root cause (TWO defects, both mechanical):**
1. `Reason: not-reproducible` is NOT in the §11.4.90 closed vocabulary. The
   gate (`scripts/gates/obsolete_details_gate.sh:28,55`) accepts ONLY:
   `superseded-by-design-change | superseded-by-later-mandate | feature-removed |
   duplicate-of | unsupported-topology`.
2. The gate regex requires the literal token `Triple-check:` followed by
   content (`/Triple-check:[ ]*[^ ]/`, line 57). The doc writes
   `Triple-check evidence:` — a SPACE not a COLON after `Triple-check`, so it
   never matches. (Note: there is NO closed-vocab value meaning
   "not reproducible"; the item is conceptually a non-reproducible env artefact.)

**Pre-existing?** YES — added 2026-06-09 (`6f1c2ca5`), ~13 days old; the gate
has flagged it every run since.

**Fix recipe:** edit `docs/Fixed.md:294` (and mirror the parallel pipe-row at
`docs/Fixed.md:13`). The `Triple-check` token fix is unambiguous; the Reason
needs an operator/judgment mapping into the closed vocab. Closest fits:
- `duplicate-of` — NO (not a duplicate).
- `unsupported-topology` — DEFENSIBLE (the failure was a worktree/topology
  artefact, "10/10 main-tree PASS"; the closure note literally says
  "worktree env artifact").
Proposed compliant line (operator to confirm the Reason choice):
```
**Obsolete-Details:** Since: 2026-06-09; Reason: unsupported-topology; Superseding-item: none; Triple-check: 10/10 main-tree PASS at HEAD 54ab4e95 — see docs/qa/HXC-044/evidence.md
```
ALTERNATIVE (cleaner, but needs constitution change): petition to ADD
`not-reproducible` to the §11.4.90 closed vocab (the vocab is "open to
additions, never contraction" per the anchor) + the gate regex — that is a
constitution-submodule edit (CONST-049 workflow), heavier.

**Effort:** SMALL (one/two-line doc edit) once Reason is chosen.
**Safe to auto-fix?** NO — the Reason value is a judgment call (operator must
pick `unsupported-topology` vs a vocab extension). The `Triple-check:` token
fix alone is safe but insufficient.

---

## G12 — §11.4.12 summary freshness — 2 stale summaries

**Violation (`/tmp/g12-summary.out`):** both
`docs/Issues_Summary.md` (last regen 2026-06-16) and
`docs/Fixed_Summary.md` (last regen 2026-06-17) are STALE vs a fresh
regeneration. **Measured diff = exactly 1 line each — ONLY the
`*Last regenerated: <date> …*` stamp** (2026-06-16/17 → 2026-06-22). Zero
content/row changes. (Verified by running the generators to a temp + diff;
then reverted.)

**Root cause:** the generators stamp the regen date; the gate regenerates with
TODAY's date and byte-compares, so any day on which a tracker doc was touched
without re-running the generators leaves the stamp stale. After in-place
regen, `generate_{issues,fixed}_summary.sh --check` both report **PASS**.

**Pre-existing?** YES — stamps are 5-6 days old; trivial drift.

**Fix recipe (CONFIRMED resolves the gate):**
```bash
bash scripts/generate_issues_summary.sh      # in-place write (default mode)
bash scripts/generate_fixed_summary.sh       # in-place write (default mode)
# verify: both --check report "PASS — summary in sync"
bash scripts/generate_issues_summary.sh --check
bash scripts/generate_fixed_summary.sh --check
# then also regen exports (G14 depends on this — see below):
bash scripts/regenerate-tracker-exports.sh   # or `docs_chain sync --all --root .`
```
**Effort:** TRIVIAL (2 commands). **Safe to auto-fix?** YES — pure mechanical
projection, no content change beyond the date stamp.

---

## G13 — §11.4.99 Sources-verified footers — 22 missing

**Violation (`/tmp/g13-sv.out`):** 50/72 operator-facing docs footered (69%);
**22** lack a `## Sources verified <date>: <urls>` footer. Gate is
`--enforce` (100% required per HXC-030 closure). Sample of the 22:
`docs/CODEGRAPH.md`, `docs/OPENDESIGN.md`, all 11 `docs/helixtrack/*.md`
(API_REFERENCE, ARCHITECTURE, DATABASE_SCHEMA, DEPLOYMENT_GUIDE,
FEATURE_INVENTORY, GAP_ANALYSIS, IMPLEMENTATION_PLAN, SECURITY_GUIDE,
TEST_STRATEGY, UI_UX_FINDINGS, USER_MANUAL), 5 `docs/features/inventory/*.md`,
`docs/features/xiaomi-status_summary.md`, `docs/features/_status_header.md`,
`docs/plans/POWER_FEATURES_PORTING_PLAN.md`, `docs/SESSION_STATUS_2026-06-21.md`.

**Footer format (from compliant docs, e.g. docs/ARCHITECTURE.md:265):**
```
## Sources verified 2026-06-04: <url1>, <url2>, <repo cross-refs>
```

**Root cause:** these 22 operator-facing docs landed (06-14..06-22, all
TRACKED, all in PRIOR commits) without the §11.4.99 footer; the enforcing gate
flags them. Most are NEW doc subtrees (helixtrack/, features/inventory/,
CODEGRAPH/OPENDESIGN added 06-22 11:26 — predate the sweep).

**Pre-existing?** YES — all 22 are committed before this session's sweep run.

**Fix recipe (per §11.4.99 — NOT a blind append):** for EACH of the 22:
1. Identify the doc's authoritative sources (the libraries/services/repo state
   it documents). 2. WebFetch the LATEST official docs + cross-reference every
   instruction. 3. Append:
```
## Sources verified 2026-06-22: <fetched-source-urls>, <repo cross-refs>
```
4. Record negative findings explicitly. 5. Cite the footer in the commit.
For internal/repo-derived docs (helixtrack ARCHITECTURE/DATABASE_SCHEMA,
features/inventory) the "sources" are the repo files cross-referenced (the
ARCHITECTURE.md footer precedent lists `.gitmodules`, `go.mod`, `ls` outputs).

**Effort:** MEDIUM-LARGE (22 docs × WebFetch-cross-ref). Parallelisable by
subagent (one per doc cluster: helixtrack, features, top-level).
**Safe to auto-fix?** NO — §11.4.99 explicitly forbids footering from memory;
each requires a live source fetch + cross-reference. A blind
`echo '## Sources verified …'` append would itself be a §11.4.99 bluff.

---

## G14 — §11.4.106 docs_chain verify — 3-4 STALE contexts

**Violation (`/tmp/g14-verify.log`):**
```
fixed       STALE: [fixed_summary]
governance  STALE: [claude_html crush_html]
issues      STALE: [issues_html issues_pdf issues_summary summary_html summary_pdf]
```
(A direct `docs_chain verify --all` also shows
`semgrep_status STALE: [semgrep_status_html semgrep_status_pdf]` — from the
parallel-session untracked `semgrep_status.yaml` context.)

**Root cause:** content-hash drift across docs_chain contexts:
- `fixed`/`issues` summary nodes = DOWNSTREAM of G12 (same stale summaries) +
  their `.html`/`.pdf` exports never regenerated after the last tracker edit.
- `governance` (`claude_html`, `crush_html`) = CLAUDE.md/CRUSH.md changed
  (the §11.4.166 governance work) without re-exporting their HTML siblings.
- `semgrep_status` = new parallel-session context not yet synced.
Engine is available 3 ways: `docs_chain` on PATH (`~/.local/bin/docs_chain`),
sibling `../docs_chain/cmd/docs_chain`, or `.docs_chain/bin/` (absent).

**Pre-existing?** YES (summary + governance drift) + PARTLY parallel-session
(semgrep_status context).

**Fix recipe (the gate prints it verbatim):**
```bash
docs_chain sync --all --root .     # atomically propagates every context, updates state.json
docs_chain verify --all --root .   # re-verify → expect all in-sync (exit 0)
```
Order note: run G12's `generate_*_summary.sh` FIRST (so the summary SOURCE is
fresh), THEN `docs_chain sync --all` (so the html/pdf exports hash-match).
**Effort:** SMALL (2 commands, after G12). **Safe to auto-fix?** YES for the
issues/fixed/governance contexts (mechanical export regen). The
`semgrep_status` context belongs to the parallel semgrep session — coordinate
before syncing it (§11.4.84 quiescence).

---

## Quick-wins vs operator-gated

**Quick-win (safe to auto-fix now, ~5 min total):**
- **G12** → `generate_issues_summary.sh` + `generate_fixed_summary.sh` (in-place).
- **G14** → `docs_chain sync --all --root .` (run AFTER G12; coordinate
  semgrep_status with the parallel session).

**Needs operator decision / judgment:**
- **G8** → choose the closed-vocab Reason for HXC-044 (`unsupported-topology`
  proposed) OR petition to add `not-reproducible` to §11.4.90 vocab; then the
  one/two-line doc edit + `Triple-check:` token fix.
- **G7** → baseline-bump (exempt 116 pre-existing) vs author 117 transcripts.
- **G13** → 22 WebFetch-cross-referenced footers (subagent-parallelisable).
- **G1** → operator-scoped 64-submodule fleet cascade (propagate-governance.sh
  + backfill_anchor_cascade.sh + per-repo push), NOT run inline.

## Captured-evidence index
- Live sweep summary: `/tmp/sweep_full.out`
- Per-gate: `/tmp/g1-cascade.out` `/tmp/g7-qa.out` `/tmp/g8-obs.out`
  `/tmp/g12-summary.out` `/tmp/g13-sv.out` `/tmp/g14-verify.log`
- G12 fix proven: in-place regen → `--check` PASS on both, diff = date-stamp only.
- G14 engine confirmed at `~/.local/bin/docs_chain` + sibling `../docs_chain`.

*§11.4.6: every count/date/sample above is verbatim from captured command
output; nothing inferred. Read-only — no source/governance file edited; the
transient G12 regen was reverted (`git checkout -- docs/{Issues,Fixed}_Summary.md`).*
