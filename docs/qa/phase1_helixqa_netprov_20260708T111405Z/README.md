# HelixQA Network-Provider LAN Capability — Bank Authoring + Live-Run Evidence

**Run ID:** `phase1_helixqa_netprov_20260708T111405Z`
**Date (UTC):** 2026-07-08T11:14:05Z
**Track:** T1/feature/helixllm-full-extension
**Scope:** §11.4.169 mandatory comprehensive test-type coverage — HelixAgent
NETWORK-PROVIDER capability (HelixAgent can use HelixLLM as a provider over
LAN/VPN, not just localhost, per helix_agent commits `afbeb1bb` / `cfa94f2f`
reading `HELIX_LLM_HOST` / `HELIX_LLM_LOCAL_OPENAI_ENDPOINT`).

## What was built

- `submodules/helix_qa/cmd/helixqa-verify-netprov/main.go` — new
  self-validated analyzer (§11.4.107(10)), text-only OpenAI-compatible chat
  client + a LAN/loopback discriminator (`is_loopback` / `lan_check_pass`).
- `submodules/helix_qa/banks/helixllm_network_provider.yaml` — new HelixQA
  test bank, 7 cases, mirroring the existing `helixllm_vision.yaml` /
  `helixllm_whisper.yaml` pattern.
- helix_qa commit: `1eac74d4f40e51119c1962ec176aebda7ed31c19` (local only,
  not pushed).

## Pre-flight: LAN-not-localhost confirmation (§11.4.6 verify-don't-trust)

The coder (Qwen3-Coder-30B-A3B-Instruct-Q4_K_M, llama.cpp server) was
confirmed live and LAN-reachable on all three host NICs BEFORE any bank
code was written:

```
$ curl -sS -X POST http://10.6.100.221:18434/v1/chat/completions \
    -H 'Content-Type: application/json' \
    -d '{"model":"/models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf","messages":[{"role":"user","content":"reply with only the word OK"}],"max_tokens":5}' \
    -w "\nHTTP_STATUS:%{http_code}\n"

{"choices":[{"finish_reason":"stop","index":0,"message":{"role":"assistant","content":"OK"}}], ...}
HTTP_STATUS:200
```

- eno1 (10.6.100.221) — primary NIC — HTTP 200, real content "OK"
- enp75s0u3 (10.111.28.100) — HTTP 200
- wlp70s0 (10.6.100.58) — HTTP 200
- localhost — also HTTP 200 (used deliberately as the golden-bad loopback
  fixture below, NOT as the LAN proof)
- **Double `/v1` gotcha reproduced:** `POST /v1/v1/chat/completions` on the
  LAN IP returned `404 {"error":{"message":"File Not Found", ...}}` —
  confirms the base-URL-without-`/v1` convention documented in the bank's
  header comment for anyone wiring `HELIX_LLM_LOCAL_OPENAI_ENDPOINT`.

This is the load-bearing "LAN not localhost" proof: a real HTTP 200 with
real assistant content, returned over routable LAN IPs on three distinct
host interfaces, not merely `127.0.0.1`.

## Bank cases and live-run results (all 7, real wire calls, no mocks)

| Case ID | Endpoint | Prompt→Real Answer | raw `pass` | `case_result` | exit |
|---|---|---|---|---|---|
| NETPROV-LAN-EXPLICIT-001 | 10.6.100.221:18434 (eno1) | "17+25?" → **"42"** | true | **true** | 0 |
| NETPROV-LAN-SECOND-NIC-001 | 10.111.28.100:18434 (enp75s0u3) | "capital of France?" → **"Paris"** | true | **true** | 0 |
| NETPROV-LAN-THIRD-NIC-001 | 10.6.100.58:18434 (wlp70s0) | "mix blue+yellow?" → **"Green"** | true | **true** | 0 |
| NETPROV-NOT-LOOPBACK-GOLDEN-BAD-001 | localhost:18434 | "OK" → "OK" (correct!) but `is_loopback=true` | **false** | **true** (via `--expect-fail`) | 0 |
| NETPROV-UNREACHABLE-GOLDEN-BAD-001 | 10.6.100.221:19999 (no listener) | connection refused | **false** | **true** (via `--expect-fail`) | 0 |
| NETPROV-SELF-VALIDATE-001-GOOD | 10.6.100.221:18434 | "17+25?" → "42", expect "42" | true | **true** | 0 |
| NETPROV-SELF-VALIDATE-001-BAD | 10.6.100.221:18434 | "17+25?" → "42", expect "99" (wrong) | **false** | **true** (via `--expect-fail`) | 0 |

Verdict JSONs (real, captured, non-metadata evidence — full HTTP request
outcome including `resolved_host`, `is_loopback`, `lan_check_pass`,
`matched_facts`, `response`, `latency_ms`, `http_status`) are in this
directory:

