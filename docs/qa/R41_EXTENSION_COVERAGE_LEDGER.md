# R41 — HelixLLM Full-Extension Coverage-Completeness Ledger (§11.4.48 / §11.4.6 / §11.4.25 / §11.4.169)

| Property | Value |
|---|---|
| **Revision** | 1 |
| **Created** | 2026-07-11 |
| **Last modified** | 2026-07-11 |
| **Status** | active |
| **Track** | (T1/feature/helixllm-full-extension - claude3) |
| **Scope** | §11.4.48 coverage-completeness audit — is EVERY HelixLLM full-extension work-item fully implemented AND fully covered with tests + captured evidence? |
| **Method** | READ-ONLY. Five parallel evidence-gathering streams (serving/broker, capabilities, protocols, providers, HelixQA) each verified impl (`file:line`/commit SHA) + test (test file + committed evidence path with a real metric quote) on disk; three of them re-ran live tests during the audit. Master plan cross-referenced: `docs/research/07.2026/00_master/MASTER_IMPLEMENTATION_PLAN.md` + the 3 domain plans + `.superpowers/sdd/progress.md` R34→R41 + `docs/qa/R41F_LIVE_VNV_SUMMARY.md`. |

> **Anti-bluff notice (§11.4.6 / §11.4.123).** An item is marked **DONE-PROVEN** ONLY when
> BOTH (a) a real implementation is cited (`file:line` or commit SHA) AND (b) a real test with
> committed captured evidence is cited (test file + `docs/qa/` path + a real metric quote).
> Anything missing either half is downgraded (IMPLEMENTED-UNTESTED / SCAFFOLD / DESIGN-ONLY /
> NOT-STARTED) or, when genuinely blocked on an operator decision/resource, OPERATOR-GATED.
> Plan docs were NOT trusted at face value — every claim was verified against `git`/disk;
> several plan claims were found stale (image-gen, video-gen, coder-boot) and corrected here.

---

## 0. Honest headline

- **38 primary extension work-items enumerated.**
- **25 DONE-PROVEN (65.8%)** — impl + committed captured-evidence test both cited.
- **4 OPERATOR-GATED (10.5%)** — genuinely blocked on an operator decision/resource, not a quality gap.
- **9 GAP (23.7%)** — 4 IMPLEMENTED-UNTESTED · 1 DESIGN-ONLY · 1 SCAFFOLD · 3 NOT-STARTED.

Additionally, several DONE-PROVEN items carry **key-gated coverage sub-gaps** (extended-provider
rows 3/13 live-proven, CONST-039 harness ~4/10 live) — the mechanism is proven; broader coverage
awaits operator API keys (§11.4.10). These are counted DONE for the mechanism, and the coverage
completion is listed under OPERATOR-GATED gaps in §3.

---

## 1. Coverage ledger

Legend — STATE: **DONE-PROVEN** / **IMPL-UNTESTED** / **SCAFFOLD** / **DESIGN-ONLY** /
**OPERATOR-GATED** / **NOT-STARTED**. §-types = §11.4.169 test types with real evidence
(U=unit, I=integration, E=e2e, L=live, B=benchmark, Cc=concurrency, Ch=chaos, D=ddos, M=memory,
R=race, SV=self-validated-analyzer golden-good/bad, RG=RED/GREEN polarity, Chl=Challenge).

### 1a. Serving + VRAM broker

