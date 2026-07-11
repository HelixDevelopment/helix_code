# HelixQA Gap-Fill vs Live 30B Coder — §11.4.108 / §11.4.169

**Run ID:** `helixqa_gapfill_live30b_20260711T140607Z`
**Date (UTC):** 2026-07-11T14:06:07Z – 2026-07-11T14:20Z (approx.)
**Track:** T1 / `feature/helixllm-full-extension`
**Coder under test:** `bin/llama-server` (llama.cpp), PID `1980342`, listening on
`:18434`, serving `/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf`
(30,532,122,624 params, Q4_K_M quant, `n_ctx=3072`). Confirmed LIVE via
`GET /v1/models` before and after every bank run (see
`evidence/../coder_models_before.json` / `coder_models_after.json`).
**Scope:** `submodules/helix_qa` (banks + analyzers) + root `docs/qa/`. Coder
kept READ-ONLY throughout (§11.4.122) — never restarted, never signalled.
`submodules/helix_agent`, `/mnt/track1`, and the helix_llm A2A stream
(`:18441`) were never touched.

---

## GAP 1 (§11.4.108 clean-target re-run) — DDoS / Chaos / Memory vs live 30B

**Prior state:** these three banks previously executed against a
placeholder/substitute model (`model: "llama3.2"` in the request payload —
a decorative field llama.cpp does not enforce, but the verdict evidence
itself claimed a different model than whatever was actually resident at
the time). That is a §11.4.108 SOURCE→ARTIFACT→RUNTIME clean-target gap:
the bank config (port `:18434`) never changed, but WHAT actually answered
behind that port at verification time was not the production 30B.

**Fix applied:** re-ran every real test case in all three banks against the
CONFIRMED-live Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf, with
`HELIX_CODER_MODEL=/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf` so the
verdict JSON's `model` field now honestly reflects the model actually
addressed. `cmd/helixqa-verify-coder-chaos/main.go` previously hardcoded
`Model: "llama3.2"` with no verdict field at all recording it — patched to
source the model from `HELIX_CODER_MODEL` (matching the sibling ddos/
memory/concurrency analyzers) and added a `model` field to its verdict
struct for evidence completeness (§11.4.6 honest metrics). `ddos` and
`memory` already supported the env var; no logic change needed there.

### DDoS bank (`banks/helixllm_coder_ddos.yaml`) — **3/3 PASS**

| Case | Mode | N | Result | Key evidence |
|---|---|---|---|---|
| DDOS-CODER-001 | burst | 200 | **PASS** | `n_ok=200 dropped=0 5xx=0`, 19.1s wall |
| DDOS-CODER-002 | soak | 500 / 30s | **PASS** | `n_ok=500 dropped=0 5xx=0`, 30.0s wall |
| DDOS-CODER-003 | conn-flood | 100 | **PASS** | `n_ok=100 dropped=0 5xx=0`, 2.3s wall |

Evidence: `evidence/ddos/{burst_001,soak_001,conn_flood_001}_verdict.json`.

### Chaos bank (`banks/helixllm_coder_chaos.yaml`) — **5/5 PASS**

| Case | Mode | Result | Key evidence |
|---|---|---|---|
| CODER-CHAOS-001 | port-flood (n=50, burst=10) | **PASS** | `recovered=true post_flood_status=200` |
| CODER-CHAOS-002 | oversized-prompt (200 KiB) | **PASS** | `oversized_status=400` — real llama.cpp `exceed_context_size_error` (`n_prompt_tokens=45529 > n_ctx=3072`), `post_oversized_ok=true` |
| CODER-CHAOS-003 | concurrent-health (5 POST + 3 health) | **PASS** | `all_ok=true models_ok=true n_models_ok=3` |
| CODER-CHAOS-SELF-VALIDATE-001-GOOD | port-flood healthy | **PASS** | `pass=true recovered=true` |
| CODER-CHAOS-SELF-VALIDATE-001-BAD | port-flood, unreachable :19999, `--expect-fail` | **PASS** (case_result via inversion) | `pass=false recovered=false` → `case_result=true` |

