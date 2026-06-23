# HelixCode Composite Doc-Sync Audit (§11.4.60 / §11.4.65)

**Date:** 2026-06-23
**Scope:** Read-only audit of governed `.md` documents for in-sync `.html`/`.pdf`(/`.docx`) sibling exports per §11.4.60 / §11.4.65 / §11.4.153.
**Method:** epoch mtime comparison (sibling mtime ≥ `.md` mtime ⇒ fresh).
**Authority:** §11.4.65 closed-set scope (INCLUDED: project-root `*.md`, `docs/**/*.md`, etc.). §11.4.153 adds `.docx` to the feature-Status set only.

---

## 1. Governed core doc-set sync table

| Doc | .md mtime | .html | .pdf | .docx | VERDICT |
|-----|-----------|-------|------|-------|---------|
| docs/Issues.md | 1782231922 | fresh | fresh | fresh | in-sync |
| docs/Issues_Summary.md | 1782231922 | fresh | fresh | fresh | in-sync |
| docs/Fixed.md | 1782231922 | fresh | fresh | fresh | in-sync |
| docs/Fixed_Summary.md | 1782231922 | fresh | fresh | fresh | in-sync |
| docs/CONTINUATION.md | 1782229446 | fresh | fresh | n/a (not required) | in-sync |
| README.md | 1780479622 | fresh | fresh | n/a (not required) | in-sync |
| docs/features/Status.md | 1782240015 | fresh | fresh | fresh | in-sync |
| docs/features/Status_Summary.md | 1782240238 | fresh | fresh | fresh | in-sync |

**Core doc-set: 8/8 in-sync.** All required `.html`/`.pdf` (and `.docx` for the feature-Status set) siblings exist and are ≥ their `.md` mtime. No regeneration needed for the core set.

Note: `docs/features/` also contains a legacy `helixcode-status.md` / `helixcode-status_summary.md` pair (the docs_chain-registered names) whose exports are fresh (mtime 1782240xxx era export run). The newer `Status.md`/`Status_Summary.md` are the active §11.4.153 feature ledger and are in-sync.

---

## 2. docs/qa/*.md — §11.4.65 scope analysis (HONEST §11.4.6)

§11.4.65 closed-set scope INCLUDES `docs/**/*.md` explicitly. By the literal mandate text, every `docs/qa/*.md` requires `.html` + `.pdf` siblings in sync. **However**, the project's docs_chain instantiation (`.docs_chain/contexts/*.yaml`) registers ONLY specific named docs (features Status pair, CONTINUATION, governance files, Issues/Fixed, xiaomi providers) — `docs/qa/*.md` are NOT wired into any docs_chain chain. So they are in §11.4.65 textual scope but have NEVER been mechanically exported.

This session created/modified the following docs/qa top-level `.md` files; NONE have current exports:

| Doc | .html | .pdf | VERDICT |
|-----|-------|------|---------|
| docs/qa/HXC-107_ledger_audit.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_android_evidence.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_cli_recordings_evidence.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_desktopgui_features_evidence.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_evidence_audit.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_ios_evidence.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_liveness_verification.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_remaining_platforms_evidence.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_selftest_evidence.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_tui_server_recordings_evidence.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_tui_views_evidence.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_video_qa_matrix.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-108_web_evidence.md | MISSING | MISSING | missing-export |
| docs/qa/HXC-112_gui_content_verification.md | MISSING | MISSING | missing-export |
| docs/qa/RETRO_LEDGER.md | MISSING | MISSING | missing-export |
| docs/qa/README.md | STALE | STALE | stale-export |

**docs/qa verdict:** 16 docs/qa `.md` files lack fresh exports (15 missing, 1 stale README). Per the literal §11.4.65 INCLUDED scope these are export-required. The conductor should EITHER regenerate `.html`+`.pdf` for each (preferred — matches the mandate text) OR, if the operator's §11.4.35 instantiation intentionally treats `docs/qa/` evidence transcripts as exempt, that exemption must be made explicit in the project governance (it is not currently documented, so this audit flags them rather than silently passing — §11.4.6 no-guessing).

---

## 3. §11.4.86 video-evidence fingerprint gate

`scripts/gates/feature_video_evidence_gate.sh` (read-only run):

```
GATE PASS: 8 cited durable evidence paths all exist; no rotatable-corpus citation;
fingerprint matches (0d8e196645b3da9efc48a65860ac768226dc8db1bef3cc298f3b0dc3b708ba36)
EXIT=0
```

Sidecar `docs/features/.video_evidence_roster.sha256` = `0d8e196645b3da9efc48a65860ac768226dc8db1bef3cc298f3b0dc3b708ba36` — consistent with the current Status.md confirmed set. **Gate: PASS.**

---

## 4. Exports needing regeneration (exact paths)

Core set: none.

docs/qa set (§11.4.65 textual scope — regenerate `.html`+`.pdf` for each, OR document an explicit exemption):
- docs/qa/HXC-107_ledger_audit.{html,pdf}
- docs/qa/HXC-108_android_evidence.{html,pdf}
- docs/qa/HXC-108_cli_recordings_evidence.{html,pdf}
- docs/qa/HXC-108_desktopgui_features_evidence.{html,pdf}
- docs/qa/HXC-108_evidence_audit.{html,pdf}
- docs/qa/HXC-108_ios_evidence.{html,pdf}
- docs/qa/HXC-108_liveness_verification.{html,pdf}
- docs/qa/HXC-108_remaining_platforms_evidence.{html,pdf}
- docs/qa/HXC-108_selftest_evidence.{html,pdf}
- docs/qa/HXC-108_tui_server_recordings_evidence.{html,pdf}
- docs/qa/HXC-108_tui_views_evidence.{html,pdf}
- docs/qa/HXC-108_video_qa_matrix.{html,pdf}
- docs/qa/HXC-108_web_evidence.{html,pdf}
- docs/qa/HXC-112_gui_content_verification.{html,pdf}
- docs/qa/RETRO_LEDGER.{html,pdf}
- docs/qa/README.{html,pdf} (regenerate — stale)
- docs/qa/helixcode_docsync_audit.{html,pdf} (this new audit doc — also export-required)

---

## 5. Overall verdict

- **Core governed doc-set (Issues/Fixed/+summaries/CONTINUATION/README/feature-Status/+summary): RELEASE-READY** — all 8 in-sync, all required siblings fresh including the feature-Status `.docx` pair.
- **§11.4.86 video-evidence fingerprint gate: PASS.**
- **docs/qa/*.md: 17 exports need regeneration** (16 existing docs missing/stale + this audit doc) under the literal §11.4.65 `docs/**/*.md` INCLUDED scope.

**Net:** docs are release-ready for the core set; **17 docs/qa exports must be regenerated** (or an explicit §11.4.35 docs/qa exemption documented) before a release tag, to satisfy §11.4.65 for the full `docs/**/*.md` surface.
