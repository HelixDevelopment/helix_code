# HelixLLM Full-Extension Programme — MASTER IMPLEMENTATION PLAN (consolidated)

| | |
|---|---|
| **Document** | Single authoritative roadmap consolidating three independently-reviewed domain plans |
| **Path** | `docs/research/07.2026/00_master/MASTER_IMPLEMENTATION_PLAN.md` |
| **Created** | 2026-07-08 |
| **Track/branch** | `(T1/feature/helixllm-full-extension)` |
| **Status** | ROADMAP — references + sequencing + decisions. Does NOT re-derive detail already written in the three source plans; read the cited plan for full evidence/task detail before implementing. |
| **Consolidates** | `01_local_models_serving/IMPLEMENTATION_PLAN_v2.md` (serving, GO), `02_vision_generative/CAPABILITIES_MASTER_PLAN_v2.md` (capabilities, GO), `06_providers_coverage/EXPANSION_PLAN_v2.md` (providers, PROVISIONAL — re-review in flight) |

> **Anti-bluff notice (§11.4/§11.4.123).** This document is a roadmap, not a new research
> pass. Every fact below is cited to one of the three source plans or to `RESUME.md`/
> `PROVIDER_COVERAGE.md`/`03_open_clarifications.md`/`99_risk_bottleneck_analysis.md`. Nothing
> here re-plans already-landed work (§11.4.74) — see §1.

---

## 1. Baseline — what's already LANDED + PROVEN (do NOT re-plan this)

Per `RESUME.md`, the branch `feature/helixllm-full-extension` is **RELEASE-READY / RELEASE
PUBLISHED**: tag `helix-code-1.0.0-dev-0.0.1` created + pushed across main-repo HEAD `10c40c85`
and all 7 owned submodule HEADs (helix_llm `071c1223`, doc_processor `b918111`, llm_provider
`4db6c49`, llm_orchestrator `ee229a7`, llms_verifier `c696c5db`, vision_engine `a97df79`,
helix_agent `cfa94f2f`). Whole-branch SDD end-gate returned **GO across all 3 independent
lenses** (anti-bluff, security/§11.4.174, integration/release-readiness). §11.4.40 pre-tag sweep
GREEN on owned scope. **§11.4.138 honesty correction on file** (2026-07-08): the tag's commit
message/annotation over-claimed "broad provider coverage" undifferentiated from live-proven
capabilities — the substantive docs were audited HONEST; see §5 below for the follow-up this
correction opened (now itself in flight, see §6.0).

### 1.1 Reviewed-GO capabilities (proven, cite — never re-plan)