Notable real evidence: CODER-CHAOS-002's oversized-prompt case returned the
30B's genuine llama.cpp context-overflow error body — a signature that is
model/config-specific (the substitute run never exercised this exact
`n_ctx=3072` ceiling), proving this run genuinely hit the production
coder's real context window, not a stand-in.

Evidence: `evidence/chaos/{port_flood_001,oversized_001,concurrent_health_001,self_validate_001_golden_good,self_validate_001_golden_bad}_verdict.json`.

### Memory bank (`banks/helixllm_coder_memory.yaml`) — **5/5 PASS**

| Case | N | Result | Key evidence |
|---|---|---|---|
| MEMORY-MONO-001 | 200 seq. | **PASS** | PID `1980342`, RSS `2961740→2961756 KB` (Δ+16 KB, 0.0005%), `monotonic_no_leak=true` |
| MEMORY-GC-001 | 200 seq. | **PASS** | RSS `2961756→2961772 KB`, `gc_stability=true` |
| MEMORY-STEADY-001 | 200 seq. | **PASS** | RSS `2961772→2961784 KB`, `steady_state=true` |
| MEMORY-SELF-VALIDATE-001-GOOD | 20 seq. | **PASS** | all 3 dimensions true |
| MEMORY-SELF-VALIDATE-001-BAD | 20 seq., `--leak-pct -1`, `--expect-fail` | **PASS** (case_result via inversion) | `monotonic_no_leak=false gc_stability=false` → `case_result=true` |

All three real cases were captured against the SAME PID (`1980342`) across
~600 sequential real completions against the 30B, RSS essentially flat
(≈2.96 GB resident, <0.01% drift end-to-end) — real physical evidence of
no memory leak under the production model, not the previous substitute.

Evidence: `evidence/memory/{mono_001,gc_001,steady_001,self_validate_001_golden_good,self_validate_001_golden_bad}_verdict.json`.

**GAP 1 verdict: CLOSED.** All 13 cases across the three banks PASS with
real captured evidence against the confirmed-live 30B. No bank required a
SKIP — every case retargeted cleanly.

---

## GAP 2 (§11.4.169 race NOT-FOUND) — new `helixllm_coder_race` bank authored + live-run

**New files (in-scope, `submodules/helix_qa` only):**
- `banks/helixllm_coder_race.yaml` — 7 test cases (3 real + 2 self-validation pairs)
- `cmd/helixqa-verify-coder-race-llm/main.go` — new RUNNABLE analyzer

This is explicitly **distinct** from the pre-existing
`banks/helixcode_coder_race.yaml` / `cmd/helixqa-verify-coder-race`
scaffold, which targets the **HelixCode HTTP server's** `ModelManager`
`sync.RWMutex` on `:8080` (a Go map/mutex correctness bug class). That
scaffold remains untouched and still un-run — it is a separate,
still-open gap outside this task's scope (booting `bin/helixcode` on
`:8080` was not authorized and was not part of the GAP 2 brief, which
named `:18434`/the LLM coder specifically). Flagging this honestly rather
than silently conflating the two.

The new bank targets the **LLM coder itself** (`:18434`, 30B) and answers
exactly the brief: concurrent identical + distinct prompts, cross-response
contamination, dropped/duplicated completions, determinism-where-expected.

### Analyzer design

Three modes, one binary (`cmd/helixqa-verify-coder-race-llm`):

- **`distinct`** — N concurrent requests, each a unique arithmetic question
  (pairwise-distinct expected sum) + a unique `RACEID_<12-char>` nonce the
  model is asked to echo. Assertions: `all_ok`, `no_loss`, `no_duplicate`
  (no two response bodies byte-identical), `own_nonce_ok` (every response
  contains its own nonce), and the core signature `no_cross_contam` — no
  response contains ANY other request's nonce (`detectCrossContamination`).
