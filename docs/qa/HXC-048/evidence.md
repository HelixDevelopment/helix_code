# HXC-048 — helixcode-system.yaml HelixQA bank (11 cases) covering non-auth server surface
Authored banks/helixcode-system.yaml (HXC-SYS-001..011): /health, /api/v1/server/info, /api/v1/system/status (401),
/api/v1/llm/providers, + negatives (unknown path 404, wrong method). Uses only runner-consumed fields
(action/body/headers/auth/expect_status/expect_json_path/expect_body_contains/_skip). Parse-validated: `helixqa list`
→ 11 cases; dry http run vs unreachable host fired real requests for 10 active cases + honored _skip on SYS-011.
Confident body asserts from captured real responses (healthy/status, database/redis/go_version, source/ollama);
status-only where exact message not captured (§11.4.6); SYS-011 _skip (404-vs-405 not pinned). helix_qa commit f18a5d3b.
LIVE-RUN against booted server QUEUED (needs server-owning stream). 