| Capability | State | Evidence |
|---|---|---|
| GPU foundation (Phase 0) | Rootless CDI passthrough + sm_120 build + real 30B inference PROVEN | `RESUME.md` Phase 0 |
| Coder fleet (Phase 1) | Qwen3-Coder-30B-A3B live at `:18434/v1`, 8 slots, 24k ctx, q8_0 KV, ~19.4 GiB VRAM; ~220 tok/s single-stream, 8×85-96 tok/s concurrent | `docs/qa/phase2_e2e_20260706/` |
| HelixAgent→HelixLLM e2e (Phase 2) | Real generate + Postgres/Redis persistence PROVEN + GO | `RESUME.md` Phase 2 |
| LLMsVerifier C1→C5 chain | Landed (`309af635`…`c696c5db`), fail-closed resolver, combined review GO — **do not re-implement in any form** | `01_local_models_serving/IMPLEMENTATION_PLAN_v2.md` §0; `06_providers_coverage/EXPANSION_PLAN_v2.md` §0.1 |
| Extended-provider config rows | 13 rows landed in `config.go` (`c696c5db`); 3 LIVE-PROVEN (poe 341 models, zai 8 models, novita 143 models), 1 partial (fireworks billing-blocked), 9 honest credential-absent SKIPs | `06_providers_coverage/EXPANSION_PLAN_v2.md` §0.2 |
| CONST-039 per-provider live-proof harness | **Landed this session (uncommitted, ready to commit)** — `helix_code/internal/llm/provider_live_proof_test.go` + `_skip_test.go`, nonce-challenged unforgeable PASS; 2 LIVE (Groq, Mistral), 3 real non-fabricated FAILs (DeepSeek/Gemini/OpenRouter), 5 honest SKIPs | `docs/qa/provider_live_proof_RESULTS_20260707.md` — this directly closes the §11.4.138 tracked follow-up |
| Embeddings | IMPLEMENTED, PROVEN — bge-small via TEI, cos margin 0.3578 | `55bdf9b6` |
| Vision-VLM (co-resident) | PROVEN when running — Qwen2.5-VL-3B-Instruct-Q4_K_M at `:18439`, `n_ctx:16384` — **container currently STOPPED** (torn down mid-session, restart on demand, no VRAM held) | `02_vision_generative/CAPABILITIES_MASTER_PLAN_v2.md` §0.1 |
| Translation (NLLB) | Landed at `:18436` | `RESUME.md` Phase 3 |
| Whisper STT | IMPLEMENTED, reviewed GO, `:18437` | `07_stt_ocr_whisper_tesseract.md` §1 |
| Tesseract OCR | IMPLEMENTED, reviewed GO, `:18438` | `07_stt_ocr_whisper_tesseract.md` §2 |
| RAG-TEI | Landed, `:18440` | `RESUME.md` Phase 3 |
| ACP→A2A | Landed (resolved to Google A2A, not Zed ACP), `:18441` | `ACP_A2A_PROVIDER.md`; `03_open_clarifications.md` C3 |
| Network-provider (LAN/VPN) | Landed | `RESUME.md` Phase 3 |
| VRAM broker CORE | `a12df57c`, reviewed GO — `ClassCoder`(resident)/`ClassVLM`/`ClassImage`/`ClassVideo`(burst, single-owner §11.4.119)/`ClassTranslate`(warm)/`ClassEmbed`(CPU 0-byte); fail-closed `admit()`; **no eviction/pause-warm-tier logic yet** (honest gap) | `submodules/helix_llm/internal/vrambroker/broker.go` |
| DZ-05 (HelixLLM API contract) | **RESOLVED** — `submodules/helix_llm/docs/API_CONTRACT.md` now exists, confirmed on disk | `02_vision_generative/CAPABILITIES_MASTER_PLAN_v2.md` §0.1/§5.1 |
| codegraph | DESIGN COMPLETE + defects PROBED (path rot, version drift, index bloat) — wiring fix not yet landed (see roadmap Phase 5) | `08_codegraph_opendesign.md` §1.1/§2/§4 |
| Operator clarifications (GPT-Sol, OKF, ACP, SubQ) | **Already answered**, do not re-derive | `03_open_clarifications.md` C1–C4 |

### 1.2 Image/video generative — scaffold-complete, runtime-proof pending

Image-gen (`0f07559`, port 18442) and video-gen (`9145505`, port 18443) are
**SCAFFOLD-COMPLETE, broker-integrated, self-validated CLIPScore analyzers, RED-first** —
reviewed GO at the scaffold layer. **Runtime proof is the only remaining gap**, and per the
re-baselined VRAM reading (§3 below) the fallback/fast-lane tiers **no longer require an
operator coder-pause** (vision is already stopped) — only the flagship tiers still need a
scheduled burst window. Do not re-scaffold; only the runtime-proof step remains (roadmap
Phase 3, §4).

---

## 2. Unified phased roadmap

Ordered by cross-plan dependency. Each phase cites source-plan task IDs — see the cited plan
for full detail, acceptance criteria, and evidence paths. "Gated" = requires an operator
decision (§5) or a live resource not currently free; "Non-gated" = startable now, subagent-
dispatchable, parallel-safe (§11.4.103/§11.4.70).