| # | Work-item | Implemented [cite] | Tested [test + evidence, real metric] | §-types | STATE | Gap |
|---|---|---|---|---|---|---|
| 1 | Lane-A coder serving (Qwen3-Coder-30B-A3B, :18434) | live container `helixllm-coder` (`-ngl 99 -c 24576 --parallel 8 --cont-batching -fa on`); broker `ClassCoder` `broker.go:19` | `docs/qa/coder_boot_liveproof_20260711T125759Z/RESULTS.md` "PASS: nonce echoed"; `docs/qa/helixqa_live_vnv_20260711T130200Z/` bench 7/7 (p50 152→1076ms, 269–624 tok/s) | L,B,Cc | **DONE-PROVEN** | — |
| 2 | Lane-B 2nd instance (ClassAgent + Mistral-Nemo-12B, `cmd/agentgen-boot`) | `broker.go:30` `ClassAgent`; `cmd/agentgen-boot/main.go` (`c92fb16`) | `docs/qa/lane_b_liveproof_20260711T130533Z/RESULTS.md` — 163.67 tok/s single, 3/3 parallel, coder PID 1980342 identical pre/post (untouched) | L,B,Cc | **DONE-PROVEN** | — |
| 3 | VRAM broker CORE (7 classes, fail-closed admit) | `broker.go:18-36`, `admit()` `:189` | `broker_test.go`/`class_agent_test.go`/`integration_test.go`/`mutation_test.go`; ledger: admit→true mutation flips 6 tests FAIL, 25 over-budget refusals no-OOM, review GO | U,I,R,SV(mut) | **DONE-PROVEN** | — |
| 4 | Pre-boot config validator (`cmd/laneconfig-validate`) | `validate.go` KV formula `:154`, `-ngl` `:262`, port/name collision `:269-281` | `validate_test.go` (KV byte-math vs §4); `laneconfig_validate_challenge.sh` real build+exec, 4 fixtures assert exact exit codes | U,Chl | **DONE-PROVEN** | — |
| 5 | D6 tool-calling-under-load guard (§11.4.135, array-as-string #1809 class) | parse/sanitize `internal/gateway/tool_call_normalizer.go`, `internal/brain/openai_provider.go:213-262` | ONLY unit `tool_call_normalizer_test.go:94 TestParseQwen3ToolCall_ArgumentsAsString` + 1 manual single-request "2+2→4". No N-concurrent array-typed bank, no §11.4.135 RED/GREEN guard | U | **IMPL-UNTESTED** | Task 3.2 deliverable (concurrent load + permanent guard) absent |
| 6 | GGUF-revision verification (Task 1.1) | none — plan itself says fix-commit SHA "UNCONFIRMED" | none (grep "Unsloth/GGUF revision/Task 1.1" → 0 evidence) | — | **NOT-STARTED** | dropped from R30–R41 stream, no tracker entry |
| 7 | Phase-4 Lane-A kv-unified/batch-size/parallel tuning | pause mechanism proven (`docs/qa/phase1_coder_pause_20260708T141500Z/`), flag changes NOT applied; Task 1.2/1.3 memos never produced | pause-cycle proven; the flag changes themselves untested | — | **OPERATOR-GATED** | Task 4.1 = operator coder-pause window (§11.4.122); Task 1.2/1.3 non-gated memos NOT-STARTED |

### 1b. Capabilities

| # | Work-item | Implemented [cite] | Tested [test + evidence, real metric] | §-types | STATE | Gap |
|---|---|---|---|---|---|---|
| 8 | Vision-VLM (Qwen2.5-VL-3B, :18439) | `cmd/visiongen-boot/main.go`; Phase-3 `e857d59` | `docs/qa/vision_liveproof_20260711T133823Z/` (co-resident ADMIT-OK, footprint 4582 MiB); bank `helixllm_vision.yaml` @ `docs/qa/phase1_helixqa_vision_20260708T061809Z/` 6 PASS/1 SKIP, `VIS-SELF-VALIDATE-001-BAD PASS` | L,SV,I,helixqa | **DONE-PROVEN** | — |
| 9 | Image-gen (FLUX.1-schnell, :18442, ClassImage) | scaffold `0f07559`; ungated proof `d8593b63`, re-val `796aee91` | `docs/qa/generative_liveproof_20260711T131205Z/RESULTS.md` — imganalyzer golden-good entropy=6.21 colors=43692 vs golden-bad entropy=0 colors=1; artifact entropy 7.82 / 147440 colors; `ADMIT-OK` broker lease | L,SV,I | **DONE-PROVEN** | flagship fp8 tier + scaffold's own FLUX.1-dev+Nunchaku path OPERATOR-GATED (HF_TOKEN/NUNCHAKU_WHEEL / coder-pause) |
| 10 | Video-gen (WAN2.2 TI2V-5B / LTX, :18443, ClassVideo) | scaffold `9145505` (`cmd/videogen-boot`, README self-declares "NOT yet a captured runtime proof"); raw-script clip `f2c71d07` | real `.mp4` (h264 1280×704 24fps 121 frames, ffprobe-verified this audit) BUT generated off-broker by raw script, never run through `vidanalyzer`, generation.log shows uninvestigated VAE-decode OOM warning | L(raw,unanalyzed) | **IMPL-UNTESTED** | not run via `cmd/videogen-boot`/ClassVideo broker; no analyzer run on real clip; flagship OPERATOR-GATED (~33.6 GB need vs 12.6 free) |
| 11 | Vectorization (pixel→SVG, vtracer/StarVector) | **none** — grep vtracer/StarVector/ClassVector → 0 source hits | none | — | **NOT-STARTED** | plan's own table: "NET-NEW … not yet a P3-Tx spike doc" |
| 12 | Translation (NLLB-200, :18436) | `e58aab9f`; harness `docs/qa/phase3_translation_nllb_20260707/harness/` | `phase3_translation_nllb_20260707/RESULTS.md` — "Das Haus ist blau.", RED echo-stub FAIL, self-val PASS; bank `helixllm_translate_nllb.yaml` @ `phase1_helixqa_translate_...` 4/4 | L,RG,SV,helixqa | **DONE-PROVEN** | — |
| 13 | Whisper STT (:18437) | `e1b3b41`; `container/whisper_stt_server.py` | `docs/qa/phase3_whisper_stt_20260707/RESULTS.md` — transcript "The quick brown fox…", determinism PASS, self-val PASS; bank `helixllm_whisper.yaml` 4/4 | L,SV,helixqa | **DONE-PROVEN** | — |
| 14 | Tesseract OCR (:18438) | `f8632e3` | `docs/qa/phase3_tesseract_ocr_20260707/RESULTS.md` — mean_conf=95.99 "HELIX OCR 2026…", self-val PASS; bank `helixllm_tesseract.yaml` 4/4 | L,SV,helixqa | **DONE-PROVEN** | — |
| 15 | Embeddings (bge-small via TEI) | `55bdf9b6` | `docs/qa/phase3_embeddings_20260706/RESULTS.md` — cos(related)=0.7509 » cos(unrelated)=0.3931 margin=0.3578, 3 golden-BAD FAIL; bank `helixllm_embeddings.yaml` 3/3 | L,RG,SV,helixqa | **DONE-PROVEN** | — |
| 16 | RAG-TEI core (:18440) | harness `docs/qa/phase3_rag_20260707/harness/` | `docs/qa/rag_liveproof_20260711T133918Z/RESULTS.md` — rank1 score=0.8852, invented token "Borealis-9" only under retrieval, RED baseline load-bearing; bank `helixllm_rag.yaml` 4/4 | L,RG,SV,helixqa | **DONE-PROVEN** | — |
| 17 | RAG Qdrant vector-DB + bge-reranker-v2-m3 fusion | **none** — no Qdrant/reranker code or harness anywhere | none | — | **DESIGN-ONLY** | `04_embeddings_rag.md` recommends it; unimplemented |
| 18 | HelixMemory (durable memory) | `helix_code/internal/memory/helixmemory_provider.go` (177L, wraps `digital.vasic.helixmemory/pkg/localstore`) | `helixmemory_provider_test.go` (real temp-SQLite, no mocks); `docs/qa/phase1_helixmemory_20260708T061824Z/RESULTS.md` MEMORY-RUNTIME-SIGNATURE PASS top1 score=0.8536 tokenFound=true | U,L,RG | **DONE-PROVEN** | reference mechanism (TEI+pgvector+coder); literal mem0/graphiti-core packages NOT the impl |
| 19 | cognee wiring | pre-existing broken integration in `submodules/helix_agent`, disabled `cognee_integration:false` | N/A (feature disabled) | — | **OPERATOR-GATED** | §11.4.174 foreign-dirty `helix_agent` + upstream bug (`COGNEE_BUG.md` AttributeError, no public fix) |
| 20 | Network-provider (LAN/VPN) | landed per RESUME; bank `helixllm_network_provider.yaml` (7 cases) | `docs/qa/phase1_helixqa_netprov_20260708T111405Z/` — `10.111.28.100:18434 "capital of France?"→"Paris"` real LAN wire | L,I,helixqa | **DONE-PROVEN** | — |

### 1c. Protocols / core providers

| # | Work-item | Implemented [cite] | Tested [test + evidence, real metric] | §-types | STATE | Gap |
|---|---|---|---|---|---|---|
| 21 | Dual-wire facade (OpenAI `/v1/chat/completions` + Anthropic `/v1/messages`) | `helix_code/internal/server/server.go` route grp + `wireFacadeAuthMiddleware()` `:594`; `8a309004` | `wire_facade_test.go`/`wire_facade_auth_test.go`/`wire_facade_live_e2e_test.go`; `docs/qa/dualwire_live_e2e_20260711T130459Z/` — 401 no-auth + both-shape nonce-echo, OpenAI args JSON-string vs Anthropic input JSON-object | U,I,E,L | **DONE-PROVEN** | — |
| 22 | MCP gateway (real go-sdk v1.6.1) | `submodules/helix_llm/cmd/mcp-gateway` + `internal/mcpgateway/*` (`895788c5`) | `server_test.go`; `docs/qa/mcp_gateway_live_e2e_20260711T132250Z/` — nonce `R41F-NONCE-f23370ad…` echoed by live Qwen via `tools/call`, no-bearer→401 | U,E,L | **DONE-PROVEN** | — |
| 23 | A2A real wire (Google A2A) | `submodules/helix_llm/internal/a2a/*` + `cmd/a2a-server/main.go` | `internal/a2a/a2a_test.go` (5 funcs); `docs/qa/phase3_a2a_20260707/RESULTS.md` "RED→GREEN … against the LIVE coder, no bluff" | U,RG,E,L | **DONE-PROVEN** | — |
| 24 | Legacy proprietary ACP stub retirement | stub STILL PRESENT unchanged `submodules/helix_agent/cmd/api/main.go` `/api/v1/acp/{execute,broadcast,status}` (canned responses) | N/A (known stub) | — | **OPERATOR-GATED** | §11.4.122 no-silent-removal; `ACP_A2A_PROVIDER.md` §Q6 — operator must choose replace/redirect/keep |
| 25 | codegraph wiring fix | path-rot `7e8e4946` (`.mcp.json` portable bare cmd); exclude-bloat `bc89f2ac` (51,961→22,548 files); live DB confirmed cli_agents/%=0 | `scripts/codegraph_validate.sh`; `docs/qa/phase4_codegraph_20260707/21_own_org_symbol_proof_mcp.txt` resolves `admit` @ `broker.go:178` | I,unforgeable-probe | **DONE-PROVEN** | §11.4.83 gap: only committed validate run (26P/3F) PREDATES `bc89f2ac`; no post-fix docs/qa re-run committed |
| 26 | opendesign bring-up (:7456, helixcode-brand seed) | daemon built off-tree `.opendesign-src`; `.mcp.json` enable `c1fe11b0` | ONLY `scratchpad/opendesign_evidence/…log` (GIT-IGNORED `*.log`) — daemon health {ok,0.14.1}, project list {helixcode-brand}. No `docs/qa/` evidence | manual-run (non-durable) | **IMPL-UNTESTED** | §11.4.83 violation: evidence git-ignored, not under docs/qa; `:7456` NOT reachable this session; BYOK/`od_generate_design` never exercised |
| 27 | LLMsVerifier C1→C5 resolver chain | `submodules/llms_verifier` `309af635`/`2c507020`/`09f9533c`/`28e6625a`/`ad18e91f`; `registry_resolve.go` (167L) | `registry_resolve_test.go`+`registry_c3_failclosed_red_test.go`; re-run live this audit: `--- PASS TestC3Resolver_…` (fail-closed) | U,RG | **DONE-PROVEN** | C1's cited `docs/qa/p2_llmsverifier_toolcalls/` not on disk today (transient scratch); C1 review-follow-up open |
| 28 | Extended-provider config rows (13) | `providers/config.go` `c696c5db` (poe/perplexity/sakana/hunyuan/xai/moonshot/zai/fireworks/deepinfra/ai21/reka +hyperbolic/xiaomi) | `extended_providers_test.go`; `submodules/llms_verifier/docs/qa/phase4_extended_providers_20260707/` — poe 341 / zai 8 / novita 143 models LIVE; re-run this audit reproduced | L,U(RG) | **DONE-PROVEN** | 3/13 live; 10/13 honest key-absent SKIP → coverage OPERATOR-GATED (§11.4.10) |
| 29 | CONST-039 per-provider live-proof harness | `helix_code/internal/llm/provider_live_proof_test.go` (424L, tag `providerlive`) + `_skip_test.go` | `docs/qa/provider_live_proof_20260711T104843Z/` + `_harnessfix_20260711T104938Z/` — mistral/deepseek/openrouter/groq nonce PASS, gemini FAIL(bad key), openai/anthropic/xai SKIP | L,U | **DONE-PROVEN** (harness) | provider coverage ~4/10 LIVE, key-gated; §11.4.50 noise on reasoning models (documented) |
| 30 | Catalogue-providers (HF-router + together) | `helix_code/internal/llm/openai_compatible_catalogue.go` `7210f373` | `openai_compatible_catalogue_test.go` (+§1.1 mutation); `docs/qa/r41_catalogue_hf_together_20260711T112230Z/` — HF 200/120 models, together honest SKIP+401 reachability | U,L | **DONE-PROVEN** | together row has no live-authed proof (reachability-only, no key) |
| 31 | HelixLLM-as-tracked-provider registration (Phase A) | `submodules/llms_verifier` `760bfde9` (`helixllm` row + seed) | `helixllm_test.go`/`helixllm_polarity_test.go`/`helixllm_seed_test.go`; re-run live this audit vs real coder: `TestHelixLLM_LiveVerification_RealProbe (36.4s)` ToolUse/FunctionCalling/CodeGen true | L,U,I | **DONE-PROVEN** | plan's named `docs/qa/helixllm_verifier_registration/` dir never created (evidence embedded in tests) |
| 32 | claude_toolkit alias reconciliation | `claude_toolkit` `5d611d9` (+sakana/fireworks-ai/deepinfra/moonshotai) | `scripts/tests/test_providers.sh`; re-run this audit 221/224 (3 fails unrelated `cma_run` regression) | U(hermetic),L | **DONE-PROVEN** | hunyuan (blocked — models.dev absent, tencent-tokenhub distinct) + ai21/reka genuinely absent (not addable via alias) |
| 33 | claude_toolkit R5 port-ownership hardening | `claude_toolkit` `ef77b19` (`detect_helixagent_record()` asserts helix-debate/helix-llm), merged to main | `verify_helixagent_test.sh` + `proof/82-helixagent-detect.txt`; re-run this audit 37/0 (neg-case fake responder rejected) | U(hermetic),neg | **DONE-PROVEN** | independent re-review (§11.4.134/§11.4.142) loop noted PENDING in commit |
| 34 | Dead-code disposition (cerebras/hf/together/replicate) | investigation `scratchpad/r41_provider_currency_audit.md` (UNTRACKED); confirmed 0 prod refs | N/A (research) | — | **OPERATOR-GATED** | §11.4.122+§11.4.124 — no disposition decided; audit doc not committed (non-durable) |

### 1d. HelixQA banks / Challenges (extension-programme test infrastructure)

| # | Work-item | Implemented [cite] | Tested [test + evidence, real metric] | §-types | STATE | Gap |
|---|---|---|---|---|---|---|
| 35 | HelixQA coder bench + concurrency banks | `helixllm_coder_bench.yaml`(7)/`helixllm_coder_concurrency.yaml`(6) | `docs/qa/helixqa_live_vnv_20260711T130200Z/RESULTS.md` — 13/13 PASS vs REAL 30B coder, fresh in-session §1.1 mutation flips golden-bad, model_path verified pre/post | L,B,Cc,SV(mut) | **DONE-PROVEN** | — |
| 36 | HelixQA coder DDoS + chaos + memory banks | `helixllm_coder_ddos.yaml`/`_chaos.yaml`/`_memory.yaml` | `docs/qa/20260708T212222Z/` + `536ac9c6` 11/11 PASS — BUT commit `dfa6a2c2` states run was against **qwen2.5:0.5b substitute**, NOT production Qwen3-Coder-30B | D,Ch,M (wrong topology) | **IMPL-UNTESTED** | validated against substitute model, not the production 30B coder — §11.4.108 clean-target gap |
| 37 | HelixQA coder race/deadlock bank | `helixcode_coder_race.yaml` + `cmd/helixqa-verify-coder-race/` (`cb3ce95f`) | `docs/qa/phase1_helixqa_coder_race/` = **only `.gitkeep`** (empty) — never executed | — | **SCAFFOLD** | §11.4.169 #12 (race/deadlock) has ZERO real evidence |
| 38 | Challenges-submodule extension coverage | `submodules/challenges` wired in `.gitmodules` | NO Challenge script for any of the 9 extension capabilities (only generic CLI-agent-fusion p1-f* items) | — | **NOT-STARTED** | §11.4.50(B) Challenges coverage for extension capabilities absent |

---

## 2. §11.4.169 mandatory-test-type coverage (extension programme, ≥1 capability with real evidence)

| # | Test type | Evidence? | Citation |
|---|---|---|---|
| 1 | Unit | PARTIAL | broker/validator/normalizer/catalogue/verifier `_test.go`; but NO `_test.go` for ~9 `cmd/helixqa-verify-*` analyzers |
| 2 | Integration | YES | all phase1 bank runs hit live services (TEI/NLLB/Whisper/Tesseract) |
| 3 | E2E | YES | `mcp_gateway_live_e2e_…` real go-sdk client→gateway→live coder |
| 4 | Full-automation | YES | all `phase1_helixqa_*` self-driving analyzer binaries, `--expect-fail` self-val |
| 5 | Challenges (vasic-digital) | **NOT-FOUND** | no extension-capability Challenge scripts (item 38) |
| 6 | helix_qa | YES | entire §1b/§1d bank corpus |
| 7 | DDoS | YES (caveat) | `docs/qa/20260708T212222Z/` 100 req/20s no-5xx — substitute model |
| 8 | Security | PARTIAL | bearer-auth 401 enforcement (MCP + dual-wire e2e); no dedicated extension security bank |
| 9 | Stress | PARTIAL | sustained load folded into bench levels 50/100 (30 req each) |
| 10 | Chaos | YES (caveat) | `docs/qa/20260708T212222Z/` concurrent-health chaos — substitute model |
| 11 | Concurrency/atomicity | YES | `helixqa_live_vnv_…` CODER-CONC 20/20, 50/50 zero-drop byte-identical |
| 12 | Race/deadlock | **NOT-FOUND** | `helixcode_coder_race.yaml` authored but evidence dir empty (item 37) |
| 13 | Memory | YES | `docs/qa/20260708T212222Z/` RSS +0.4% monotonic-no-leak |
| 14 | Benchmarking | YES | `helixqa_live_vnv_…` full p50/p95/p99/RPS/tok/s @ 1–100 concurrency, TTFT p50=3ms |

**8 clean YES · 4 PARTIAL · 2 NOT-FOUND (Challenges, Race).**

---

## 3. Prioritized GAP LIST

### (a) Implemented-but-undertested → needs tests (own quality gaps — actionable now)

1. **Video-gen runtime proof through the product path** (item 10) — real `.mp4` exists but was
   generated off-broker by a raw script and never run through `vidanalyzer` / `ClassVideo`
   admission; the uninvestigated VAE-decode OOM warning is unresolved. Needs: run
   `cmd/videogen-boot` → broker admit → `vidanalyzer` (ffprobe geometry + freeze/frame-advance +
   per-frame CLIPScore + golden-bad) → commit `docs/qa/<run>/`. *(Flagship tier separately
   operator-gated — see (c).)*
2. **HelixQA coder DDoS/chaos/memory re-run against the PRODUCTION 30B coder** (item 36) — current
   PASS is against a `qwen2.5:0.5b` substitute (§11.4.108 clean-target gap). Re-run against the
   real `helixllm-coder`, commit evidence.
3. **D6 tool-calling-under-load §11.4.135 regression guard** (item 5) — only single-request unit
   coverage exists; author the N-concurrent array-typed-args RED/GREEN permanent guard (Task 3.2).
4. **opendesign durable evidence** (item 26) — capability was exercised once but the only proof is
   a git-ignored `scratchpad/*.log`; §11.4.83 requires committed `docs/qa/` evidence. Re-run with
   `:7456` live + `od_list_projects` + a codegraph dual-challenge, commit it.
5. **codegraph post-fix `codegraph_validate.sh` committed re-run** (item 25) — the fix is real
   (git + live DB) but the only committed validate run predates `bc89f2ac`; commit a clean post-fix run.
6. **HelixQA coder race/deadlock bank execution** (item 37) — bank + analyzer authored, evidence
   dir empty; run it against the real coder, close §11.4.169 #12.

### (b) Scaffold / design-only / not-started → needs implementation

7. **RAG Qdrant vector-DB + bge-reranker-v2-m3 fusion** (item 17) — DESIGN-ONLY; the TEI-embed +
   cosine-retrieve + ground core is proven, but the durable vector-DB + reranker layers are unwritten.
8. **Vectorization (pixel→SVG, vtracer default + StarVector smart tier)** (item 11) — NOT-STARTED;
   zero source code; author `VECTORIZE_PROVIDER.md` spike then implement + prove (re-rasterize
   perceptual-similarity signature).
9. **GGUF-revision verification (Task 1.1)** (item 6) — NOT-STARTED; confirm running GGUF postdates
   Unsloth's tool-calling fix commit (direct HF-API check).
10. **Challenges-submodule extension coverage** (item 38) — NOT-STARTED; no Challenge script for any
    extension capability (§11.4.50(B)).
11. **Task 1.2 (`--kv-unified` PoC memo) + Task 1.3 (batch-size sizing memo)** — NOT-STARTED, and
    the plan explicitly marks these as NON-operator-gated (throwaway-model PoC + arithmetic memo).

### (c) Operator-gated → name the gate

- **Phase-4 Lane-A kv-unified/batch/parallel apply (Task 4.1)** — operator coder-pause window (§11.4.122).
- **Image-gen flagship fp8 tier + gated FLUX.1-dev/Nunchaku scaffold path** — HF_TOKEN + NUNCHAKU_WHEEL
  (§11.4.10) and/or coder-pause burst window (§11.4.122).
- **Video-gen flagship (WAN 2.2 A14B fp8, ~14–20 GB)** — operator coder-pause burst window (§11.4.122).
- **cognee re-enable** (item 19) — §11.4.174 foreign-dirty `helix_agent` worktree + upstream bug (no public fix).
- **Legacy ACP stub retirement** (item 24) — §11.4.122 no-silent-removal; operator chooses replace/redirect/keep.
- **Dead-code disposition (cerebras/hf/together/replicate)** (item 34) — §11.4.122/§11.4.124; decision + commit the audit doc.
- **Extended-provider full live coverage (10/13 remaining)** (item 28) + **CONST-039 full coverage
  (~6/10 remaining)** (item 29) — operator API keys (§11.4.10).
- **claude_toolkit ai21/reka aliases + hunyuan reconciliation** (item 32) — genuinely absent from
  models.dev catalog; needs endpoint decision/keys.
- **Cross-cutting security**: leaked `GEMINI_API_KEY` in pushed history (`f994c0c2`, redacted at HEAD
  `41372967`) — operator MUST rotate (force-push forbidden §11.4.113); this is why item 29's gemini
  live-proof FAILs.

---

## 4. Honest boundary (§11.4.6)

This ledger is a **read-only audit**: it re-verified impl+evidence on disk (three streams re-ran
live tests during the audit), but it did NOT itself re-run every cited bank, nor re-flash any
artifact. The **65.8% DONE-PROVEN** figure counts an item DONE only when BOTH an implementation
AND a committed captured-evidence test are cited — it does NOT claim those 25 items are bug-free,
nor that they have passed the §11.4.185 manual-QA-team final confirmation (which remains open per
the SDD ledger). The **23.7% GAP** and **10.5% OPERATOR-GATED** figures are the honest remainder;
several DONE items additionally carry the key-gated / evidence-durability sub-gaps enumerated in §3.
Plan docs were treated as leads, not truth — image-gen/video-gen/coder-boot plan claims were found
stale and corrected against `git`/disk here.

## 5. Sources verified

- Evidence streams (this session, 5× read-only, 3× with live test re-runs): serving/broker,
  capabilities, protocols, providers, HelixQA.
- `docs/research/07.2026/00_master/MASTER_IMPLEMENTATION_PLAN.md` + `RESUME.md`;
  `01_local_models_serving/IMPLEMENTATION_PLAN_v2.md`;
  `02_vision_generative/CAPABILITIES_MASTER_PLAN_v2.md`;
  `06_providers_coverage/EXPANSION_PLAN_v2.md`.
- `.superpowers/sdd/progress.md` (R34→R41 ledger) · `docs/qa/R41F_LIVE_VNV_SUMMARY.md`.
- Direct reads this session: `submodules/helix_llm/internal/vrambroker/broker.go`,
  `submodules/helix_llm/cmd/` (a2a-server/agentgen-boot/laneconfig-validate/mcp-gateway/…),
  `submodules/helix_qa/banks/helixllm_*.yaml`, `.mcp.json`, `docs/qa/*_liveproof_*`,
  `docs/qa/phase1_helixqa_*`, `docs/qa/phase3_*`, `docs/qa/phase4_*`,
  `docs/qa/phase1_helixqa_coder_race/` (`.gitkeep` only).
