# HXC-117 — QA evidence (§11.4.83)

**Item:** HXC-117 (Bug / High) — Model capability flags (MCP, LSP, ACP, RAG, Skills, Plugins) are not sourced from the verifier as required (CONST-040)
**Module:** `helix_code/` inner Go app (`dev.helix.code`) — CLI + server display surfaces
**Fix commit:** helix_code `aa6b20b4` (branch feature/helixllm-full-extension; parent 3e67fa1a)
**Date (UTC):** 2026-07-12T14:09:00Z
**Closure vocab:** Fixed (§11.4.33, Bug)
**Discipline:** §11.4.102 systematic-debugging, §11.4.115 RED-first, §11.4.120 test reconciliation, §11.4.142/§11.4.125 independent review, §11.4.134 iterate-until-GO, §11.4.145 blast-radius, CONST-040 (verifier single-source, no hardcoded caps), CONST-046 (i18n).

## Root cause (Phase 1 — FACT; investigation subagent)

CONST-040 requires the 6 advanced capabilities be verifier-sourced. HXC-135 (llms_verifier 1096057f)
made the verifier COMPUTE + EMIT all 6, and helix_code's `internal/verifier/client.go`
`capabilityAliasFields` decoder DECODES them onto `VerifiedModel.Supports{MCP,LSP,ACP,RAG,Skills,Plugins}`
(`internal/verifier/types.go:54-59`). But NEITHER user-facing surface DISPLAYED them, so users still
could not see verifier-sourced capabilities:
- CLI `printVerifiedModels` (`cmd/cli/main.go`) printed only ID/Name/Provider/Score/ContextSize/Status.
- Server `verifiedModelToJSON` (`internal/server/handlers.go`) emitted `supports_vision/tools` but not the 6.
A full-tree grep confirmed no hardcoded capability literals anywhere (not a BLUFF-002 regression) — a
pure display OMISSION. No infrastructure needed (both operate on in-memory Go structs).

## Fix

- CLI: `printVerifiedModels` → thin wrapper over new `renderVerifiedModels(models) string`
  (strings.Builder seam, testable); a `Capabilities:` line added to the i18n template
  `cli_model_info_verified` in `cmd/cli/i18n/bundles/active.en.yaml`, fed by `formatCapabilityFlags(m)`
  reading the 6 decoded `Supports*` fields. NO hardcoded values (CONST-040).
- Server: `verifiedModelToJSON` adds 6 gin.H keys `supports_mcp/lsp/acp/rag/skills/plugins`, same source.
- Fallback path fix (blast radius, §11.4.145 / §11.4.1): `printFallbackModels` (the CONST-035
  offline-fallback path) shared the same template key → new `renderFallbackModels` seam supplies an
  honest i18n-sourced `cli_capability_flags_unknown` ("unknown (verifier unavailable...)") instead of
  leaking Go text/template's `<no value>` literal.
- CONST-046: Skills/Plugins labels + ✓/✗ indicators + the unknown indicator moved to 5 i18n keys
  (`cli_capability_indicator_supported/unsupported`, `cli_capability_label_skills/plugins`,
  `cli_capability_flags_unknown`); MCP/LSP/ACP/RAG stay literal (protocol acronyms, matching the
  server JSON key convention).

## Guard tests (RED→GREEN, §11.4.115) — per-flag, anti-constant

- `cmd/cli/hxc117_capability_flags_test.go`: `TestHXC117_CLI_PrintVerifiedModels_ShowsCapabilityFlags`
  (per-flag individual-flip — hardcoding ANY one of the 6 flags makes it FAIL) +
  `TestHXC117_CLI_RenderFallbackModels_NoRawTemplateLeak` (fallback path shows no `<no value>`).
- `internal/server/hxc117_capability_flags_test.go`:
  `TestHXC117_Server_VerifiedModelToJSON_IncludesCapabilityFlags` (per-key mixed-profile).
RED confirmed on parent 3e67fa1a (flags absent from both surfaces; fallback leaks `<no value>`).

## Captured verification (`-tags=nogui`, per make verify-compile; bare build fails on pre-existing Fyne/X11 cgo, unrelated — touched pkgs have no go-gl deps)

```
go build -tags=nogui ./cmd/... ./internal/...  -> exit 0
go vet   -tags=nogui                            -> exit 0
go test  -tags=nogui -count=1 ./cmd/cli/... ./internal/server/... ./internal/verifier/...
   -> 358 PASS, 0 FAIL, all ok
anti-bluff smoke -> clean
```

## Independent review (§11.4.142/§11.4.134) — 2 rounds → clean GO

Round 1 NOT-GO: (BLOCKING) `printFallbackModels` sibling regression rendering `<no value>`;
(gap) CLI guard's aggregate polarity-flip missed a single miswired flag (reviewer proved hardcoded LSP
passed); (nit) CONST-046 hardcoded Skills/Plugins/✓/✗. All 3 remediated (§11.4.134). Round 2 GO,
zero findings/warnings — verified with 3 independent §1.1 mutations (single-flag LSP FAIL, single-flag
Plugins FAIL, server supports_lsp FAIL; fallback-key-drop reproduced the exact `<no value>` leak),
each restored byte-identical; fallback render proof (honest "unknown", no fabricated ✓); all 5 i18n
keys resolve to real text; `cli_model_info_verified` has exactly 2 production call sites, both fixed.

## §11.4.108 runtime signature (definition of done)

A `VerifiedModel` whose verifier-computed `SupportsMCP=true` renders "MCP:✓" in the CLI `--list-models`
output AND `"supports_mcp": true` in `/api/v1/llm/models` JSON — sourced from the decoded field, never
hardcoded; the fallback path renders the honest "unknown" indicator, never `<no value>`.
