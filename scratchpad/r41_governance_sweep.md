# R41 — Constitution post-pull sweep (§11.4.32) + §11.4.157 5-carrier lockstep check

**(T1/feature/helixllm-full-extension)** — repo `/home/milos/Factory/projects/tools_and_research/helix_code`, run 2026-07-11.

**Constitution submodule state:** `constitution` pinned at `6c28fd8` (`helixcode-v1.1.0-76-g6c28fd8`) — ahead of the prior ledger's `b816e66`; the sweep below is that post-pull validation (§11.4.32).

---

## 1. Sweep run — before / after

Script: `scripts/verify-all-constitution-rules.sh` (16 gates).

| Run | PASS | FAIL | SKIP (honest) |
|---|---|---|---|
| Before (this session's first run) | 11 | 4 (G7, G11, G12, G13) | 1 (G14) |
| After (fixes applied)             | 14 | 1 (G7 — pre-existing, honest-tracked) | 1 (G14) |

Full raw output: `scratchpad/sweep_output.txt` (before), `scratchpad/sweep_output_2.txt` (after).

---

## 2. §11.4.157 — 5-carrier lockstep matrix

Extracted the set of `11.4.NNN` anchor literals from each root-level carrier via
`grep -oE '11\.4\.[0-9]+' <file> | sort -u`, then pairwise-diffed every carrier
against `CLAUDE.md`.

| Carrier | File size | Distinct `11.4.NNN` anchors | Highest anchor | Diff vs CLAUDE.md |
|---|---|---|---|---|
| CLAUDE.md | 407,369 B | 177 | 11.4.185 | — (baseline) |
| AGENTS.md | 416,670 B | 177 | 11.4.185 | **empty (identical set)** |
| QWEN.md   | 365,521 B | 177 | 11.4.185 | **empty (identical set)** |
| GEMINI.md | 366,072 B | 177 | 11.4.185 | **empty (identical set)** |
| CRUSH.md  | 346,917 B | 177 | 11.4.185 | **empty (identical set)** |

`§12.10` / `§12.12` (the non-`11.4.x` anchors, e.g. the process/thread-limit
mandate) also present identically in all five files.

**Result: all five carriers are ALREADY in full §11.4.157 lockstep — no
missing anchors, no propagation needed.** This differs from the constitutional
anchor text's own forensic example (GEMINI.md once lagged at §11.4.141) — that
drift has evidently already been closed by a prior session (files carry mtime
2026-07-09, before this sweep). No governance-carrier edits were made in this
session because none were needed; confirmed mechanically, not assumed.

---

## 3. What was closed (fixable-by-us, real mechanical fixes, no fabrication)

### G11 — §11.4.93/95 workable-items DB↔MD sync
- **Finding:** `workable-items-linux diff` showed `HXC-114 present in Markdown, absent in DB`.
- **Fix:** backed up `docs/workable_items.db` (§9.2 pre-op backup) to
  `scratchpad/backups/workable_items.db.bak.20260711T152044`, ran
  `constitution/scripts/workable-items/bin/workable-items-linux sync md-to-db --db docs/workable_items.db --issues docs/Issues.md --fixed docs/Fixed.md`
  (`synced 308 total items`), then `sqlite3 docs/workable_items.db "PRAGMA wal_checkpoint(TRUNCATE);"` per §11.4.95.
- **Verified:** re-ran `diff` → `DB and Markdown are in sync`. Sweep G11 now PASS.

### G12 — §11.4.12/91 summary-doc freshness
- **Finding:** `docs/Issues_Summary.md` and `docs/Fixed_Summary.md` stale
  (`Last regenerated: 2026-07-08` vs current tracker state).
- **Fix:** ran `scripts/generate_issues_summary.sh` (44 items: 3 open / 41
  closed) and `scripts/generate_fixed_summary.sh` (Bug=79 Feature=79 Task=30
  total=188).
- **Verified:** `--check` on both → `CM-ISSUES-SUMMARY-SYNC: PASS`,
  `CM-FIXED-SUMMARY-SYNC: PASS`. Sweep G12 now PASS.
- **Known out-of-scope side-effect (not chased):** the tracked HTML/PDF/DOCX
  siblings of these two files (`docs/Issues_Summary.{html,pdf,docx}`,
  `docs/Fixed_Summary.{html,pdf,docx}`) are now stale by mtime relative to the
  regenerated `.md`. Confirmed by inspecting `scripts/verify-all-constitution-rules.sh`
  and `scripts/gates/` that md→html→pdf hash parity is checked *only* inside
  **G14** (the docs_chain engine gate), which is honestly SKIPPED — the engine
  is not installed on this host (not on PATH, no sibling `../docs_chain`, no
  `.docs_chain/bin/docs_chain`). There is no separate always-on sibling-mtime
  gate in this 16-gate sweep. Regenerating the exports via an ad-hoc pandoc
  call outside the constitution's declared mechanism (§11.4.106 mandates
  docs_chain as the sync engine) would itself be a governance shortcut, so it
  was deliberately not done. This is pre-existing/latent debt (missing engine
  install), not a new failure introduced by this session's `.md` regen.