| # | Phase | Source task IDs | Gated? | Depends on |
|---|---|---|---|---|
| 0 | Baseline capture (config/throughput snapshot, no coder touch) | Serving-plan Phase 0 (Tasks 0.1–0.2) | No | — |
| 1a | Serving non-coder-touching work: GGUF-revision check, kv-unified PoC memo, batch-size sizing memo, broker `ClassAgent`, pre-boot config validator | Serving-plan Phase 1 (1.1–1.5) | No | Phase 0 |
| 1b | Capabilities non-blocked work: codegraph wiring fix (path-rot + exclude-bloat + version reconcile), opendesign bring-up, HelixQA bank authoring (Bank B first — vision) | Capabilities-plan P5-T1′/P5-T2′, §5 | No | DZ-05 resolved (done) |
| 1c | Providers Phase A–C: HelixLLM-as-tracked-provider registration in LLMsVerifier, claude_toolkit alias reconciliation (hunyuan/moonshot ambiguity resolution + sakana/fireworks/deepinfra/ai21/reka add), R5 port-ownership hardening | Providers-plan §3 Phase A/B/C | No | LLMsVerifier chain (landed) |
| 2 | Serving Lane-B build-out: model benchmarking spike (Mistral-Nemo-12B priority 1, GLM-4.7-Flash, DeepSeek-Coder-V2-Lite), production container + broker wiring | Serving-plan Phase 2 (2.1–2.2) | No (fits current free VRAM) | Phase 1a Task 1.4/1.5 |
| 2b | Capabilities Phase 3 non-GPU-burst items: vectorization spike doc + vtracer default, translation P3-T5′ spike + NLLB CPU-tier route, embeddings/RAG completion (Qdrant + reranker), HelixMemory (Zep/Graphiti+mem0) spike doc re-authored | Capabilities-plan P3-T4′/T5′/T6′/T7′ | No | — |
| 2c | Providers Phase D: dual OpenAI+Anthropic wire facade for HelixCode's own server (the one net-new adapter) | Providers-plan §3 Phase D | No | — |
| 2d | Providers Phase E/F: setup/PATH-install gap-fill, provider-verification test extension | Providers-plan §3 Phase E/F | No | Phase 1c |
| 3 | Runtime-proof for image/video generative + serving observability | Capabilities-plan P3-T2′/T3′; Serving-plan Phase 3 (3.1–3.3) | **Partially gated** — fallback/fast-lane tiers run NOW (no pause needed); flagship tiers need operator coder-pause | Phase 1a broker validator; VRAM re-measured live at proof time |
| 4 | Vision hardening (8B warm tier) + STT/OCR wiring-everywhere sweep | Capabilities-plan P3-T1′/T8′ | No | — |
| 5 | MCP gateway + A2A real-wire landing (retire/replace proprietary stub) | Capabilities-plan P4-T5′/T4′ | **A2A partially gated** — stub retirement needs operator decision (§11.4.122 no-silent-removal); MCP gated on confirming beta SDK Streamable-HTTP server transport | 2026-07-28 MCP RC (external) |
| 6 | Serving Phase 4: batched Lane-A operator-authorized changes (`--kv-unified`, batch-size/parallel tuning, GGUF re-pull if needed) — SINGLE coder-pause window | Serving-plan Phase 4 (Task 4.1) | **Gated** — operator coder-pause (§11.4.122) | Phases 1a/2/3 done |
| 7 | claude_toolkit LLMsVerifier trunk-bump for the extended-provider commits | Providers-plan §3 Phase G.1 | **Gated** — needs `feature/helixllm-full-extension`→`main` merge approval (§11.4.167) OR explicit deviation sign-off | Operator decision |
| 8 | Provider live-proof coverage completion (remaining ~7 of 10 CONST-039 providers + remaining ~9 of 13 extended rows) | Providers-plan §3 Phase F.3 | **Gated** — needs API keys (§11.4.10) | Phase 0 harness now exists (uncommitted) |
| 9 | Full HelixQA bank sweep (all 9 banks) + Challenges + autonomous session | Capabilities-plan §5 | Partially gated (Banks C/D/E/F wait on Phases 2b/3 landing) | Phases 2b, 3 |
| 10 | Next release tag prep — pointer bumps, §11.4.40 full-suite sweep, prefixed tag | — | **Gated** — operator decision on scope/timing | All above phases the operator wants in-scope |

---

## 3. VRAM lane budget table — the single shared GPU truth

**Live reading, re-baselined 2026-07-08 03:45:27 local** (`nvidia-smi
--query-gpu=memory.total,memory.used,memory.free`): **32607 MiB total (≈31.84 GiB), 19436 MiB
used (≈18.98 GiB, coder-only resident), 12685 MiB free (≈12.39 GiB)**. The vision co-resident
container (`helixllm_visiongen_visiongen_1`) that was up when this session's plans were first
drafted has since been **torn down** — this is the single most important reconciled fact across
both the serving plan and the capabilities plan (both independently re-baselined against it).

