# HelixQA Coder Memory-Leak Soak — Phase 1 Evidence

**Date**: 2026-07-08T15:00:00Z
**Bank**: `banks/helixllm_coder_memory.yaml`
**Analyzer**: `cmd/helixqa-verify-coder-memory`
**Scope**: 200 sequential requests to the live coder at :18434,
RSS sampled every 10 requests via `ps -o rss= -p <PID>`.

## Cases

| ID | Name | Priority | Verdict |
|---|---|---|---|
| MEMORY-MONO-001 | 200 sequential, monotonic-no-leak assertion | high | PENDING |
| MEMORY-GC-001 | 200 sequential, GC-point stability | high | PENDING |
| MEMORY-STEADY-001 | 200 sequential, steady-state plateau | medium | PENDING |
| MEMORY-SELF-VALIDATE-001-GOOD | Golden-good self-validation | critical | PENDING |
| MEMORY-SELF-VALIDATE-001-BAD | Golden-bad self-validation | critical | PENDING |

## Infrastructure

- **Coder**: llama.cpp on :18434 (HelixLLM sidecar)
- **Model**: HELIX_CODER_MODEL / llama3.2
- **Evidence root**: `qa-results/helixllm_coder_memory/` (gitignored raw corpus)
- **Evidence dir**: `docs/qa/phase1_helixqa_coder_memory_20260708T150000Z/`

## Run procedure

```bash
# 1. Confirm coder is up
curl -sS -o /dev/null -w '%{http_code}' \
  http://localhost:18434/v1/chat/completions \
  -d '{"model":"llama3.2","messages":[{"role":"user","content":"hi"}],"stream":false}'
# Expect 200

# 2. Build the analyzer
cd submodules/helix_qa && go build -o bin/helixqa-verify-coder-memory ./cmd/helixqa-verify-coder-memory/

# 3. Run each case
# MEMORY-MONO-001
mkdir -p qa-results/helixllm_coder_memory
bin/helixqa-verify-coder-memory \
  --n 200 --prompt "Count from 1 to 5." --interval 10 \
  --monotonic \
  --out qa-results/helixllm_coder_memory/mono_001_verdict.json \
  --conduit-dir qa-results/helixllm_coder_memory/conduit \
  --challenge-id MEMORY-MONO-001

# MEMORY-GC-001
bin/helixqa-verify-coder-memory \
  --n 200 --prompt "Count from 1 to 5." --interval 10 \
  --gc-stability \
  --out qa-results/helixllm_coder_memory/gc_001_verdict.json \
  --conduit-dir qa-results/helixllm_coder_memory/conduit \
  --challenge-id MEMORY-GC-001

# MEMORY-STEADY-001
bin/helixqa-verify-coder-memory \
  --n 200 --prompt "Count from 1 to 5." --interval 10 \
  --steady \
  --out qa-results/helixllm_coder_memory/steady_001_verdict.json \
  --conduit-dir qa-results/helixllm_coder_memory/conduit \
  --challenge-id MEMORY-STEADY-001

# MEMORY-SELF-VALIDATE-001-GOOD
bin/helixqa-verify-coder-memory \
  --n 20 --prompt "Count from 1 to 3." --interval 5 \
  --monotonic --gc-stability --steady \
  --out qa-results/helixllm_coder_memory/self_validate_001_golden_good_verdict.json \
  --conduit-dir qa-results/helixllm_coder_memory/conduit \
  --challenge-id MEMORY-SELF-VALIDATE-001-GOOD

# MEMORY-SELF-VALIDATE-001-BAD
bin/helixqa-verify-coder-memory \
  --n 20 --prompt "Count from 1 to 3." --interval 5 \
  --monotonic --gc-stability --steady \
  --leak-pct 0.0001 --expect-fail \
  --out qa-results/helixllm_coder_memory/self_validate_001_golden_bad_verdict.json \
  --conduit-dir qa-results/helixllm_coder_memory/conduit \
  --challenge-id MEMORY-SELF-VALIDATE-001-BAD

# 4. Curate evidence — copy verdict JSONs here
cp qa-results/helixllm_coder_memory/*_verdict.json \
  docs/qa/phase1_helixqa_coder_memory_20260708T150000Z/
```

## Evidence inventory

- `mono_001_verdict.json` — MEMORY-MONO-001 structured verdict
- `gc_001_verdict.json` — MEMORY-GC-001 structured verdict
- `steady_001_verdict.json` — MEMORY-STEADY-001 structured verdict
- `self_validate_001_golden_good_verdict.json` — golden-good self-validation
- `self_validate_001_golden_bad_verdict.json` — golden-bad self-validation