- `lan_explicit_001_verdict.json`
- `lan_second_nic_001_verdict.json`
- `lan_third_nic_001_verdict.json`
- `not_loopback_golden_bad_001_verdict.json`
- `self_validate_001_golden_good_verdict.json`
- `self_validate_001_golden_bad_verdict.json`
- `unreachable_golden_bad_001_verdict.json`

### RED → GREEN (§11.4.115)

Before authoring the bank, RED-first proofs were captured directly against
the analyzer binary:

1. **RED (unreachable LAN port, no `--expect-fail`):** endpoint
   `10.6.100.221:19999` → `exit=2` (infra error), `error` field non-empty
   ("connection refused"), `pass=false`.
2. **RED (loopback endpoint, no `--expect-fail`):** endpoint
   `localhost:18434` with the SAME live coder → the model correctly
   answers "OK" (`matched_facts=1/1`) but `lan_check_pass=false` forces
   `pass=false` → `exit=1` (case FAIL). This is the discriminating proof
   that a naive "coder answers correctly" check would wrongly PASS on
   loopback alone — this bank's LAN-not-loopback check catches it.
3. **GREEN:** endpoint `10.6.100.221:18434` (real LAN IP) → `pass=true`,
   `exit=0`.

## §11.4.107(10) self-validated analyzer — golden-good / golden-bad + §1.1 mutation

- **Golden-good** (`NETPROV-SELF-VALIDATE-001-GOOD`): real LAN endpoint,
  correct expected fact "42" → raw `pass=true`, `case_result=true`.
- **Golden-bad** (`NETPROV-SELF-VALIDATE-001-BAD`): same LAN endpoint,
  deliberately wrong expected fact "99" via `--expect-fail` → raw
  `pass=false` (`matched_facts=0/1`, response genuinely "42"),
  `case_result=true` (inverted).
- **Paired §1.1 mutation** (load-bearing proof the discriminator is real,
  not decorative): line 275 of `cmd/helixqa-verify-netprov/main.go`
  ```go
  // ORIGINAL:
  v.Pass = v.ExpectedCount > 0 && v.MatchedFacts == v.ExpectedCount && !v.Hallucinated && v.LANCheckPass
  // MUTATED:
  v.Pass = v.LANCheckPass // MUTATED for paired §1.1 mutation test — do not commit
  ```
  Rebuilt, re-ran `NETPROV-SELF-VALIDATE-001-BAD` under the mutation:
  ```
  FAIL: NETPROV-SELF-VALIDATE-001-BAD-MUTATED host=10.6.100.221 loopback=false
    lan_check_pass=true matched=0/1 hallucinated=false expect_fail=true
    raw_pass=true response="42"
  exit=1
  ```
  Raw `pass` bluff-flipped to `true` (the analyzer wrongly accepted the
  mismatched fact once the fact-match conjunct was dropped), `case_result`
  flipped to `false` (case FAIL, exit 1) — **the mutation was caught.**
  The mutation was then reverted (`diff` confirmed byte-identical restore
  against a pre-mutation backup), rebuilt, and `NETPROV-SELF-VALIDATE-001-BAD`
  re-run to confirm restored discrimination: `raw_pass=false,
  case_result=true, exit=0`. No mutation code was committed anywhere in the
  tree (§11.4.84 quiescence) — `git status --short` before commit showed
  only the two new intended paths.

## Honest boundary / SKIP

No honest SKIP was needed — the LAN path was genuinely reachable on all
three host NICs and the loopback/unreachable golden-bad fixtures both
behaved exactly as designed. No fake PASS was required at any point.

## §11.4.174 process-ownership + §11.4.122 coder-untouched note

This bank is strictly read-only inference against the already-running
coder at `:18434` — no boot/teardown, no process kill/signal, no config
change to the coder itself. All 10+ HTTP calls made during authoring and
live-run were plain outbound POSTs from this analyzer/curl; nothing on the
shared host was inspected, killed, or restarted.

## Commits

- `submodules/helix_qa` local commit `1eac74d4f40e51119c1962ec176aebda7ed31c19`
  — `test(banks): HelixQA network-provider LAN capability bank +
  self-validated analyzer (§11.4.169)`. **Not pushed** (per task
  instruction — local commit only).
- Root evidence commit: this directory, committed in the main repo at
  `docs/qa/phase1_helixqa_netprov_20260708T111405Z/`.

## Conductor note (submodule pointer bump)

The conductor should bump the root's tracked helix_qa submodule pointer to
`1eac74d4f40e51119c1962ec176aebda7ed31c19` once S-L's concurrent A2A +
MCP-gateway bank work also lands in the same helix_qa checkout (both
streams share one submodule working tree; the conductor coordinates the
final pointer bump after both local commits are confirmed present on the
shared HEAD).
