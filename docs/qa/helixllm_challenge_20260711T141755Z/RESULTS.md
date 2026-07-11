# HelixLLM Coder Live End-to-End Challenge — RESULTS

**Run ID:** `helixllm_challenge_20260711T141755Z`
**Date (UTC):** 2026-07-11T14:17:55Z – 2026-07-11T14:18:01Z
**Purpose:** Close the CONST-050(B) / §11.4.169 mandatory test-type gap flagged by the
audit as **Challenges: NOT FOUND** for the HelixLLM extension. This run adds and
executes a real, end-to-end anti-bluff Challenge that proves a HelixLLM capability
(a live coder model answering a real coding request) genuinely works for an end
user, per §11.4 / §107 (Article XI §11.9 forensic anchor).

**Challenge script (new):**
`submodules/challenges/challenges/scripts/helixllm_coder_live_e2e_challenge.sh`
(registered in the submodule's own inventory — `challenges_describe_challenge.sh`
meta-gate: 25/25 PASS after the addition, script count 21 → 22, doc row added to
`docs/test-coverage.md`).

**Target:** live Qwen3-Coder-30B-A3B-Instruct (llama.cpp / OpenAI-compatible
`/v1/chat/completions`) at `http://localhost:18434`, PID 1980342, auto-discovered
via `GET /v1/models` — no hardcoded model id (CONST-036).

---

## What the Challenge actually does

1. Preflight: verifies `curl`/`jq`/`python3` present; verifies the coder is
   reachable via `GET /v1/models`; auto-discovers the model id (never
   hardcoded).
2. Builds a **realistic end-user coding task**: "write and verify a function" —
   specifically `two_sum(nums, target)`, the classic real-world coding-interview
   task, sent as a real chat-completion request.
3. Generates ONE freshly-randomised 4th test vector at run time (in addition to
   3 fixed vectors) so a hardcoded lookup-table "solution" could never pass.
4. Sends a **real HTTP POST** to the live coder and captures the exact request
   and response bodies verbatim.
5. Extracts the code from the response and runs a **static bluff scan**
   (TODO / simulate / NotImplementedError / placeholder / "for now" / stub
   markers) — reject on any match.
6. **Actually executes** the extracted code with a real `python3` interpreter
   against all 4 assertions (not merely "no exception raised" — the returned
   indices must be correct and sum to the target).
7. Only PASSes if the coder's code is bluff-free AND genuinely produces correct
   results under real execution.
8. Anti-bluff paired mutation (`--anti-bluff-mutate`, §1.1): bypasses the live
   coder, plants a deliberately-wrong stub (`return [0, 0]` for every input),
   and runs it through the **identical** extraction → scan → execution →
   assertion pipeline, proving the checker itself is not a rubber stamp.

---

## RUN 1 — Paired mutation (anti-bluff self-proof)

Command:
```
HELIXLLM_CHALLENGE_EVIDENCE_DIR=".../evidence_mutation" \
  bash submodules/challenges/challenges/scripts/helixllm_coder_live_e2e_challenge.sh --anti-bluff-mutate
```

Full transcript (verbatim stdout):
```
=== HelixLLM Coder Live End-to-End Challenge ===
  base_url=http://localhost:18434 timeout=90s mutate=1
[0/8] Coder reachable: http://localhost:18434
[0/8] model=/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf (source: auto-discovered)
[0/8] evidence_dir=/home/milos/Factory/projects/tools_and_research/helix_code/docs/qa/helixllm_challenge_20260711T141755Z/evidence_mutation
[1/8] randomised 4th vector: nums=[315,58,238,-63] target=553 expect_indices=(0,2)
[2/8] request built (model=/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf, run_token=helixllm20260711T141755Z3051025)
[MUT] planting deliberately-broken stub (two_sum -> constant [0, 0])
[MUT] executing broken stub through real assertion pipeline...
CASE fixed_1: FAIL nums=[2, 7, 11, 15] target=9 got=[0, 0] expect_indices=[0, 1]
CASE fixed_2: FAIL nums=[3, 2, 4] target=6 got=[0, 0] expect_indices=[1, 2]
CASE fixed_3: FAIL nums=[3, 3] target=6 got=[0, 0] expect_indices=[0, 1]
CASE random_4: FAIL nums=[315, 58, 238, -63] target=553 got=[0, 0] expect_indices=[0, 2]
OVERALL: FAIL
  mutation correctly detected — broken stub FAILED the real-execution assertions
=== HelixLLM Coder Challenge: MUTATION DETECTED (anti-bluff OK) ===
```

