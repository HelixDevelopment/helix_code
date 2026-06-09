# HXC-048 — helixcode-system.yaml HelixQA bank (11 cases) covering non-auth server surface
Authored banks/helixcode-system.yaml (HXC-SYS-001..011): /health, /api/v1/server/info, /api/v1/system/status (401),
/api/v1/llm/providers, + negatives (unknown path 404, wrong method). Uses only runner-consumed fields
(action/body/headers/auth/expect_status/expect_json_path/expect_body_contains/_skip). Parse-validated: `helixqa list`
→ 11 cases; dry http run vs unreachable host fired real requests for 10 active cases + honored _skip on SYS-011.
Confident body asserts from captured real responses (healthy/status, database/redis/go_version, source/ollama);
status-only where exact message not captured (§11.4.6); SYS-011 _skip (404-vs-405 not pinned). helix_qa commit f18a5d3b.
LIVE-RUN against booted server QUEUED (needs server-owning stream). 

## CLOSED — live runtime evidence (commit 4d2dcb2)
Rebuilt helix_code/bin/helixcode from CURRENT source (the on-disk binary was stale 12:16; rebuild 17:16).
Booted no-infra (db=nil graceful, redis off) on :18090, /health 200. Live `helixqa http`:
- helixcode-system.yaml: 11/11 PASS (SYS-011 pinned: non-GET on /health → real 404 "404 page not found"; Gin has no HandleMethodNotAllowed → un-skipped).
- helixcode-auth.yaml: 16/16 PASS.
Stale-artifact lesson (§11.4.108/§11.4.139): first auth run on the STALE pre-fix binary returned 500 (HXC-043 panic);
after rebuild it returns 401 — re-confirms the HXC-043 fix on a clean artifact. Server SIGTERM clean, port freed.
