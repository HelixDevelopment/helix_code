# Post-Pull Constitution Validation (§11.4.55) + Fleet Propagation State — 2026-06-22

**Scope:** READ-ONLY evidence report. No files were edited or committed.
**Working dir:** `/Volumes/T7/Projects/helix_code`
**Constitution submodule HEAD:** `4ff4985a1a2f3066ddb271eda0c42dc1f03460c1` (`helix_translate-2.3.1-23-g4ff4985`)
  — confirmed source-of-truth contains `§11.4.166` (Universal Semgrep static-analysis mandate, 1 hit in `constitution/Constitution.md`) and `§11.4.157` (GEMINI lockstep, 5 hits). Highest anchor in `constitution/Constitution.md` = **11.4.166**.
**§11.4.6 note:** every figure below is verbatim from captured command output; nothing inferred.

---

## STEP 1 — Does the §11.4.55 sweep script exist? YES — and it ran fully.

`scripts/verify-all-constitution-rules.sh` is present (`-rwxr-xr-x  26362 bytes  Jun 9 10:33`).
(The original 180s run was killed by `timeout` mid-G11; re-run with 540s timeout completed.)

### Captured verdict (`bash scripts/verify-all-constitution-rules.sh`)

```
=== verify-all-constitution-rules.sh summary ===
Gates run: 14
Failures:  7
  G1  FAIL  cascade verifier reported failures (see /tmp/g1-cascade.out)
  G2  FAIL  production code contains bluff markers
  G3  PASS  no production code imports internal/mocks
  G4  PASS  no nested own-org submodule chains in any owned submodule
  G5  PASS  every owned submodule has .gitignore + no sensitive files tracked at owned-paths
  G6  PASS  all top-level directories snake_case OR protected (0 protected)
  G7  FAIL  feature commit(s) lack docs/qa/<run-id>/ evidence (see /tmp/g7-qa.out)
  G8  FAIL  Obsolete item(s) missing/invalid Obsolete-Details (see /tmp/g8-obs.out)
  G9  PASS  every summary one-liner is self-contained (no anti-pattern rows)
  G10  PASS  no multi-platform script drops a manifest platform without honest-gap citation
  G11  PASS  PASS — committed DB validates + md⟷db byte-identical in sync
  G12  FAIL  summary docs stale vs Issues.md/Fixed.md — re-run scripts/generate_{issues,fixed}_summary.sh
  G13  FAIL  operator-facing doc(s) lack a §11.4.99 Sources-verified footer (see /tmp/g13-sv.out)
  G14  FAIL  docs_chain verify --all reported drift or error — run: docs_chain sync --all --root .

FAIL: 7 gate(s) violate the constitution
```

**Overall §11.4.55 sweep verdict: FAIL (7 of 14 gates failing).**

### Gate details captured

- **G1** FAIL — wraps the cascade verifier (see STEP 2; 213 failures).
- **G2** FAIL — `production code contains bluff markers`; canonical fix cites `helix_code/cmd/cli/main.go — replace simulation with real implementation`.
- **G7** FAIL — `RESULT: FAIL (enforcing — 116 violation(s))` — feature commits lacking `docs/qa/<run-id>/` evidence.
- **G8** FAIL — `0 compliant Obsolete item(s), 1 violation(s)`; the offending item: `docs/Fixed.md ## HXC-044 — internal/cognee: AMD-GPU rocm-smi JSON parser ... missing/invalid sub-fact(s): Reason(closed-vocab) Triple-check`.
- **G12** FAIL — Issues/Fixed summary docs stale.
- **G13** FAIL — operator-facing docs missing §11.4.99 Sources-verified footer.
- **G14** FAIL — docs_chain verify --all drift/error.

> Note for the task framing: the task hypothesised the sweep script may be absent ("not implemented here"). That hypothesis is **disproven** — the script exists and is the live enforcement engine; it returns FAIL.

---

## STEP 2 — `verify-governance-cascade.sh` (extended to anchors 142→166)

`scripts/verify-governance-cascade.sh` is present (`-rwxr-xr-x  10306 bytes  Jun 22 11:24`).

### Captured summary

```
=== Result: 213 failures ===
FAIL
```

Total lines 290 · PASS lines 63 · FAIL lines 214 (213 distinct failures + header).
Sections: `Root governance`, `Root govfiles — covenant-114 propagation (§11.4.69, §11.4.75..§11.4.166)`, `Owned-by-us submodules`, `Third-party submodules`.

### Root-carrier propagation lag (which anchors each root govfile is MISSING)

```
PASS: root/CONSTITUTION.md (governance presence)   PASS: root/AGENTS.md (governance presence)
FAIL: root/CLAUDE.md       — missing: §11.4.166
FAIL: root/AGENTS.md       — missing: §11.4.159 §11.4.160 §11.4.161 §11.4.162 §11.4.163 §11.4.164 §11.4.165 §11.4.166
FAIL: root/QWEN.md         — missing: §11.4.159 .. §11.4.166  (8 anchors)
FAIL: root/GEMINI.md       — missing: §11.4.159 .. §11.4.166  (8 anchors)
FAIL: root/CRUSH.md        — missing: §11.4.142 .. §11.4.166  (25 anchors)
FAIL: root/CONSTITUTION.md — missing: §11.4.142 .. §11.4.157, §11.4.159 .. §11.4.166  (24 anchors; carrier propagation check)
```

So the root carriers themselves are CLOSEST to current — root `CLAUDE.md` lags by exactly ONE anchor (§11.4.166, the just-pulled semgrep mandate). AGENTS/QWEN/GEMINI lag from §11.4.159; CRUSH lags from §11.4.142.

### Owned-by-us submodule lag

```
owned-submodule carrier files PASS: 0
owned-submodule carrier files FAIL: 207
distinct owned submodules failing: 64
```

