# OpenDesign Integration Audit (§11.4.162) — Findings

| Field | Value |
|-------|-------|
| Date | 2026-06-22 |
| Scope | helix_code consumer + constitution submodule |
| Method | Read-only audit, captured evidence (§11.4.6/§11.4.123) |
| Verdict | **WIRED-BUT-DORMANT + NOT CONSUMED BY ANY UI** |

## Overall status

The OpenDesign MCP entry exists but is `disabled:true`, the daemon is down, no
enforcement gate is implemented, and every HelixCode UI theme is hand-coded
hardcoded hex — **zero OpenDesign tokens anywhere**.

## Captured evidence (verified 2026-06-22)

- `open-design-mcp` v0.16.1 installed (homebrew, `/opt/homebrew/bin/open-design-mcp`).
- `.mcp.json` `open-design` entry = `"disabled": true`; env `OD_DAEMON_URL=http://localhost:7456`.
- `curl localhost:7456` → **HTTP 000 (curl exit 7)** — daemon NOT running.
- `open-design-mcp` binary without env → `FATAL: invalid core env vars - OD_DAEMON_URL: Required`. Functionally inert.
- `internal/theme/builtin.go` has light+dark, but as **Go-pinned palettes**, not OD-sourced.
- `applications/desktop/theme.go` uses hardcoded brand hex constants (`hxcPrimary = "#A8DD22"` …, "always dark", no light variant) — the ad-hoc pattern §11.4.162 forbids.
- No `tokens.json` / `.css` / `design-tokens` artifact exists anywhere.
- §11.4.162 anchor present in all 5 constitution carriers + repo `CLAUDE.md`, but **MISSING** from `assets/{CLAUDE,AGENTS}.md` and `submodules/helix_agent/{CLAUDE,AGENTS}.md`.
- `find constitution scripts -iname '*opendesign*'` → **empty**. Neither `CM-OPENDESIGN-UI-SYSTEM` nor `CM-COVENANT-114-162-PROPAGATION` is implemented as a script; both exist only as governance prose.
- No visual-regression test harness wired to any HelixCode UI surface.
- Doc drift: `docs/OPENSIGN.md` (filename typo) references nonexistent `assets/Logo.jpeg` (actual: `assets/Logo.png`).

## Per-requirement compliance table

| Req | Requirement | Status | Evidence |
|-----|-------------|--------|----------|
| (a) | OpenDesign mandatory for every surface, not ad-hoc CSS | **MISSING** | Only `.mcp.json` refs OD; themes are hardcoded hex; MCP disabled+dormant |
| (b) | Installed dep; tokens for color(light+dark), typography, spacing, component | **PARTIAL** | Binary installed but disabled; brand colors are Go constants, no token file, no typography/spacing tokens |
| (c) | Every component ships light+dark | **PARTIAL** | inner theme has both; desktop Fyne brand = always-dark, no light |
| (d) | No overlap/font collision/label overlay | **MISSING (unverified)** | No layout-regression check exists |
| (e) | All UI changes covered incl. visual-regression | **MISSING** | No visual-regression harness for HelixCode UI |
| (f) | Both gates + paired §1.1 mutation | **PARTIAL** | Anchor text in 6 files; NO gate scripts; no mutation |

**Totals: 0 COMPLIANT / 3 PARTIAL / 3 MISSING.**

## Remediation plan

### Constitution level (do FIRST — mandate is currently unenforceable)
- Author `CM-OPENDESIGN-UI-SYSTEM` gate script (does not exist) asserting: OD is a
  declared dependency, a token artifact exists and is consumed (no ad-hoc hex in
  theme files), light+dark variants present, visual-regression tests cover UI.
- Author `CM-COVENANT-114-162-PROPAGATION` gate (scan fleet for literal `11.4.162`)
  + paired §1.1 mutation (strip literal → FAIL; inline-hex-for-token → FAIL;
  light-only component → FAIL). Keep project-agnostic per §11.4.28; consumer
  registers token path + UI globs per §11.4.35.

### helix_code consumer level
- Stand up the OD daemon (Docker per OPENDESIGN.md), set `.mcp.json disabled:false`,
  confirm `curl localhost:7456` healthy.
- Generate a design-token source from `assets/Logo.png` (light+dark palettes,
  typography, spacing) via OpenDesign → tracked token file.
- Refactor `applications/desktop/theme.go` + `internal/theme` to consume the token
  file instead of hardcoded hex; add a light brand variant for desktop.
- Add per-pixel/perceptual visual-regression tests for desktop/TUI/web, wired into
  the standard suite.
- Propagate §11.4.162 into `assets/` + `submodules/helix_agent/` governance files.
- Fix doc drift: rename `docs/OPENSIGN.md` → `OPENDESIGN.md`; correct
  `assets/Logo.jpeg` → `assets/Logo.png`.

## Conclusion

`CM-OPENDESIGN-UI-SYSTEM` gate MUST be authored; it currently has zero
implementation. OpenDesign is documented and MCP-wired but functionally dormant
and consumed by no UI — the §11.4.162 mandate is presently unenforced and unmet.