| Lane | Class | Residency | Size | Co-resides with coder? |
|---|---|---|---|---|
| **Lane A — coder** | `ClassCoder` | Resident, always-on, never evicted, uncounted against admission | ~19.4 GiB | N/A (baseline) |
| **Lane B — 2nd coder/agent instance** | New `ClassAgent` (broker gap, Phase 1a Task 1.4) | Warm, admission-gated | Priority: (1) Mistral-Nemo-12B Q4 ~6.52 GiB weights (~3.87 GiB KV headroom — best fit); (2) GLM-4.7-Flash smallest quant ~9.78 GiB (~0.61 GiB KV headroom — single-slot only); (3) DeepSeek-Coder-V2-Lite Q4_K_M ~9.65 GiB (~0.74 GiB KV headroom — single-slot only); (4) 2nd Qwen3-Coder replica — demoted, does not fit | Yes, within 10.39 GiB ceiling |
| **Vision (VLM)** | `ClassVLM` | Warm, currently STOPPED | 3B ~2 GiB when running; 8B upgrade tier ~12-16 GiB | Yes at 3B; 8B needs re-measure |
| **Image-gen fallback** | `ClassImage` (burst, single-owner §11.4.119) | Burst | SDXL/FLUX.1-schnell-Q4 ~7-9 GiB | **Fits NOW** (9+2=11 ≤ 12.39 GiB) |
| **Image-gen flagship** | `ClassImage` | Burst | FLUX.1-schnell fp8 ~16-20 GiB | **Does NOT fit** — needs coder-pause burst window |
| **Video-gen fast lane** | `ClassVideo` (burst, single-owner) | Burst | LTX-Video 2B-distilled ~6-8 GiB or WAN 2.2 TI2V-5B ~7-9 GiB | **Fits NOW** |
| **Video-gen flagship** | `ClassVideo` | Burst | WAN 2.2 A14B MoE fp8 ~14-20 GiB | **Does NOT fit** — needs coder-pause burst window |
| **Translation/embeddings** | `ClassTranslate`/`ClassEmbed` | Warm/CPU, 0 or tiny GPU | NLLB CPU-first ~3-4 GiB or 0 GPU | Always fits |

**Reconciled co-reside ceiling: `needBytes + 2 GiB hard-floor headroom ≤ 12.39 GiB free` ⇒
`needBytes ≲ 10.39 GiB`.** This is the number every Lane-B candidate and every generative
fallback/fast-lane tier is computed against. **Single-owner sequencing (§11.4.119):**
`ClassImage` and `ClassVideo` are mutually exclusive burst classes — never run concurrently;
Lane B (`ClassAgent`) and vision (`ClassVLM`) are independent warm-tier admissions, both gated
by the same live `Budget().free` read.

**Volatility warning (DZ-23, both plans independently flag this):** free VRAM moved from
≈7.89 GiB (coder+vision resident) to ≈12.39 GiB (coder-only) **within the same session** — proof
the number moves in EITHER direction. **Every admission decision at every phase above MUST
re-read `nvidia-smi`/`Budget().free` live immediately before acting — never trust this table's
cached numbers past the moment they were read.**

**Coder-never-casually-restart constraint (D8/§11.4.122):** `helixllm-coder` is explicitly
"never restart" without operator authorization. All Lane-A-affecting changes (kv-unified flag,
batch-size tuning, GGUF re-pull) MUST be batched into a **single** operator-authorized
coder-pause window (roadmap Phase 6) — never several separate pauses.

---

## 4. Consolidated danger-zone rollup — top 10 cross-cutting risks

Sourced from the 61-item programme register (`99_risk_bottleneck_analysis.md`, 56 items +
5 new DZ-23…AB-16 from the capabilities plan) plus the serving plan's own 8 danger zones (D1–D8).
Ranked by current relevance (several original Critical items — DZ-01…DZ-04, GPU passthrough/
toolchain/VRAM-contention/broker-design — are now **RESOLVED** per §1 and excluded from this
"still live" top-10; cite them only as history).