**Exit code: 99** (mutation correctly detected — proves the checker genuinely
catches a wrong implementation instead of rubber-stamping any response).

Note: `evidence_mutation/request.json` was built but intentionally **never sent**
in this run — the mutation path bypasses the live coder by design (that is the
point of the paired-mutation proof: it isolates the checker itself from the
model). Only `evidence_real/` below carries a genuine sent/received HTTP
transcript.

---

## RUN 2 — Real end-to-end vs the LIVE coder

Command:
```
HELIXLLM_CHALLENGE_EVIDENCE_DIR=".../evidence_real" \
  bash submodules/challenges/challenges/scripts/helixllm_coder_live_e2e_challenge.sh
```

Full transcript (verbatim stdout):
```
=== HelixLLM Coder Live End-to-End Challenge ===
  base_url=http://localhost:18434 timeout=90s mutate=0
[0/8] Coder reachable: http://localhost:18434
[0/8] model=/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf (source: auto-discovered)
[0/8] evidence_dir=/home/milos/Factory/projects/tools_and_research/helix_code/docs/qa/helixllm_challenge_20260711T141755Z/evidence_real
[1/8] randomised 4th vector: nums=[76,-410,-73,-291] target=-701 expect_indices=(3,1)
[2/8] request built (model=/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf, run_token=helixllm20260711T141801Z3052234)
[3/8] POST http://localhost:18434/v1/chat/completions (real end-user coding task)...
[3/8] HTTP 200 received
[4/8] response content: 217 chars
  usage.completion_tokens=55
[5/8] extracted 7 lines of code -> .../evidence_real/solution.py
[6/8] static bluff scan: clean (no TODO/simulate/NotImplementedError/placeholder markers)
[7/8] executing extracted code with python3 against 4 real assertions (3 fixed + 1 random)...
CASE fixed_1: PASS nums=[2, 7, 11, 15] target=9 got=[0, 1] expect_indices=[0, 1]
CASE fixed_2: PASS nums=[3, 2, 4] target=6 got=[1, 2] expect_indices=[1, 2]
CASE fixed_3: PASS nums=[3, 3] target=6 got=[0, 1] expect_indices=[0, 1]
CASE random_4: PASS nums=[76, -410, -73, -291] target=-701 got=[1, 3] expect_indices=[1, 3]
OVERALL: PASS
[7/8] all 4 real-execution assertions PASSED (see harness_stdout.txt)

=== HelixLLM Coder Challenge: PASSED ===
  evidence: .../evidence_real
  model=/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf completion_tokens=55 assertions=4/4
```

**Exit code: 0** (real, working code from the live coder — genuinely executed
and verified correct, including a fresh random vector generated at run time).

### Bidirectional transcript — exact HTTP request sent

`evidence_real/request.json`:
```json
{
    "model": "/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf",
    "messages": [
        {
            "role": "system",
            "content": "You are a precise Python coding assistant. Output ONLY a single fenced ```python code block containing exactly one function definition named two_sum(nums, target). No prose, no explanation, no example usage outside the code block."
        },
        {
            "role": "user",
            "content": "Write a Python function `two_sum(nums, target)` that returns a list of the two 0-based indices of the two distinct elements in `nums` that add up to `target`. Assume exactly one valid pair exists and you must not reuse the same index twice. Return ONLY a fenced python code block with the function definition — no tests, no explanation, no extra text."
        }
    ],
    "temperature": 0.1,
    "max_tokens": 500
}
```

### Bidirectional transcript — exact HTTP response received

`evidence_real/response.json`:
```json
{
    "choices": [
        {
            "finish_reason": "stop",
            "index": 0,
            "message": {
                "role": "assistant",
                "content": "```python\ndef two_sum(nums, target):\n    seen = {}\n    for i, num in enumerate(nums):\n        complement = target - num\n        if complement in seen:\n            return [seen[complement], i]\n        seen[num] = i\n```"
            }
        }
    ],
    "created": 1783779481,
    "model": "/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf",
    "system_fingerprint": "b1-3b4fca1",
    "object": "chat.completion",
    "usage": {
        "completion_tokens": 55,
        "prompt_tokens": 132,
        "total_tokens": 187,
        "prompt_tokens_details": { "cached_tokens": 16 }
    },
    "id": "chatcmpl-4IP2PoxssFKLjkYlQn3VgcbaG4ZRtbxE",
    "timings": {
        "cache_n": 16, "prompt_n": 116, "prompt_ms": 43.804,
        "predicted_n": 55, "predicted_ms": 181.743, "predicted_per_second": 302.63
    }
}
```

### Extracted code (`evidence_real/solution.py`)

```python
def two_sum(nums, target):
    seen = {}
    for i, num in enumerate(nums):
        complement = target - num
        if complement in seen:
            return [seen[complement], i]
        seen[num] = i
