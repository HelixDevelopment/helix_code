# HelixCode — Issues Summary

> Generated from `docs/Issues.md` per Constitution §11.4.15 + §11.4.16. Must stay in sync with the source via `CM-DOCS-EXPORT-SYNC` discipline.
>
> **Round 189 prefix convention:** IDs are now scope-prefixed (`HXC` = root project; `HXA` = HelixAgent; `HXL` = HelixLLM; `HXQ` = HelixQA; `VEN` = VisionEngine; etc.). See `docs/Issues.md` "Prefix convention" section for the full table + legacy `ISSUE-NNN` → new mapping.

| ID | Title | Type | Status | Discovered | Notes |
|---|---|---|---|---|---|
| VEN-001 | VisionEngine `helix-gitlab` remote repo missing (404) | Task | Completed (→ Fixed.md) round 188 | 2026-05-19 | NOT missing — URL misconfig (HelixDevelopment vs helixdevelopment1 path); fixed via git remote set-url; 100/100 owned-org URLs reachable |
| VEN-002 | VisionEngine `vasic-digital-github` fork lineage divergent at SHA 93c830a | Bug | Queued — BLOCKED on operator | 2026-05-19 | CONST-061 merge-first investigation needed; carries round-48/52/57 commits |
| HXL-001 | HelixLLM `analysis_test.go` hardcoded absolute path (was: helix_agent) | Bug | Fixed (→ Fixed.md) round 105 | 2026-05-19 | Closed via t.TempDir + fixtures (a5e56d4) |
| HXL-002 | HelixLLM `gateway/middleware` TOON `WriteTOON` 500 (was: helix_agent) | Bug | Fixed (→ Fixed.md) round 105 | 2026-05-19 | Closed via json.Marshal fallback (a5e56d4) |
| HXC-001 | CONST-052 rename programme: meta-repo dirs still PascalCase | Task | Plan-Ready (→ specs round 113) | 2026-05-15 | Round 113 produced phased plan (f666410). 12 operator decisions needed before execution |
| HXC-002 | Round-74 residual LOGIC FAILs | Bug | Fixed (→ Fixed.md) | 2026-05-19 | All 3 components closed: HelixMemory (106), Planning (107), helix_agent (109) |
| HXA-001 | helix_agent handler tests surfaced after round-109 fix | Bug | Fixed (→ Fixed.md) round 116 | 2026-05-19 | Closed: 4 tests fixed (da782d4) |
| HXA-002 | helix_agent 3 build-failed packages (sibling debate API drift) | Bug | Queued — BLOCKED on cross-submodule coord | 2026-05-19 | digital.vasic.debate API changed; helix_agent consumers broken |
| HXA-003 | venice TestGetCapabilities model-list drift (CONST-037) | Bug | Fixed (→ Fixed.md) round 190 | 2026-05-19 | Closed: structural assertion (NotEmpty + family-substring) + SKIP-OK fallback per CONST-035 (220eff0f) |
| HXC-003 | CONST-046 migration backlog (57,329 violations, baselined, shrinking) | Feature | In progress | 2026-05-19 | Phase 4 (rounds 100+) actively migrating; audit-gate `--fail-on-new` enforced |
| HXQ-001 | helix_qa intermittent TestPerformance flake (host-load-sensitive) | Bug | Queued — BLOCKED on operator | 2026-05-19 | Either loosen tolerance or gate behind HOST_LOAD_DEDICATED env var |

**Counts**: 4 open (VEN-001 closed round 188; HXA-001 closed round 116; HXA-003 closed round 190; HXC-002/HXL-001/HXL-002 also closed) | 3 Bugs open | 1 Task open (HXC-001 Plan-Ready) | 1 Feature open (HXC-003) | 3 BLOCKED (VEN-002 + HXQ-001 operator + HXA-002 cross-submodule coord)

*Last regenerated: 2026-05-19 (round 189 — prefix rename). See `docs/Issues.md` for full details + prefix convention table. PDF/HTML exports auto-regenerated via `scripts/regenerate-tracker-exports.sh`.*
