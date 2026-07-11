# R41-F — Consolidated Live Validation+Verification Evidence Summary

**Revision:** 1
**Created:** 2026-07-11
**Last modified:** 2026-07-11
**Status:** active
**Track:** (T1/feature/helixllm-full-extension - claude3)
**Scope:** §11.4.83 (docs/qa/ end-user evidence mandate) — one reader-friendly
index over the R41-F fleet's fresh LIVE evidence, produced 2026-07-11 after
the coder was booted rootless (§11.4.111 fix). This document is a
**read-only compilation**: nothing it cites was re-run, re-verified, or
modified to produce this index — every cell below is drawn directly from the
named `RESULTS.md` (or equivalent) already committed by its owning stream.

## Table of contents

- [1. Scope and method](#1-scope-and-method)
- [2. Evidence index](#2-evidence-index)
- [3. Per-capability detail](#3-per-capability-detail)
- [4. §11.4.111 rootless coder-boot fix](#4-114111-rootless-coder-boot-fix)
- [5. Honest boundary — what this document does NOT establish](#5-honest-boundary--what-this-document-does-not-establish)
- [6. Sources verified](#6-sources-verified)

## 1. Scope and method

On 2026-07-11 the HelixLLM coder (`Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf`,
llama.cpp OpenAI-compatible server, port `:18434`) was brought up **rootless**
after a CDI (Container Device Interface) stale-binding blocker was
root-caused and fixed (§11.4.111 — see §4). With the coder live, four
coder-independent-but-coder-exercising R41-F streams produced fresh,
committed, real-evidence `RESULTS.md` documents. This summary:

- Verifies each cited `RESULTS.md` (or evidence file) **exists on disk** and
  **skims its verdict/metrics** before citing it (§11.4.6 — no fabricated
  metric appears below).
- Cites the **exact evidence path + commit hash** for every claim so a
  reader can open the primary source directly.
- Marks anything not yet landed as **PENDING**, never invents a result
  (§11.4.6 / §11.4.123).
- Was produced **read-only**: no submodule, `/mnt/track1`, `adb`, secret-scan
  script, git-hook script, or `helix_code/internal/server` file was touched
  by this task; the only new file is this document itself.

## 2. Evidence index

| Capability | Live-proven? | Key metric | Evidence path (+ commit) | Analyzer self-validated? |
|---|---|---|---|---|
| Coder boot (rootless) | **YES** | Fresh nonce `HELIX-1783774679-17765` echoed verbatim by the live coder (`completion_tokens=20`) | `docs/qa/coder_boot_liveproof_20260711T125759Z/RESULTS.md` (+`response.json`) — **untracked at time of writing**, commit pending in the same batch as the boot-fix (`4e993f1a` covers the script; the liveproof dir itself is not yet committed) | N/A (raw nonce-echo probe, not a bank analyzer) |
| Dual-wire facade — full HTTP E2E | **YES** | 401 fail-closed (no auth) + OpenAI `/v1/chat/completions` 200 nonce-echo + Anthropic `/v1/messages` 200 nonce-echo + live tool-call wire-shape divergence (both shapes), all via real `curl`+`httptest`+router+middleware+coder | `docs/qa/dualwire_live_e2e_20260711T130459Z/RESULTS.md` — commit `8a309004` | N/A (live HTTP round-trip test, not a self-validating analyzer; asserts against real captured response bytes) |
| Lane-B multi-instance serving (co-resident) | **YES** | 163.67 tok/s mean / 163.34 tok/s median single-stream (matches prior non-co-resident baseline); 3/3 concurrent succeeded; coder `llama-server` PID identical (1980342) pre/post — never restarted | `docs/qa/lane_b_liveproof_20260711T130533Z/RESULTS.md` (+`benchmark_results.json`, `benchmark_script.py`) — commit `8244b33e` | N/A (live benchmark harness against real ports `:18434`/`:18435`, not a golden-good/golden-bad analyzer) |
| HelixQA coder-bench + concurrency banks | **YES** | **13/13 PASS, 0 FAIL** — p50/p95/p99 latency 152/171/187 ms @concurrency-1 scaling to 1076/2113/2128 ms @concurrency-100; throughput 269.7–624.2 tok/s; TTFT p50=3 ms; 50/50 concurrent requests HTTP 200, zero drops, zero duplicate responses | `docs/qa/helixqa_live_vnv_20260711T130200Z/RESULTS.md` — commit `930cfddb` | **YES** — golden-good/golden-bad fixture pair for both analyzers (§11.4.107(10)), PLUS a **fresh live §1.1 mutation check** performed in-session (forced `Pass=true`, verified the mutated binary flips golden-bad's `case_result` false→ FAIL, then reverted with `git diff --stat` empty) |
| Generative (image/video) live-proof | **PENDING** | Not yet on disk at time of writing (`docs/qa/generative_liveproof_*` absent; `find docs/qa -maxdepth 1 -iname "*generative*"` returns nothing) | N/A — tracked in the SDD ledger (`.superpowers/sdd/progress.md` §"R41-F") as stream `ad9a782848e068b2a`, dispatched as a GPU-freed backfill after Lane-B teardown, status "BACKFILL ... generative" at time of writing, not yet returned | N/A (not yet produced) |

## 3. Per-capability detail

### 3.1 Coder boot (rootless)

`docs/qa/coder_boot_liveproof_20260711T125759Z/RESULTS.md` (2 lines) records:

```
nonce=HELIX-1783774679-17765
PASS: nonce echoed
```

The accompanying `response.json` in the same directory shows the full raw
OpenAI-compatible chat-completion response from
`/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf` echoing that exact nonce
(`"content":"HELIX-1783774679-17765"`), with real timing fields
(`prompt_ms`, `predicted_ms`, `predicted_per_second≈285`) — a genuine,
non-cached model response, not a fixture. This directory is **currently
untracked** (`git status` shows `??`); it was produced by the coder-boot
stream in the same session and is expected to land in the same commit batch
as the `scripts/boot_coder_cdi.sh` rootless fix (`4e993f1a`, already
committed — see §4).

### 3.2 Dual-wire facade — full HTTP E2E

`docs/qa/dualwire_live_e2e_20260711T130459Z/RESULTS.md` (commit `8a309004`)
proves the complete live path
`curl → gin router → wireFacadeAuthMiddleware → chatCompletions/
anthropicMessages → resolveLLMProvider("helixllm"/"local") →
llm.OpenAICompatibleProvider → http://localhost:18434 (live coder) → real
answer → back through the same translation layer`:

- **401** on an unauthenticated request (fail-closed, confirmed before the
  request ever reaches provider resolution).
- **200 + nonce echo** on both `/v1/chat/completions` (OpenAI shape) and
  `/v1/messages` (Anthropic shape), each with a **fresh, independently
  generated nonce** (`HELIXCODE-FULLHTTP-E2E-203036440e6c` and
  `...-f6a30d57b156`) echoed verbatim by the same live coder.
- **Live tool-call wire-shape divergence**: the same coder, given the same
  `get_weather` tool definition and a nonce-bearing prompt, was driven
  through both wire shapes — OpenAI's `function.arguments` renders as a
  JSON-**encoded string**, Anthropic's `content[].input` renders as a JSON
  **object** — both carrying the identical live nonce
  (`HELIXCODE-FULLHTTP-E2E-292679c493a1`), proving the divergence is a
  genuine translation of one underlying `llm.ToolCall`, not two
  independently fabricated responses.
- **Coder-untouched confirmation**: every call was a read-only
  `GET /v1/models` or `POST /v1/chat/completions` — no config write, no
  restart (§11.4.122).
- **Key-leak-free confirmation**: only a test-only, self-documented
  non-secret key was used; `grep` for real credential patterns over the
  report and raw evidence returned no hits.

### 3.3 Lane-B multi-instance serving (co-resident with the coder)

`docs/qa/lane_b_liveproof_20260711T130533Z/RESULTS.md` (commit `8244b33e`,
plus `benchmark_results.json` raw data and `benchmark_script.py` the exact
harness executed) proves Mistral-Nemo-Instruct-2407-Q4_K_M (Lane-B, port
`:18435`) was booted **co-resident** with the live 30B coder (port
`:18434`) via `submodules/helix_llm/cmd/agentgen-boot`, rootless podman +
NVIDIA CDI, on the shared RTX 5090:

- **Admission**: live `nvidia-smi` read immediately before boot — free
  12650 MiB ≥ need 9216 MiB + 2048 MiB headroom → `ADMIT-OK`.
- **Co-residence**: both `:18434` and `:18435` answered `GET /v1/models`
  simultaneously post-boot; the coder's `llama-server` PID (`1980342`) was
  **identical** before and after Lane-B's full admit→boot→benchmark→teardown
  cycle — mechanical proof the coder was never restarted.
- **Single-stream throughput**: mean 163.67 tok/s / median 163.34 tok/s —
  matches the 2026-07-08 non-co-resident baseline, i.e. co-residence did not
  degrade Lane-B's own throughput.
- **3-parallel concurrent**: 3/3 succeeded, 2.73 s wall-clock, per-request
  tok/s dropping ~2.2× under 3-way contention (consistent with
  continuous-batching sharing one GPU).
- **Tool-calling/arithmetic correctness**: `"What is 2+2?"` → `"4"` via the
  chat-templated endpoint — a genuine methodology finding is documented
  in-report (§11.4.102): the raw `/v1/completions` continuation endpoint
  returned empty for a closed question, root-caused as a
  test-harness-endpoint mismatch (not a Lane-B defect), then fixed by
  switching to `/v1/chat/completions`.
- **Teardown**: VRAM restored to ~12.6 GiB free (16 MiB delta = normal
  driver noise), Lane-B container/port fully removed, coder PID unchanged
  throughout.

### 3.4 HelixQA — coder-bench + coder-concurrency banks, live against the coder

`docs/qa/helixqa_live_vnv_20260711T130200Z/RESULTS.md` (commit `930cfddb`)
re-ran two HelixQA banks live against the resident coder with freshly
rebuilt analyzers (`bin/helixqa-verify-coder-bench`,
`bin/helixqa-verify-coder-concurrency`, both built from unmodified current
source immediately before use — §11.4.108 SOURCE→ARTIFACT):

- **`helixllm_coder_bench.yaml`**: 7/7 PASS — p50/p95/p99 latency and
  tok/s at graduated concurrency (1/10/50/100), TTFT streaming case
  (p50=3 ms, p95=4 ms, p99=4 ms, well under the 5000 ms budget), plus a
  golden-good and a golden-bad self-validation case.
- **`helixllm_coder_concurrency.yaml`**: 6/6 PASS — all-ok / no-loss /
  nonce-integrity / response-consistency confirmed across 50 concurrent
  requests (0 drops, 0 duplicate/canned responses), plus golden-good and
  golden-bad self-validation.
- **Self-validation (§11.4.107(10))**: both analyzers' golden-bad fixtures
  genuinely produced a raw `pass=false` (never a hardcoded/metadata-only
  true), correctly inverted to a case PASS only because the bank declares
  `--expect-fail`.
- **Fresh live §1.1 mutation check** (not merely cited from the bank's
  2026-07-08 authoring metadata): both analyzers were rebuilt with an
  unconditional `Pass = true` line injected immediately before the
  `ExpectFail` inversion, run against their golden-bad fixtures, and shown
  to flip `case_result` from PASS to **FAIL** — proving the assertion logic
  is genuinely load-bearing. The source files were reverted from `/tmp`
  backups immediately after, confirmed by an empty `git diff --stat` and no
  `MUTATED` residue (§11.4.84 working-tree quiescence held throughout).
- **Coder-untouched**: same PID (`1980342`) and identical `/health` +
  `/props` responses before and after the full 13-case run; every call was
  read-only.
- **Scope note** (stated honestly in the source report): DDoS, chaos,
  memory, and race dimensions for the coder are covered by separate banks
  not re-run in this task — those were previously live-run per
  `docs/qa/phase1_providers_rerun_20260708T204553Z/` and prior commits.

### 3.5 Generative (image/video) live-proof — PENDING

No `docs/qa/generative_liveproof_*` directory exists on disk at the time of
writing (`find docs/qa -maxdepth 1 -iname "*generative*"` returns nothing,
and `find docs/qa -maxdepth 1 -type d -newer
docs/qa/helixqa_live_vnv_20260711T130200Z` also returns nothing, confirming
no newer generative-evidence directory has landed since the last confirmed
evidence in this fleet). Per the SDD ledger
(`.superpowers/sdd/progress.md`, section "R41-F"), this capability is owned
by background stream `ad9a782848e068b2a`, dispatched as a GPU-freed backfill
immediately after Lane-B's teardown released VRAM, described as:

> "fast-lane GENERATIVE live-proof (FLUX-schnell/WAN fits free ~12.6GiB,
> coder untouched, §11.4.107 self-validated analyzer, honest-BLOCKED if
> HF_TOKEN-gated model absent)"

**This row is marked PENDING per §11.4.6 — no metric is claimed or
invented for it.** When that stream lands its `RESULTS.md`, this summary
should be updated (or superseded) to fill in the row.

## 4. §11.4.111 rootless coder-boot fix

The coder was down at the start of the 2026-07-11 session
(`podman start helixllm-coder` failing). Root cause, per the SDD ledger and
confirmed by reading the committed fix script
(`scripts/boot_coder_cdi.sh`, commit `4e993f1a`,
*"fix(§11.4.111): make coder-boot fully rootless — CDI to ~/.config/cdi +
CDI_SPEC_DIRS (no root/su/sudo)"*):

- The coder container binds its GPU via the CDI name `nvidia.com/gpu=all`.
- The CDI spec was **pinned to a stale device path** (`/dev/dri/card0`) from
  before a host reboot re-enumerated DRM devices to `card1` — a textbook
  §11.4.111 "resolve by stable name, not enumeration index" violation.
- No CDI spec had been (re)generated at all (`nvidia-ctk cdi list` reported
  0 devices; `/etc/cdi` and `/var/run/cdi` were absent).
- **Fix, entirely rootless** (no `su`/`sudo` needed): `nvidia-ctk cdi
  generate --output=~/.config/cdi/nvidia.yaml` regenerates a fresh,
  correctly-resolved spec into the user's own XDG config dir, then
  `CDI_SPEC_DIRS=~/.config/cdi podman start helixllm-coder` points the
  rootless podman invocation at it — no `/etc/cdi` write, no privilege
  escalation, no image rebuild, no container recreation.
- The rewritten script is a **permanent boot-guard**: it is safe to
  re-run on any future DRM re-enumeration.
- Per the SDD ledger, the operator's `su` password was offered but **not
  stored** by the agent (§11.4.10/CONST-042 leak-avoidance) — the rootless
  path made storing/using it unnecessary. The ledger further notes the
  operator may wish to rotate that password out of caution since it briefly
  appeared in a transcript; that rotation decision is the operator's, not
  restated as fact here beyond what the ledger records.

Result: the coder came up **rootless**, live at `:18434`, and its liveness
was immediately confirmed by a fresh nonce echo (§3.1) before any of the
other three live-proven streams began exercising it.

## 5. Honest boundary — what this document does NOT establish

Per §11.4.6 (no-guessing) and §11.4.123 (rock-solid-proof-or-research), this
summary is explicit about its limits:

- **This document re-ran nothing.** Every metric above is transcribed from
  an already-committed `RESULTS.md` (or, for the coder-boot capability, an
  already-produced but not-yet-committed evidence pair). If any underlying
  evidence file is later found to be inaccurate, this summary inherits that
  inaccuracy — it is an index, not an independent re-verification.
- **Generative capability is genuinely PENDING**, not degraded-but-passing.
  No claim is made about FLUX/WAN2 image/video generation working or not
  working as of this document's creation.
- **DDoS, chaos, memory, and race dimensions of the §11.4.169 coverage
  enumeration were NOT re-run in this fleet** for the coder — the HelixQA
  report explicitly scopes itself to benchmarking/performance and
  concurrency/atomicity only, citing prior separate bank runs
  (`docs/qa/phase1_providers_rerun_20260708T204553Z/` and earlier commits)
  for those other dimensions. This summary does not claim otherwise.
- **Operator-gated items remain open** (per the SDD ledger, outside this
  task's scope to resolve): constitution submodule push, root/submodule
  push to all upstreams (§11.4.113 ff-only, CONST-043 explicit approval),
  rotation of the previously-leaked Gemini API key and the HF token exposed
  in a transcript, merge-to-main + production tag (§11.4.167/§11.4.151),
  §11.4.185 manual QA-team final confirmation, and the cerebras
  dead-code-removal / replicate-provider-wiring decisions. None of these
  are addressed or resolved by this document — it only indexes the
  already-committed live evidence produced so far.
- **The coder-boot evidence directory is currently untracked** (`??` in
  `git status`) — it is cited here because it exists on disk and its
  content was verified, but it had not yet been committed by its owning
  stream at the time this summary was written. Its commit hash is therefore
  listed as "commit pending" rather than a concrete SHA.
- No secret values, API keys, or tokens are reproduced anywhere in this
  document (§11.4.10) — all evidence that mentions redacted credentials in
  the source `RESULTS.md` files is summarized here without those values.

## 6. Sources verified

- `docs/qa/coder_boot_liveproof_20260711T125759Z/RESULTS.md` +
  `response.json` — read directly, 2026-07-11 (this session).
- `docs/qa/dualwire_live_e2e_20260711T130459Z/RESULTS.md` — read directly,
  commit `8a309004` confirmed present via `git log`/`git show --name-only`.
- `docs/qa/lane_b_liveproof_20260711T130533Z/RESULTS.md` — read directly,
  commit `8244b33e` confirmed present via `git log`/`git show --name-only`.
- `docs/qa/helixqa_live_vnv_20260711T130200Z/RESULTS.md` — read directly,
  commit `930cfddb` confirmed present via `git log`/`git show --name-only`.
- `scripts/boot_coder_cdi.sh` — read directly, commit `4e993f1a` confirmed
  present via `git log`/`git show --stat`.
- `.superpowers/sdd/progress.md` (section "R41-F") — read directly for the
  narrative context (fleet composition, generative-stream status, rootless
  boot root-cause) attributed explicitly to the ledger throughout this
  document.
- `find docs/qa -maxdepth 1 -iname "*generative*"` and
  `find docs/qa -maxdepth 1 -type d -newer
  docs/qa/helixqa_live_vnv_20260711T130200Z` — both run 2026-07-11, both
  empty, confirming the generative-evidence directory genuinely does not
  yet exist (not merely unsearched).