```

This is a genuinely correct hash-map two-sum implementation — not a stub, not
a memorized lookup table, not a `pass`/`NotImplementedError` placeholder. It was
verified by real execution, not by inspection alone.

### Real execution output (`evidence_real/harness_stdout.txt`)

```
CASE fixed_1: PASS nums=[2, 7, 11, 15] target=9 got=[0, 1] expect_indices=[0, 1]
CASE fixed_2: PASS nums=[3, 2, 4] target=6 got=[1, 2] expect_indices=[1, 2]
CASE fixed_3: PASS nums=[3, 3] target=6 got=[0, 1] expect_indices=[0, 1]
CASE random_4: PASS nums=[76, -410, -73, -291] target=-701 got=[1, 3] expect_indices=[1, 3]
OVERALL: PASS
```

---

## Coder-untouched verification (§11.4.122 — read-only, never restarted)

```
BEFORE:
    PID ELAPSED CMD
1980342    4953 llama-server -m /models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf ...

AFTER (post both runs):
    PID ELAPSED CMD
1980342    4971 llama-server -m /models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf ...

GET /v1/models post-run: HTTP 200, same model listed.
```

Same PID (1980342) throughout, elapsed time monotonically increasing
(4953s → 4971s = 18s wall clock for both Challenge runs), never restarted or
signalled — the script only ever issued `GET`/`POST` HTTP requests.

---

## Meta-gate: submodule inventory still green after the addition

```
$ cd submodules/challenges && bash challenges_describe_challenge.sh
...
Section 4: challenges/scripts/*.sh inventory + sh -n parseability
  PASS: challenges/scripts directory present
  PASS: challenges/scripts has 22 scripts (>=16)
  PASS: all 22 scripts parse clean under bash -n
  PASS: all 22 scripts are executable
  PASS: challenges/baselines/bluff-baseline.txt present
...
=== Summary: 25/25 PASS, 0 FAIL ===
```

(Script count 21 → 22 after adding `helixllm_coder_live_e2e_challenge.sh`;
`docs/test-coverage.md` §4 table updated with a documentation row for the new
script.)

---

## VERDICT

| Check | Result |
|---|---|
| Challenges submodule located | `submodules/challenges/` (`digital.vasic.challenges`, downstream consumers per its `CLAUDE.md`: **HelixLLM**, HelixQA, LLMsVerifier) |
| Challenge authored following submodule convention | YES — `challenges/scripts/<name>_challenge.sh`, env-var-driven target (§11.4.28 decoupled, no hardcoded host), honest `SKIP-OK` on missing toolchain/unreachable target, `--anti-bluff-mutate` paired-mutation flag, exit codes `0`/`1`/`99`/`2` matching sibling scripts (`persistent_memory_challenge.sh`, `ddos_health_flood_challenge.sh`) |
| Real end-to-end PASS vs live coder, captured transcript | YES — exit 0, full `request.json`/`response.json`/`solution.py`/`harness_stdout.txt` captured under `evidence_real/` |
| Anti-bluff: real output, not a stub | YES — static bluff scan clean AND genuine `python3` execution against 3 fixed + 1 freshly-randomised vector, all 4/4 correct; paired mutation independently proves the checker rejects a wrong stub (exit 99) |
| Coder untouched | YES — same PID 1980342 before/after, never restarted/signalled, only `GET`/`POST` HTTP calls issued |
| Self-driving / re-runnable (§11.4.98) | YES — no manual intervention; re-running produces a fresh random 4th vector and a fresh live-model response each time |

**Overall: PASS.** The CONST-050(B) / §11.4.169 Challenges-coverage gap for the
HelixLLM extension is closed with a real, working, anti-bluff Challenge.

## Sources verified
No external documentation dependency — this Challenge targets an already-running
in-repo llama.cpp server via its local OpenAI-compatible HTTP API (verified live
at `http://localhost:18434/v1/models` during this session, 2026-07-11).
