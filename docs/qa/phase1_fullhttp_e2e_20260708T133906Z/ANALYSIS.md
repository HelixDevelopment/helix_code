# Self-Validated Analyzer Results -- fullhttp_gen_selfval_20260708T133850

## Golden-Good Fixture
```json
{"status":"success","content":"Hello! How can I help you today?","provider":"helixllm","model":"llama3.2","usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20},"finish_reason":"stop"}
```
Accepts: TRUE

## Golden-Bad Fixture
```json
{"status":"success","content":"","provider":"helixllm","model":"llama3.2","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0},"finish_reason":"stop"}
```
Rejects: TRUE

## Bank Structure
Bank: /home/milos/Factory/projects/tools_and_research/helix_code/submodules/helix_qa/banks/helixcode-generate-e2e.yaml
RED tests: 3

## Summary
PASS: 9
FAIL: 0
SKIP: 0