### G13 — §11.4.99 Sources-verified footer
- **Finding:** `docs/guides/HELIXLLM_CAPABILITIES_GUIDE.md` was the 1 of 74
  operator-facing docs lacking a `## Sources verified <date>: <urls>` footer.
  The doc already carried an extensive, honest internal `## Sources` section
  (captured `docs/qa/*/RESULTS.md` evidence, dated 2026-07-06→2026-07-09) but
  documents live invocation of several third-party provider APIs (Mistral,
  Codestral, Cohere v2, Cerebras, Groq) whose base-URL claims warranted real
  external cross-reference per §11.4.99.
- **Fix:** performed genuine (not fabricated) `WebFetch` verification against
  4 official sources, then appended an honest `## Sources verified 2026-07-11`
  footer citing exactly what was checked and what it corroborated — no
  overclaiming beyond what was actually fetched:
  - `https://docs.mistral.ai/api/` → confirmed `api.mistral.ai/v1` current (matches guide §17a).
  - `https://inference-docs.cerebras.ai/api-reference/chat-completions` → confirmed `api.cerebras.ai/v1` current (matches guide §17d's fix).
  - `https://docs.cohere.com/v2/reference/chat` → confirmed `POST https://api.cohere.com/v2/chat` current (matches guide §17b); noted explicitly that this source does **not** itself confirm v1 removal — that specific claim rests on the guide's own live HTTP-404 probe evidence, not on the external source (no overclaim).
  - `https://console.groq.com/docs/api-reference` → confirmed `https://api.groq.com/openai/v1` current (matches guide §17d).
- **Verified:** sweep now reports `74/74 operator-facing docs footered (100%)`. Sweep G13 now PASS.

---

## 4. What was honest-tracked as pre-existing debt (NOT fixed, NOT fabricated)

### G7 — §11.4.83 docs/qa/ end-user evidence (13 violations)
13 feature-shipping commits lack a matching `docs/qa/<run-id>/` evidence
directory (14 evaluated; only `a52a523a` already has evidence at
`docs/qa/DeepSeek/`).

**Regression-isolation evidence** (per task instruction, `git diff <merge-base main HEAD>..HEAD -- <path>` methodology, applied at the commit level since G7 is commit-scoped not path-scoped):

```
$ git merge-base main HEAD
dfa6a2c2be302c431c4762773aa6a2c6a6d1f6f7
```

Checked all 13 violating commit hashes with
`git merge-base --is-ancestor <commit> dfa6a2c2be302c431c4762773aa6a2c6a6d1f6f7`
— **all 13 resolved `BEFORE merge-base (pre-existing)`**:

```
f8c38181  fix(provider): update Cerebras model list to match live /models endpoint
67c9a9bc  fix(provider): Cohere v1->v2 endpoint + model default update
f9dcf6a6  fix(server): update CORSMiddleware call sites for allowlist signature
4727a9d0  fix(security): CORS no wildcard-origin-with-credentials
9c876819  fix(security): authenticate /ws MCP WebSocket + fix CSWSH CheckOrigin gap
d6c05f76  docs(build): desktop/GUI (Fyne) build-host prerequisites
2ff55c31  fix(security): auth-gate the wire facade
a21ad7ca  feat(server): route HelixCode LLM to local HelixLLM coder (:18434)
51c058b1  feat(server): dual OpenAI + Anthropic wire facade on HelixCode's own server
1254e0a6  sync: auto-commit before cross-host sync 20260628
66f9c21e  fix(server): advertise streaming_enabled + plugins_enabled in /server/info
c9bad26a  fix(mcp): move blocking I/O out of toolMux critical section
b058c7c2  fix(tests/cognee): make the package deterministic under load
```

None of these were authored in this sweep session (this session made zero
feature/fix commits — only the governance commit below). Per the hard
constraint ("Do not fabricate docs/qa transcripts for commits that aren't
ours"), these 13 are **honestly tracked as pre-existing debt**, not fixed,
not annotated with a fabricated evidence directory. This grows the prior
ledger's "4 pre-session commits" figure to 13 — expected, since work
continued on the branch between the last ledger snapshot and this sweep;
the growth itself is transparently visible in the regression-isolation
evidence above and is not concealed.

### G14 — §11.4.106 docs_chain verify — honest SKIP (unchanged)
docs_chain engine is not installed on this host. `SKIP-OK: §11.4.3`. Out of
scope for this task (installing a new engine is separate, larger work, not a
"close genuinely-ours governance debt" sweep item).

---

## 5. Files touched this session

- `docs/workable_items.db` — re-synced from `docs/Issues.md` + `docs/Fixed.md` (md-to-db), WAL-checkpointed. Backup at `scratchpad/backups/workable_items.db.bak.20260711T152044`.
- `docs/Issues_Summary.md` — regenerated via `scripts/generate_issues_summary.sh`.
- `docs/Fixed_Summary.md` — regenerated via `scripts/generate_fixed_summary.sh`.
- `docs/guides/HELIXLLM_CAPABILITIES_GUIDE.md` — additive `## Sources verified 2026-07-11` footer appended (no existing content removed or altered).
- `CLAUDE.md` / `AGENTS.md` / `QWEN.md` / `GEMINI.md` / `CRUSH.md` — inspected only, **not modified** (already in lockstep, see §2 above).

No submodule paths touched. No `/mnt/track1`, no adb device, no
`submodules/helix_agent` interaction of any kind.
