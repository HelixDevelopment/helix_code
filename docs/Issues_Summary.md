# HelixCode — Issues Summary

> Generated from `docs/Issues.md` per Constitution §11.4.15 + §11.4.16. Must stay in sync with the source via `CM-DOCS-EXPORT-SYNC` discipline.

| ID | Title | Type | Status | Discovered | Notes |
|---|---|---|---|---|---|
| ISSUE-001 | VisionEngine `helix-gitlab` remote repo missing (404) | Task | Queued — BLOCKED on operator | 2026-05-19 | Operator must create the GitLab repo or remove the named remote |
| ISSUE-002 | VisionEngine `vasic-digital-github` fork lineage divergent at SHA 93c830a | Bug | Queued — BLOCKED on operator | 2026-05-19 | CONST-061 merge-first investigation needed; carries round-48/52/57 commits |
| ISSUE-003 | HelixLLM `analysis_test.go` hardcoded absolute path (was: helix_agent) | Bug | Fixed (→ Fixed.md) round 105 | 2026-05-19 | Closed via t.TempDir + fixtures (a5e56d4) |
| ISSUE-004 | HelixLLM `gateway/middleware` TOON `WriteTOON` 500 (was: helix_agent) | Bug | Fixed (→ Fixed.md) round 105 | 2026-05-19 | Closed via json.Marshal fallback (a5e56d4) |
| ISSUE-005 | CONST-052 rename programme: meta-repo dirs still PascalCase | Task | Plan-Ready (→ specs round 113) | 2026-05-15 | Round 113 produced phased plan (f666410). 12 operator decisions needed before execution |
| ISSUE-006 | Round-74 residual LOGIC FAILs | Bug | Fixed (→ Fixed.md) | 2026-05-19 | All 3 components closed: HelixMemory (106), Planning (107), helix_agent (109) |
| ISSUE-009 | helix_agent handler tests surfaced after round-109 fix | Bug | Queued | 2026-05-19 | 4 tests masked by mid-run panic; now visible |
| ISSUE-010 | helix_agent 3 build-failed packages (sibling debate API drift) | Bug | Queued — BLOCKED on cross-submodule coord | 2026-05-19 | digital.vasic.debate API changed; helix_agent consumers broken |
| ISSUE-011 | venice TestGetCapabilities model-list drift (CONST-037) | Bug | Queued | 2026-05-19 | Hardcoded venice-uncensored no longer in API; needs LLMsVerifier dynamic lookup |
| ISSUE-007 | CONST-046 migration backlog (57,329 violations, baselined, shrinking) | Feature | In progress | 2026-05-19 | Phase 4 (rounds 100+) actively migrating; audit-gate `--fail-on-new` enforced |
| ISSUE-008 | helix_qa intermittent TestPerformance flake (host-load-sensitive) | Bug | Queued — BLOCKED on operator | 2026-05-19 | Either loosen tolerance or gate behind HOST_LOAD_DEDICATED env var |

**Counts**: 7 open (ISSUE-003/004 closed round 105; ISSUE-006 CLOSED round 109; +3 new ISSUE-009/010/011 surfaced by round 109) | 5 Bugs | 1 Task (ISSUE-005 Plan-Ready) | 1 Feature | 4 BLOCKED (3 original + ISSUE-010 cross-submodule coord)

*Last regenerated: 2026-05-19. See `docs/Issues.md` for full details. PDF/HTML exports deferred — generate via pandoc on demand.*
