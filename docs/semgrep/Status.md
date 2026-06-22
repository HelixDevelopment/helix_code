# Semgrep Status — helix_code

**Constitution:** §11.4.166 (Universal Semgrep static analysis mandate)
**Scope:** /Volumes/T7/Projects/helix_code (consumer-side ledger)
**Source of truth for events:** this file (append-only). Derived exports:
`Status.html`, `Status.pdf` via the `semgrep_status` docs_chain context
(`.docs_chain/contexts/semgrep_status.yaml`). The constitution submodule keeps
its own authoritative ledger at `constitution/docs/semgrep/Status.md`; the
scripts are inherited by reference (§11.4.28 / §11.4.35), never copied.

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-22 |
| Status summary | Installed + validated + real-scan PASS; wiring prepared (conductor to apply) |

---

## Event log

### 2026-06-22 — Onboarding (install + validate + real-scan)

- **Install:** semgrep already present — `semgrep --version` = `1.167.0`,
  `command -v semgrep` = `/opt/homebrew/bin/semgrep` (Homebrew). No install
  needed; working-binary confirmed (not just a present file).
- **Validate:** `sh constitution/scripts/semgrep/semgrep_validate.sh` → ALL PASS
  (exit 0). Valid JSON with results + registry reachable. Evidence under
  `constitution/docs/.semgrep/`.
- **Real scan:** `semgrep scan --config auto helix_code/internal/auth helix_code/internal/llm`
  — 161 rules on 112 files, **4 findings** (all
  `go.lang.security.audit.dangerous-exec-command`, Blocking, in
  `helix_code/internal/llm/`): `auto_llm_manager.go:520`,
  `local_llm_manager.go:439`, `model_converter.go:303`,
  `model_download_manager.go:479`. Audit-class (input-provenance) — not confirmed
  exploits. `SEMGREP_APP_TOKEN` absent → Community registry rules; Pro/app tier
  honestly SKIPPED per §11.4.3.
- **Evidence basis:** `docs/research/semgrep_onboarding_20260622/findings.md`.

### 2026-06-22 — Wiring prepared (NOT yet applied; conductor/operator applies)

- docs_chain context `.docs_chain/contexts/semgrep_status.yaml` created (this ledger).
- Pre-commit wiring spec: `docs/research/semgrep_wiring_20260622/precommit_wiring.md`.
- `.mcp.json` "semgrep" entry spec: `docs/research/semgrep_wiring_20260622/mcp_entry.json`.
- Shell rc PATH line spec: `docs/research/semgrep_wiring_20260622/wiring_instructions.md`.
