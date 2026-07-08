# Phase-1 MCP gateway real end-to-end proof — summary

Gateway binary: /home/milos/Factory/projects/tools_and_research/helix_code/docs/qa/phase1_mcp_gateway_20260708T074850Z/harness/mcp-gateway.bin
Gateway listen: :18444
Coder base URL (never touched, read-only): http://localhost:18434

## 401 without Bearer
HTTP status: 401
Body: no bearer token



## tools/list (with valid Bearer)
[helixllm_generate helixllm_list_models]

## helixllm_generate real call
prompt: "What is 2+2? Reply with only the digit, nothing else."
content: "4"

## helixllm_list_models real call
[/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf]