- **`identical`** — N concurrent identical prompts at `temperature=0`;
  asserts `all_ok`, `no_loss`, `deterministic_ok` (first-integer extraction
  identical across all N responses).
- **`selftest-crosscontam`** — offline, zero-network self-validation of
  `detectCrossContamination()` itself against an in-memory synthetic
  fixture (`clean` / `contaminated`).

### Live results vs the 30B — **7/7 PASS**

| Case | Mode | Result | Key evidence |
|---|---|---|---|
| CODER-RACE-001 | distinct, n=8 | **PASS** | `all_ok=true no_loss=true no_duplicate=true own_nonce_ok=true no_cross_contam=true`; e.g. request 0 → `1001\nRACEID_I5RZWJ6K9VOF` (correct sum, own nonce, no foreign nonce) |
| CODER-RACE-002 | distinct, n=16 | **PASS** | same 5 dimensions true at 2× concurrency — zero cross-contamination pairs across 16×15=240 pairwise checks |
| CODER-RACE-003 | identical, n=8, `17*23` @ temp=0 | **PASS** | `deterministic_ok=true distinct_answers=["391"]` — all 8 concurrent completions returned the byte-identical correct answer |
| CODER-RACE-SELF-VALIDATE-001-GOOD | distinct, n=6, healthy | **PASS** | `pass=true` |
| CODER-RACE-SELF-VALIDATE-001-BAD | distinct, n=6, 1ms timeout, `--expect-fail` | **PASS** (inverted) | `all_ok=false n_timeout=6` → `case_result=true` |
| CODER-RACE-SELF-VALIDATE-002-GOOD | selftest-crosscontam, `clean` fixture | **PASS** | `no_cross_contam=true` |
| CODER-RACE-SELF-VALIDATE-002-BAD | selftest-crosscontam, `contaminated` fixture, `--expect-fail` | **PASS** (inverted) | `no_cross_contam=false cross_contam_pairs=["result[0] contains nonce belonging to request[1] (RACEID_BBB)"]` → `case_result=true` |

Evidence: `evidence/race/*.json`.

### Self-validation is real, not asserted (§11.4.107(10))

Two independent self-validation pairs, both executed:

1. **CODER-RACE-SELF-VALIDATE-001** (live HTTP, established repo
   convention) — proves the generic `all_ok`/`no_loss` wiring is
   load-bearing (healthy PASS vs. impossible-1ms-timeout FAIL-inverted).
2. **CODER-RACE-SELF-VALIDATE-002** (offline synthetic) — proves the
   **novel** `detectCrossContamination()` logic itself is load-bearing.

### Paired §1.1 mutation proof (executed live, never committed)

```
BEFORE experiment sha256: 0f44146a414fe872ea7a80acf074f73e586bd2a6eb27effad79b866c143b2e60

The body of detectCrossContamination() in
cmd/helixqa-verify-coder-race-llm/main.go was temporarily replaced (never
committed — see §11.4.84 note below) with an unconditional early return
that reports "no contamination, ever", short-circuiting the real
pairwise-nonce comparison loop entirely:

    func detectCrossContamination(results []singleResult, nonces []string) (bool, []string) {
        return true, nil // <experiment marker, reverted immediately after>
        var pairs []string
        ...

Rebuilt: go build -o /tmp/helixqa-verify-coder-race-llm-EXPERIMENT ./cmd/helixqa-verify-coder-race-llm/
  -> build OK

Re-ran the golden-bad fixture (contaminated + --expect-fail) against the
experimental binary:
  EXPERIMENTAL BINARY: no_cross_contam=True pass=True case_result=False   (exit 1, FAIL)
  (correct/unaltered behaviour was: no_cross_contam=False pass=False case_result=True, exit 0, PASS)

  ==> case_result FLIPPED true -> false under the neutered detector. The check is load-bearing.

Reverted: cp main.go.orig_backup -> cmd/helixqa-verify-coder-race-llm/main.go
AFTER revert sha256:  0f44146a414fe872ea7a80acf074f73e586bd2a6eb27effad79b866c143b2e60  (BYTE-IDENTICAL to BEFORE)
Post-revert scan of the restored source for any leftover experiment marker
or short-circuit residue: zero matches (§11.4.84 quiescence confirmed).

Rebuilt restored binary: build OK
Re-ran golden-bad fixture on restored binary:
  RESTORED: no_cross_contam=False pass=False case_result=True   (exit 0, PASS) — matches original.
```

