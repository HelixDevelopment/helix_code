# HelixCode — Issues Summary

> Generated from `docs/Issues.md` per Constitution §11.4.15 + §11.4.16. Must stay in sync with the source via `CM-DOCS-EXPORT-SYNC` discipline.

| ID | Title | Type | Status | Discovered | Notes |
|---|---|---|---|---|---|
| ISSUE-001 | VisionEngine `helix-gitlab` remote repo missing (404) | Task | Queued — BLOCKED on operator | 2026-05-19 | Operator must create the GitLab repo or remove the named remote |
| ISSUE-002 | VisionEngine `vasic-digital-github` fork lineage divergent at SHA 93c830a | Bug | Queued — BLOCKED on operator | 2026-05-19 | CONST-061 merge-first investigation needed; carries round-48/52/57 commits |
| ISSUE-003 | helix_agent `analysis_test.go` hardcoded absolute path | Bug | Queued | 2026-05-19 | Test bluff per CONST-035; passes only on operator's host |
| ISSUE-004 | helix_agent `gateway/middleware` TOON `negotiation_test.go` returns 500 | Bug | Queued | 2026-05-19 | Handler bug OR test expectation wrong; investigate which |
| ISSUE-005 | CONST-052 rename programme: meta-repo dirs still PascalCase | Task | Queued | 2026-05-15 | Renames break path-encoded refs; phased migration required |
| ISSUE-006 | Round-74 environmental-class residual LOGIC FAILs (~7 submodules) | Bug | Queued | 2026-05-19 | HelixMemory + vasic-digital/Planning + helix_agent inner test bugs |
| ISSUE-007 | CONST-046 migration backlog (57,329 violations, baselined, shrinking) | Feature | In progress | 2026-05-19 | Phase 4 (rounds 100+) actively migrating; audit-gate `--fail-on-new` enforced |
| ISSUE-008 | helix_qa intermittent TestPerformance flake (host-load-sensitive) | Bug | Queued — BLOCKED on operator | 2026-05-19 | Either loosen tolerance or gate behind HOST_LOAD_DEDICATED env var |

**Counts**: 8 open | 5 Bugs | 2 Tasks | 1 Feature | 3 BLOCKED-on-operator

*Last regenerated: 2026-05-19. See `docs/Issues.md` for full details. PDF/HTML exports deferred — generate via pandoc on demand.*