| # | Risk | Severity | Mitigation (already designed — land it, don't re-derive) |
|---|---|---|---|
| 1 | **DZ-23 / Live free-VRAM residency is volatile** — the single most-repeated finding across both the serving and capabilities plans this session | Med (was High) | Re-measure `Budget().free`/`nvidia-smi` immediately before EVERY admission decision; never trust a cached table (§3 above) |
| 2 | **D8 / Coder-container never-restart constraint** vs iteration need | High (process) | Batch ALL Lane-A flag changes into one operator-authorized coder-pause window (Phase 6); everything else proceeds without touching the live container |
| 3 | **D1 / VRAM OOM under concurrency burst** (Lane A + Lane B simultaneous peak) | Critical if unmitigated | Pre-boot config validator (Phase 1a Task 1.5) enforces worst-case static KV ceiling sums ≤ card total − headroom; runtime `/metrics`+`nvidia-smi` sidecar; chaos test asserting broker refuses admission rather than OS OOM-kill |
| 4 | **D6 / Tool-calling type-validation failures under load** (OpenCode #1809 array-as-string bug class) | High | Verify running GGUF postdates Unsloth's fix; reproduce-first regression test at N concurrent tool-calling turns; register as permanent §11.4.135 guard |
| 5 | **DZ-11 / Frozen-frame, silence-hallucination, analyzer-bluff family** — every media/vision/STT test could pass-bluff | High | Mandatory §11.4.107/.137/.117/.163 self-validated golden-good/golden-bad + paired §1.1 mutation on every analyzer (already the design; land it per capability as each ships) |
| 6 | **D3 / Slot starvation from prefill-heavy system prompts** (CLI-agent frameworks resend large system prompt every turn) | Med-High for the 6-12 agent goal | Verify KV-cache-reuse/prefix-caching is actually active via `/metrics` runtime signature; stress test at 8-12x concurrency measuring prefill/decode split |
| 7 | **DZ-08 / codegraph index bloat indexes third-party, violates §11.4.79** | High | Land the exclude-pattern fix (Phase 1b) **before** codegraph→RAG fusion (Phase 2b Task T6′.3) — explicit ordering rule, do not fuse first |
| 8 | **DZ-10 / claude_toolkit LLMsVerifier bump-without-rebuild** (SOURCE→ARTIFACT gap) | High | Force-rebuild binaries + re-run live tests before any gitlink bump (blocked anyway on Phase 7's operator decision) |
| 9 | **D5 / Broker `ClassVLM` reuse gap** — if Lane B reuses `ClassVLM` instead of a new `ClassAgent`, a future real vision workload collides for the same class | Med | Implement the new `ClassAgent` (Phase 1a Task 1.4), never reuse `ClassVLM` semantically for a second coding lane |
| 10 | **DZ-24/DZ-25 / HelixMemory spike doc missing + cognee has TWO known upstream bugs** | Med | Re-author `HELIXMEMORY_PROVIDER.md` from `04_embeddings_rag.md` §5 (Zep/Graphiti+mem0), independent of cognee's fate; cognee re-enable (separate, §11.4.174-blocked) must test both bug classes before any attempt |

---

## 5. OPERATOR-DECISION LIST

Every item genuinely gated on an explicit operator call, collected in one place per
§11.4.66/§11.4.101. **Count: 9.**

1. **GPU generative burst (coder-pause) for flagship image/video tiers** — FLUX.1-schnell fp8
   (~16-20 GiB) and WAN 2.2 A14B MoE fp8 (~14-20 GiB) exceed the 10.39 GiB co-reside ceiling and
   require pausing `helixllm-coder` for a scheduled full-burst window (§11.4.122). Fallback/
   fast-lane tiers do NOT need this — they fit now.
2. **Batched Lane-A serving changes (`--kv-unified`, batch-size/parallel tuning, GGUF re-pull)**
   — a single coder-pause window (§11.4.122), enumerated choices already drafted: "[A] authorize
   a single window now · [B] defer to next scheduled maintenance · [C] apply only the GGUF
   re-pull if needed, defer flag changes."
3. **Broad-provider API keys** (§11.4.10) — needed to advance the CONST-039 harness from 2/10
   LIVE-PROVEN to full coverage, and the extended-provider rows from 3/13 to full coverage.
4. **claude_toolkit → LLMsVerifier trunk merge** (§11.4.167) — the extended-provider commits
   live on `origin/feature/helixllm-full-extension`, not `origin/main`; either (a) wait for an
   operator-approved merge to `main`, or (b) point claude_toolkit's vendored submodule at the
   feature branch explicitly as a documented deviation.
5. **cognee wire / HelixMemory** — cognee re-enable is §11.4.174-blocked (foreign
   `helix_agent` go.mod/.qa_bak from a concurrent QA track) AND has its own upstream bug (two
   known classes, DZ-25); operator decision needed on whether/when to re-attempt vs proceed
   HelixMemory-only (Zep/Graphiti+mem0), which is independent and unblocked.
6. **Release-tag/version for the next tag** — scope, timing, and which of the phases above
   are included before cutting the next prefixed tag (§11.4.151).
7. **Outward push target** — `feature/helixllm-full-extension` currently has no upstream
   tracking configured; operator decision on which remote(s)/branch to push to next.
8. **GPT-Sol / Subquadratic GA-wait** — both held `UNCONFIRMED-needs-endpoint`/
   `BLOCKED-until-GA` per already-resolved clarifications; revisit only on a public GA
   announcement, no operator action needed until then (listed for completeness/tracking).
9. **A2A stub retirement** — the pre-existing `/api/v1/acp/{execute,broadcast,status}` canned
   stub routes are a §11.4.122 no-silent-removal case: operator must choose (a) replace in-place
   with the real A2A wire, or (b) add A2A alongside and deprecate the stub on a schedule.

*(Qwythos-9B provenance sign-off and /ws credential channel, named in the task brief, were
searched for in all three source plans and RESUME/PROVIDER_COVERAGE — Qwythos appears only as
a resolved self-host item in `03_open_clarifications.md` with no pending sign-off tracked in
this corpus, and no `/ws` credential-channel item was found anywhere in the three plans or
grounding docs. Both are noted here as **not found as open items in the reviewed source
material** rather than silently omitted — §11.4.6.)*

---

## 6. Recommended immediate Phase-1 work (non-operator-gated, non-coder-disturbing)

Concrete first steps producing real testable capability without touching the live coder or
needing operator input. All are independently subagent-dispatchable in parallel (§11.4.103).

### 6.0 Already in flight (commit it)
The CONST-039 provider live-proof harness (`helix_code/internal/llm/provider_live_proof_test.go`
+ `_skip_test.go`) is **complete and uncommitted** — 2/10 providers LIVE-PROVEN (Groq, Mistral),
3 honest real FAILs, 5 honest SKIPs, evidence at `docs/qa/provider_live_proof_RESULTS_20260707.md`.
**Acceptance proof:** already captured; this is a commit-and-land action, not new work.

### 6.1 Serving Lane-B second instance (fits free VRAM now)
Boot Mistral-Nemo-12B (Q4, ~6.52 GiB weights) in its own container via the `containers`
submodule, gated through the new `ClassAgent` broker class (must land first, below) and the
pre-boot config validator. **Acceptance:** benchmark report comparing tok/s + tool-calling
correctness vs GLM-4.7-Flash/DeepSeek-Coder-V2-Lite, GO/NO-GO recommendation.
(Serving-plan Task 2.1)

### 6.2 Broker `ClassAgent` addition (prerequisite for 6.1)
Add `ClassAgent Class = "agent"` to `broker.go`'s enum, warm-tier semantics identical to
`ClassVLM`. **Acceptance:** unit test asserting `IsResident()==false`/`IsBurst()==false` +
admission-gated identically to `ClassVLM`; paired §1.1 mutation flips `IsResident()` to prove
the test discriminates. (Serving-plan Task 1.4)

### 6.3 Pre-boot config validator
Small Go tool (pattern: `submodules/helix_llm/cmd/videogen-boot`) computing worst-case static
KV ceiling, `-ngl 99` full-GPU-residency achievability, and port/container-name distinctness;
refuses non-zero exit on any failure. **Acceptance:** Challenge test feeding an over-budget
config asserts non-zero exit + specific error; a valid config asserts zero exit.
(Serving-plan Task 1.5)

### 6.4 MCP/OKF wiring groundwork
Pull `github.com/modelcontextprotocol/go-sdk` latest release notes (post beta-SDK
announcement) and confirm Streamable-HTTP **server** transport coverage before committing the
gateway design. **Acceptance:** a documented go/no-go on the beta SDK; if GO, stateless-first
gateway design; if not-yet, stdio-only fallback for this cycle. (Capabilities-plan Task P4-T5′.1)

### 6.5 HelixQA vision bank (Bank B) — authored first per risk-descending priority
Author `helixllm_vision.yaml` ground-truth fixtures against the LIVE 3B model's actual behavior
(run each candidate fixture during authoring to confirm it's achievable before committing it as
"must-match"). **Acceptance:** Bank B PASS on the 3B tier + self-validated analyzer golden-bad
fixture FAILs. (Capabilities-plan §5.2 item 1)

### 6.6 codegraph/opendesign-as-core wiring fix
Rewrite `.mcp.json` codegraph entry to the portable bare-command form; add exclude patterns
closing the 6.09 GB/102k-file third-party bloat (§11.4.79); reconcile 1.1.1↔1.2.0 version drift.
Separately: install/start the opendesign daemon on `:7456`, seed the `helixcode-brand` project.
**Acceptance:** `codegraph_validate.sh` cross-submodule probe PASSes + exclude-then-restore
paired mutation FAILs when own-org content is wrongly excluded; the §11.4.78-style unforgeable
dual challenge (`od_list_projects` + `codegraph_status`) resolves real facts in one script.
(Capabilities-plan Tasks P5-T1′/P5-T2′ — flagged in that plan as "good candidate for the FIRST
parallel stream dispatched")

### 6.7 claude_toolkit alias reconciliation
Resolve the `tencent-tokenhub` vs `hunyuan` and `kimi-for-coding` vs `moonshot` ambiguities via
live endpoint comparison (one-command check), then add `sakana` (unambiguous) +
`fireworks`/`deepinfra`/`ai21`/`reka`. **Acceptance:** `claude-providers sync` produces new
`<id>.env` files with correct base URL/transport; `verify_providers_live.sh` produces real
non-simulated round-trip proof. (Providers-plan §3 Phase B)

### 6.8 Image/video-gen fallback/fast-lane runtime proof (no pause needed right now)
Because vision is currently stopped, the fallback tiers (SDXL/FLUX-schnell-Q4) and fast lane
(LTX-Video/WAN-TI2V-5B) fit the live ≈12.39 GiB free without pausing anything. Re-measure
`Budget().free` live immediately before attempting; if still free, run the full generate-and-
verify cycle (CLIPScore + golden-bad fixtures) now. **Acceptance:** exact §5 signature from
`IMAGE_GEN_PROVIDER.md`/`VIDEO_GEN_PROVIDER.md` — captured evidence, register as permanent
regression guard. **Caveat:** if VRAM state has changed by execution time (volatility, §4 item 1),
this step honestly falls back to requesting the operator-gated pause (item 1 of §5) rather than
forcing admission. (Capabilities-plan Tasks P3-T2′/T3′)

---

## Sources

- `docs/research/07.2026/01_local_models_serving/IMPLEMENTATION_PLAN_v2.md`
- `docs/research/07.2026/02_vision_generative/CAPABILITIES_MASTER_PLAN_v2.md`
- `docs/research/07.2026/06_providers_coverage/EXPANSION_PLAN_v2.md`
- `docs/research/07.2026/00_master/RESUME.md`
- `docs/research/07.2026/00_master/PROVIDER_COVERAGE.md`
- `docs/research/07.2026/00_master/03_open_clarifications.md`
- `docs/research/07.2026/99_risk_analysis/99_risk_bottleneck_analysis.md`
- `docs/qa/provider_live_proof_RESULTS_20260707.md` (in-flight uncommitted evidence, cited for §6.0)
- Direct source reads (not web, this consolidation pass): `helix_code/internal/llm/provider_live_proof_test.go`, `docs/qa/helixllm_vision_boot_20260707T215007Z/` (directory listing)