Earliest-missing-anchor frequency across the 207 owned carriers:
```
 201 carriers : earliest missing = §11.4.142   (carry through 11.4.141, missing 142→166 = 25 anchors)
   6 carriers : earliest missing = far older    (e.g. submodules/doc_processor — missing from CONST-047 / §11.4.98 onward)
```

The 201-carrier bulk is the "missing §11.4.142 .. §11.4.166" cohort — every owned submodule's CLAUDE/AGENTS/CONSTITUTION trio lags the meta-repo's most-advanced anchor set by the full 142→166 band. `doc_processor` is materially worse, missing from the CONST-047 / §11.4.98 era.

### Third-party submodules

PASS — correctly excluded (listed in `submodule_third_party.txt`), e.g. `cli_agents/*`, `dependencies/*`, `cli_agents_resources/*`, `mcp_servers`.

---

## STEP 3 — `git submodule status` + per-submodule anchor counts

`git submodule status` lists **130** submodule entries total (the large majority third-party `cli_agents/*`, `dependencies/*`, `cli_agents_resources/*`).

Requested sample carriers — all FOUND under `submodules/` (flat `helix_qa/`, `helix_agent/`, `llm_provider/`, `llms_verifier/` do NOT exist; only the `submodules/<name>/` form):

| Carrier (CLAUDE.md)                       | cnt `11.4.162` | cnt `11.4.166` | highest anchor present |
|-------------------------------------------|:--------------:|:--------------:|:----------------------:|
| `constitution/CLAUDE.md` (source of truth)|       3        |       1        | **11.4.166**           |
| `CLAUDE.md` (root meta-repo)              |       2        |       0        | 11.4.165               |
| `submodules/helix_qa/CLAUDE.md`           |       0        |       0        | 11.4.141               |
| `submodules/helix_agent/CLAUDE.md`        |       2        |       0        | 11.4.162               |
| `assets/CLAUDE.md`                        |       2        |       0        | 11.4.162               |
| `submodules/llm_provider/CLAUDE.md`       |       0        |       0        | 11.4.141               |
| `submodules/llms_verifier/CLAUDE.md`      |       0        |       0        | 11.4.141               |

Quantified lag:
- **constitution** (canonical root) — current at 11.4.166. ✅
- **root CLAUDE.md** — has 11.4.162, lacks 11.4.166; highest = 11.4.165 → lags by 1 anchor (the new semgrep §11.4.166).
- **helix_agent / assets** — have 11.4.162, lack 11.4.166; highest = 11.4.162 → lag ~4 anchors (163→166).
- **helix_qa / llm_provider / llms_verifier** — lack BOTH 11.4.162 and 11.4.166; highest = 11.4.141 → lag ~25 anchors (142→166). **Worst-lagging cohort.**

---

## STEP 4 — Post-pull obligation: gates a full §11.4.55 sweep runs

The constitution was bumped to `4ff4985` (now includes §11.4.166 semgrep + §11.4.157 GEMINI lockstep). Per §11.4.55, on a content-changing constitution pull the project MUST run `scripts/verify-all-constitution-rules.sh` BEFORE the new HEAD is canonical. That sweep runs the **14 gates** enumerated above:

```
G1  Governance cascade (§11.9 + CONST-047..059)              [wraps verify-governance-cascade.sh]
G2  CONST-035 anti-bluff smoke (production code)
G3  CONST-050(A) mock-from-production audit
G4  CONST-051(C) nested-own-org submodule chains
G5  CONST-053 .gitignore + sensitive-file audit
G6  CONST-052 case-conformance (soft, phased)
G7  §11.4.83 docs/qa/ end-user evidence (HXC-019)
G8  §11.4.90 Obsolete-Details (HXC-018)
G9  §11.4.91 summary-doc clarity (HXC-018)
G10 §11.4.81 cross-platform parity (HXC-015)
G11 §11.4.93/95 workable-items md↔db sync (HXC-026)
G12 §11.4.12/91 summary-doc freshness (CM-{ISSUES,FIXED}-SUMMARY-SYNC)
G13 §11.4.99 Sources-verified footers (CM-SOURCES-VERIFIED; HXC-030)
G14 §11.4.106 docs_chain verify (CM-COVENANT-114-106-PROPAGATION)
```

Observation: the sweep's anchor coverage currently TOPS OUT at §11.4.106 (G14). It does NOT yet carry dedicated gates for the newly-pulled §11.4.157 (GEMINI lockstep), §11.4.166 (semgrep), or the 142→166 band beyond what G1's cascade-propagation check enforces. The propagation lag those new anchors create is caught indirectly via **G1 → verify-governance-cascade.sh** (which DID flag §11.4.166 missing on root CLAUDE.md and 142→166 missing fleet-wide), but no per-mandate runtime gate (e.g. a semgrep-wired gate for §11.4.166) is present in this sweep.

---

## Bottom line

- §11.4.55 sweep script: **PRESENT and FUNCTIONAL** — verdict **FAIL (7/14 gates)**.
- Cascade verifier: **FAIL — 213 failures** (63 PASS).
- Worst-lagging carriers: every owned submodule (64 distinct, 207 carrier files) — bulk cohort missing §11.4.142→166 (25 anchors); helix_qa / llm_provider / llms_verifier worst at highest=11.4.141.
- Root carriers are closest: root CLAUDE.md lags by just §11.4.166; AGENTS/QWEN/GEMINI from §11.4.159; CRUSH from §11.4.142.
- A genuine post-pull cascade (CONST-047 recursive propagation of §11.4.142→166 incl. §11.4.157 GEMINI lockstep + §11.4.166 semgrep) is OWED across the whole fleet before `4ff4985` can be treated as canonical.
