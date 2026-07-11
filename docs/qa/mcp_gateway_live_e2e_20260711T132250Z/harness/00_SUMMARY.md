# R41F MCP gateway LIVE end-to-end re-proof — summary

Gateway binary: /home/milos/Factory/projects/tools_and_research/helix_code/docs/qa/mcp_gateway_live_e2e_20260711T132250Z/harness/mcp-gateway.bin
Gateway listen: :18446
Coder base URL (never touched, read-only throughout — see 00/05 pre/post-flight): http://localhost:18434
Bearer token: <REDACTED-HELIX_MCP_GATEWAY_TOKEN> (fresh random test credential, never written to any evidence file)

## 0. Coder pre-flight (before gateway touched anything)
HTTP 200, see 00_coder_preflight_models.json

## 1. 401 without Bearer
HTTP status: 401
Body: no bearer token



## 2. tools/list (with valid Bearer) — both tools present
[helixllm_generate helixllm_list_models]

## 3. helixllm_generate real call — FRESH nonce echo (anti-bluff/anti-cache proof)
nonce sent: "R41F-NONCE-f23370ad49e54ea3"
content received: "R41F-NONCE-f23370ad49e54ea3"

## 4. helixllm_list_models real call — live model id from :18434
[/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf]

## 5. Coder post-flight (after gateway torn down) — still answers, untouched
HTTP 200, see 05_coder_postflight_models.json
