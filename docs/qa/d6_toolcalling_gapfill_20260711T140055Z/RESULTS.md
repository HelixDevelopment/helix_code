# D6 Concurrent Tool-Calling Guard + GGUF-Revision Verification — RESULTS

| Property | Value |
|---|---|
| **Revision** | 1 |
| **Created** | 2026-07-11 |
| **Last modified** | 2026-07-11 |
| **Status** | active |
| **Track** | (T1/feature/helixllm-full-extension - claude3) |
| **Scope** | Closes `docs/qa/R41_EXTENSION_COVERAGE_LEDGER.md` items #5 (D6 tool-calling-under-load guard) and #6 (GGUF-revision verification) — §11.4.135 standing regression guard, §11.4.108 runtime-signature, §11.4.5/§11.4.69/§11.4.107 captured evidence. |

## Table of contents

- [1. Summary](#1-summary)
- [2. Task 1 — GGUF-revision verification](#2-task-1--gguf-revision-verification)
- [3. Task 2 — D6 concurrent-load guard](#3-task-2--d6-concurrent-load-guard)
- [4. Coder-untouched proof](#4-coder-untouched-proof)
- [5. Evidence index](#5-evidence-index)

## 1. Summary

This run closes two audit gaps flagged in `docs/qa/R41_EXTENSION_COVERAGE_LEDGER.md`:

- **Item #5 (D6)**: tool-calling type-validation under load had only a single unit
  test (`submodules/helix_llm/internal/gateway/tool_call_normalizer_test.go`
  `TestParseQwen3ToolCall_ArgumentsAsString`) plus one manual single-request probe.
  No N-concurrent bank against the LIVE coder existed, and no permanent §11.4.135
  regression guard was registered.
- **Item #6**: it was unverified whether the running GGUF (`Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf`
  on `helixllm-coder` at `:18434`) postdates Unsloth's tool-calling fix for
  Qwen3-Coder.

Both gaps are now closed with real, captured, live evidence (no mocks, no simulated
output — CONST-035 / §11.4 / §11.4.107).

## 2. Task 1 — GGUF-revision verification

### 2.1 Identify the running GGUF

`curl :18434/v1/models` (live, this session):

```json
{"data":[{"id":"/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf", ... ,
 "meta":{"n_params":30532122624,"size":18550716416,"ftype":"Q4_K - Medium", ...}}]}
```

`podman inspect helixllm-coder` (full output: `podman_inspect_helixllm-coder.txt`):

- **Model file**: `/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf` (bind-mounted
  read-only from host `/home/milos/models/`)
- **Server args**: `llama-server -m ... -ngl 99 -c 24576 --parallel 8 --cont-batching
  -fa on --cache-type-k q8_0 --cache-type-v q8_0 --jinja --metrics`
- **Image**: `localhost/helixllm/llamacpp-router:cuda12.8-sm120`, container image
  `Created: 2026-07-06 19:05:40`
- **Container State**: `Running: true`, `Pid: 1980342`, `StartedAt: 2026-07-11 17:55:17`

### 2.2 Host-side model-file provenance (`gguf_metadata_strings.txt`, `gguf_file_stat.txt`)

Read-only header scan of the GGUF file's embedded metadata (first 2 MB, safely
covers the GGUF key-value header section — never touches the container):

```
general.name              -> Qwen3-Coder-30B-A3B-Instruct
general.basename          -> Qwen3-Coder-30B-A3B-Instruct
general.quantized_by      -> Unsloth
general.repo_url          -> https://huggingface.co/unsloth
general.base_model.0.name -> (Qwen/Qwen3-Coder-30B-A3B-Instruct)
general.base_model.0.repo_url -> https://huggingface.co/Qwen/Qwen3-Coder-30B-A3B-Instruct
LICENSE reference          -> https://huggingface.co/Qwen/Qwen3-Coder-30B-A3B-Instruct/blob/main/LICENSE
```

Host filesystem timestamps (`stat`, read-only):

```
Modify: 2026-07-06 17:40:54  (local file write/download completion)
Birth:  2026-07-06 16:19:37  (local file creation/download start)
```

This confirms the running GGUF is `unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF`'s
`Q4_K_M` quant, quantized by Unsloth from `Qwen/Qwen3-Coder-30B-A3B-Instruct`.

### 2.3 Deep multi-angle research on the Unsloth tool-calling fix timeline (§11.4.99/§11.4.150)

Three independent angles, all fetched live this session:

1. **HuggingFace repo file history** (`unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF/tree/main`,
   fetched live): the `Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf` file itself —
   the exact quant this coder serves — was last modified on the HF repo
   **"5 months ago"** relative to this session (2026-07-11), i.e. **~February 2026**,
   via commit "Add files using upload-large-folder tool". This matches (and slightly
   predates, allowing for our own download/caching lag) our local file's
   Birth/Modify timestamps of 2026-07-06.

2. **Unsloth's official tool-calling fix announcement** (HF discussion
   `unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF/discussions/10`, "New Chat Template +
   Tool Calling Fixes as of 05 Aug, 2025", fetched live): Unsloth shipped a
   chat-template (embedded Jinja template) fix specifically addressing tool-calling
   inconsistencies on **2025-08-05**, stating prior fixes "only worked in certain
   setups, such as llama.cpp" and this update broadened + hardened the fix. A later
   community-reported edge case (April 2026: template crash when a tool schema
   lacks a `properties` field) was also surfaced in that thread — NOT applicable to
   this guard's `add` tool, which always declares `properties` (see
   `addToolSpec()` in the test file).

3. **Upstream bug-class citation** (`github.com/sst/opencode` issue **#1809**,
   filed **2025-08-11**, fetched live): documents the "array-as-string" bug class —
   `Qwen3-Coder-30B-A3B-Instruct` (served via vLLM/AWQ with
   `--tool-call-parser qwen3_coder`) returning an array-typed tool argument
   (`todos`) as a JSON-encoded STRING instead of a native array, tripping OpenCode's
   schema validator ("Expected array, received string"). This is the canonical bug
   class the D6 guard (§3 below) exercises against.

4. **Refresh-then-revert** (HF discussion `.../discussions/21`, "New Refresh with
   added Tool Calling in calibration dataset and improved imatrix", fetched live):
   a later re-quantization attempt was **reverted** by the maintainer after reports
   of quality regression — confirming the **current main-branch GGUF files (the ones
   this coder serves) do NOT carry that reverted, regressed refresh** and instead
   reflect the stable, fixed baseline from the 2025-08-05 chat-template fix.

### 2.4 Finding

**The running GGUF (`Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf`, `general.quantized_by=Unsloth`,
HF repo commit ~February 2026, locally cached 2026-07-06) postdates Unsloth's
2025-08-05 tool-calling / chat-template fix by roughly six months at the HF-repo
layer and eleven months at our local-cache layer.** No evidence of the reverted
"refresh" regression (`discussions/21`) shipping on main was found — the refresh
was rolled back before merging.

This provenance finding is corroborated by **live behavioral evidence**: both the
single-request probe (§2.5) and the 16-concurrent guard (§3) show the model
correctly emitting `arguments` as a well-typed, correctly-scoped JSON object on
every call — exactly the outcome the 2025-08-05 fix targeted, and the opposite of
the OpenCode #1809 array-as-string failure mode.

Sources (fetched live this session, 2026-07-11):
- https://huggingface.co/unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF/tree/main
- https://huggingface.co/unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF/discussions/10
- https://huggingface.co/unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF/discussions/21
- https://github.com/sst/opencode/issues/1809

### 2.5 Single-request live sanity probe (real captured output)

Request: `curl_8concurrent_probe/single_request_probe_request.json` (tool `add`,
`17 + 25`).

Response (`curl_8concurrent_probe/single_request_probe_response.json`, real,
captured this session):

```json
"tool_calls":[{"type":"function","function":{"name":"add","arguments":"{\"a\":17,\"b\":25}"},"id":"mTLmJpzf9Qt7bGF4zaDLXOtBNEVJGhHe"}]
```

`arguments` is a correctly-scoped JSON-object-as-string — the spec-correct shape,
not the array-as-string bug class.

## 3. Task 2 — D6 concurrent-load guard

### 3.1 New permanent regression test

Added `helix_code/internal/llm/tool_calling_concurrent_test.go` (package `llm`,
no build tag — zero-cost local loopback, participates in the default
`go test ./internal/llm/...` run, exactly matching the existing precedent
`internal/server/llm_generate_helixllm_live_test.go`):

- `TestToolCalling_ConcurrentLoad_ArgumentsAreTypedJSON` — fires **N=16**
  SIMULTANEOUS `POST /v1/chat/completions` tool-calling requests at the live coder
  (`:18434`, override via `HELIX_LLM_LOCAL_OPENAI_ENDPOINT`), each with a
  deterministic, per-request-unique `(a, b)` integer pair sent to a typed `add(a,b)`
  tool schema. For every response it asserts: (1) exactly one `tool_call`;
  (2) `function.name == "add"`; (3) `function.arguments` STRICT-decodes as a
  well-typed JSON object `{a:int, b:int}` (rejects bare arrays, double-encoded
  strings, and wrong-typed fields — the OpenCode #1809 bug-class analogue);
  (4) the decoded `(a, b)` match THIS request's own expected values, catching
  cross-contamination between concurrently in-flight requests under load.
  Honestly `t.Skip()`s (never fake-PASSes) if the coder is unreachable.
- `TestDecodeAddArguments_RejectsArrayAsStringBugClass` — pure unit,
  no network, always runs. Golden-good/golden-bad self-validation
  (§11.4.107(10)) of the strict decoder itself: proves a bare-array payload,
  a double-encoded string, and a wrong-typed field are ALL correctly rejected —
  so the guard's own analyzer cannot silently pass a regressed wire shape.

### 3.2 §1.1 paired-mutation note — what a regression would look like

If the coder's jinja chat-template, the llama.cpp tool-call formatter, or any
downstream normalizer regressed into the #1809 bug class, this test's assertions
would provably FAIL on real captured output:

- **(a) Bare-array regression**: `arguments` decodes to `[17,25]` instead of an
  object — `decodeAddArguments`'s strict struct-typed `json.Decoder` rejects this
  with a `json.UnmarshalTypeError` ("cannot unmarshal array into Go struct").
- **(b) Double-encoding regression**: `arguments` becomes a JSON string containing
  another JSON string (e.g. `"\"{\\\"a\\\":17,\\\"b\\\":25}\""`) — the strict
  decode fails on the outer layer because a JSON string is not a JSON object.
- **(c) Cross-contamination under load**: a queueing/batching bug in the coder's
  `--cont-batching` slot scheduler swaps two in-flight requests' outputs — the
  per-request expected-vs-actual `(a,b)` equality check (impossible to exercise
  with a single-request unit test) catches this.

`TestDecodeAddArguments_RejectsArrayAsStringBugClass` proves (a)/(b) are
mechanically rejected by the decoder today, via 5 golden-good/golden-bad cases
(1 good, 4 bad — all bad cases correctly produce a decode error).

### 3.3 Live run — real captured N=16-concurrent output

Command:

```
cd helix_code/helix_code && go test -v -count=1 \
  -run TestToolCalling_ConcurrentLoad_ArgumentsAreTypedJSON ./internal/llm/
```

Full raw output: `go_test_output/go_test_v_TestToolCalling_ConcurrentLoad_ArgumentsAreTypedJSON.log`.
Verbatim excerpt (all 16 requests, real, this session):

```
=== RUN   TestToolCalling_ConcurrentLoad_ArgumentsAreTypedJSON
    tool_calling_concurrent_test.go:402: PASS req[0] (expected a=1000,b=2000): arguments="{\"a\":1000,\"b\":2000}" -> decoded {a:1000 b:2000} in 932.574935ms
    tool_calling_concurrent_test.go:402: PASS req[1] (expected a=1007,b=2011): arguments="{\"a\":1007,\"b\":2011}" -> decoded {a:1007 b:2011} in 468.227977ms
    tool_calling_concurrent_test.go:402: PASS req[2] (expected a=1014,b=2022): arguments="{\"a\":1014,\"b\":2022}" -> decoded {a:1014 b:2022} in 931.569283ms
    ...
    tool_calling_concurrent_test.go:402: PASS req[15] (expected a=1105,b=2165): arguments="{\"a\":1105,\"b\":2165}" -> decoded {a:1105 b:2165} in 467.830466ms
    tool_calling_concurrent_test.go:417: D6 CONCURRENT-LOAD GUARD PASS: N=16 simultaneous tool-calling requests against live coder http://localhost:18434 — every tool_call.function.arguments decoded as a correctly-typed JSON object {a:int,b:int} (no array-as-string, no double-encoding, no cross-contamination). Wall-clock for all 16 concurrent requests: 932.999817ms
--- PASS: TestToolCalling_ConcurrentLoad_ArgumentsAreTypedJSON (0.93s)
PASS
ok  	dev.helix.code/internal/llm	0.942s
```

**16/16 requests**: correctly-typed JSON arguments, zero array-as-string, zero
cross-contamination. Wall-clock ~933 ms for all 16 concurrent requests against a
coder configured `--parallel 8 --cont-batching` — confirms the queue-then-serve
path (N=16 > configured parallelism 8) also preserves per-request argument
correctness, not just the in-slot-parallel path.

Also re-ran with `-race` (`go test -race -count=1 -run
TestToolCalling_ConcurrentLoad_ArgumentsAreTypedJSON ./internal/llm/`): **PASS**,
no data races detected in the test's own goroutine fan-out/fan-in.

### 3.4 8-concurrent curl cross-check (independent of the Go test, `curl_8concurrent_probe/`)

Before authoring the Go test, an independent 8-concurrent `curl` probe (bash
background jobs, `req_1.json`..`req_8.json` / `resp_1.json`..`resp_8.json`) was
run against the live coder with random distinct `(a,b)` pairs per request, all
`http_code=200`, wall-clock 0.68–0.69 s for all 8. Every response's `arguments`
field independently confirmed correctly-typed + correctly-scoped (no
cross-contamination), corroborating the Go test's own result via a fully
independent client implementation.

## 4. Coder-untouched proof

`podman inspect helixllm-coder` **before** this session's work and **after** all
probes + the Go test run (including `-race`) report the **identical PID
(`1980342`)** and **identical `StartedAt` (`2026-07-11 17:55:17.079512721 +0500`)**
— the coder container was never restarted, reconfigured, or otherwise touched
(§11.4.122). All requests in this run were stateless `GET /v1/models` and
`POST /v1/chat/completions` calls only.

## 5. Evidence index

```
docs/qa/d6_toolcalling_gapfill_20260711T140055Z/
├── RESULTS.md                                        (this file)
├── gguf_metadata_strings.txt                          (GGUF header provenance strings)
├── gguf_file_stat.txt                                 (host file stat — Birth/Modify dates)
├── podman_inspect_helixllm-coder.txt                  (container Args/Image/State — before+after identical)
├── curl_8concurrent_probe/
│   ├── single_request_probe_request.json / _response.json
│   └── req_1..8.json / resp_1..8.json                 (independent 8-concurrent curl cross-check)
└── go_test_output/
    └── go_test_v_TestToolCalling_ConcurrentLoad_ArgumentsAreTypedJSON.log  (full -v output, 16/16 PASS)
```

## Sources verified

Sources verified 2026-07-11:
- https://huggingface.co/unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF/tree/main
- https://huggingface.co/unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF/discussions/10
- https://huggingface.co/unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF/discussions/21
- https://github.com/sst/opencode/issues/1809
- https://platform.openai.com/docs/api-reference/chat/object (arguments field spec — cited in test-file doc comment)
