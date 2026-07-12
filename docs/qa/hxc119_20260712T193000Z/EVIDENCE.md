# HXC-119 — QA evidence (§11.4.83)

**Item:** HXC-119 (Feature/High) — ACP Phase-5 permission mapping
**Source commit:** helix_code `fbfffd7d` (4 files; pushed github+gitlab)
**Date (UTC):** 2026-07-12T19:30:00Z
**Closure vocab:** Implemented (§11.4.33, Feature)
**Operator decision:** Option B — map onto internal/tools/permissions (maximal flexibility)

## What was implemented
PermissionAdapter bridges ACP's session/request_permission onto HelixCode's
internal/tools/permissions engine. Flow: tool call → BuildConfirmationRequest →
Engine.Decide → Allow(execute)/Deny(reject)/Ask(conn.RequestPermission to client).
Fail-closed: nil engine → ActionAsk (never auto-approve). classifyToolKind maps
to ACP ToolKind (read/edit/delete/other).

## Tests
14 unit tests: all operation types, risk levels, reversibility, nil-engine fail-closed,
parameter extraction, tool kind classification. All PASS (0.065s).

## Verification
go build/vet exit 0. Anti-bluff smoke clean. Agent struct updated with SetPermissionAdapter.
doc.go updated to reflect Phase-5 implementation.