Raw JSON artifacts for every step of this proof are under
`evidence/mutation_proof/` (`selftest_clean.json`,
`selftest_contam_raw.json`, `selftest_contam_expectfail.json`,
`MUTATED_selftest_contam_expectfail.json`,
`RESTORED_selftest_contam_expectfail.json`).

**GAP 2 verdict: CLOSED.** A real race/concurrency-correctness bank was
authored, its core detector was proven load-bearing by an executed (and
cleanly reverted) source mutation, and every one of its 7 cases passed
against the confirmed-live 30B coder.

---

## Coder-untouched verification (§11.4.122 / §11.4.108 clean-target)

| Check | Before (start of run) | After (end of run) |
|---|---|---|
| Listening PID on `:18434` | `1980342` (`llama-server`) | `1980342` (`llama-server`) — **unchanged** |
| `/v1/models` served model id | `/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf` | same — **unchanged** |
| Model params | `30,532,122,624` | same |
| HTTP health (`GET /v1/models`) | `200` | `200` (checked after each of the 4 bank runs) |

Full raw captures: `coder_models_before.json`, `coder_models_after.json`,
`coder_pid_before.txt`, `coder_pid_after.txt` (this directory). The coder
was read-only exercised throughout — no restart, no signal, no config
change. Every action taken against it was an HTTP `GET`/`POST` or a
read-only `ps -o rss=` / `ss -tlnp` probe; no `kill`, no service-management
command was issued (§11.4.122 / §11.4.174).

## Scope discipline

Touched only:
- `submodules/helix_qa/banks/helixllm_coder_race.yaml` (new)
- `submodules/helix_qa/cmd/helixqa-verify-coder-race-llm/main.go` (new)
- `submodules/helix_qa/cmd/helixqa-verify-coder-chaos/main.go` (small honest-metrics patch: model field now env-sourced + recorded in the verdict)
- `submodules/helix_qa/qa-results/**` (gitignored raw evidence, not committed)
- `submodules/helix_qa/bin/**` (gitignored build output, not committed)
- root `docs/qa/helixqa_gapfill_live30b_20260711T140607Z/**` (this deliverable)

Not touched: `submodules/helix_agent`, `/mnt/track1`, any file under the
helix_llm A2A stream (`:18441`) or the test-suite stream's working files.
`submodules/helix_qa/tools/opensource/{docling,skyvern}` show as modified
in `git status` but are **pre-existing** nested-submodule pointer drift
unrelated to this task — left untouched and excluded from the commit via
pathspec (§11.4.84).

## Honest boundaries (§11.4.6)

- The `model` field in an OpenAI-compatible chat-completion request is
  decorative for this llama.cpp build (single model resident, field not
  enforced) — confirmed empirically (a request with `model: "llama3.2"`
  was still served correctly by the 30B). The fix corrects the evidence's
  *documentation honesty*, not a functional bug in prior runs' routing.
- `banks/helixcode_coder_race.yaml` (HelixCode HTTP server `:8080`
  ModelManager race bank) remains scaffold-only / never run — genuinely
  out of this task's scope (different target, different service, would
  require booting `bin/helixcode`, not authorized here). Recorded as a
  still-open, separate gap rather than silently folded into this closure.
- CODER-RACE-003's `deterministic_ok` assertion is a real, positive,
  temperature-0 result (`391` × 8) on this run; the bank's own
  documentation notes floating-point non-associativity under concurrent
  batched inference could in principle perturb even greedy decoding on a
  different run — not asserted as an absolute physical guarantee.
