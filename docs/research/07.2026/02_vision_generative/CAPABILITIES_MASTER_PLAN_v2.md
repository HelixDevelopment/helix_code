# HelixLLM Full Local-Capability Master Plan v2 — Vision, Generative, Translation, Embeddings/RAG/HelixMemory, MCP+OKF, codegraph/opendesign, HelixQA

| | |
|---|---|
| **Document** | Capabilities master plan v2 (phased implementation, danger-zone consolidation, HelixQA extension) |
| **Path** | `docs/research/07.2026/02_vision_generative/CAPABILITIES_MASTER_PLAN_v2.md` |
| **Revision** | 2 · **Created** 2026-07-08 · **Revised** 2026-07-08 (§11.4.134 independent-review remediation: risk-register recount, VRAM re-baseline after vision-container teardown, StarVector caveat, doc-sibling correction) |
| **Track/branch** | `(T1/feature/helixllm-full-extension)` |
| **Status** | PLAN — extends, does NOT replace, the Phase-R/Phase-P research corpus |
| **Does NOT duplicate** | `00_master/00_programme_master_plan.md`, `00_master/04_implementation_plan.md`, `99_risk_analysis/99_risk_bottleneck_analysis.md`, `02_vision_generative/02_vision_generative.md`, `00_master/IMAGE_GEN_PROVIDER.md`, `00_master/VIDEO_GEN_PROVIDER.md`, `03_translation/03_translation.md`, `04_embeddings_rag/04_embeddings_rag.md`, `00_master/EMBEDDINGS_PROVIDER.md`, `05_mcp_acp_protocols/05_mcp_acp_protocols.md`, `00_master/ACP_A2A_PROVIDER.md`, `07_stt_ocr_whisper_tesseract/07_stt_ocr_whisper_tesseract.md`, `08_codegraph_opendesign/08_codegraph_opendesign.md`, `12_helixqa_testing/12_helixqa_testing.md` — this document **indexes, reconciles against live state, and sequences** them into one ready-to-execute plan. Read the cited source doc for the full evidence chain before implementing any line item.

> **Anti-bluff notice (§11.4 / §11.4.123).** Every model recommendation below carries
> a cited source (from the referenced sibling doc) or a freshly-captured host fact
> (marked `[LIVE, 2026-07-08]`). Design estimates are labelled `(EST — measure)`.
> `UNCONFIRMED:` / `PENDING_FORENSICS:` mark anything not yet proven. No placeholder
> or mocked implementation ships (§11.4.2 / CONST-050).

---

## 0. Grounding — what is PROVEN vs DESIGNED vs SCAFFOLD right now (2026-07-08)

Per RESUME.md (§11.4.131 standing resumption file) the branch is **RELEASE-READY / RELEASE PUBLISHED** (`helix-code-1.0.0-dev-0.0.1`). This document plans the **next-tag scope**, continuing the endless-loop (§11.4.126) past the release.

### 0.1 Live host facts (captured this session, §11.4.6)

| Fact | Value | Source |
|---|---|---|
| GPU | RTX 5090, 32607 MiB total VRAM | `nvidia-smi` [LIVE, 2026-07-08] |
| VRAM used / free right now | **RE-BASELINED (§1.3): 19436 MiB used / 12685 MiB free ≈ 18.98 GiB used / 12.39 GiB free**, coder-only resident — supersedes the as-drafted reading of `24047 MiB used / 8074 MiB free` (≈23.48 GiB / **≈7.89 GiB** — the draft's "8.07 GiB" label was `8074÷1000`, not the correct `÷1024`) captured earlier the same session while the vision container was still co-resident | `nvidia-smi --query-gpu=memory.total,memory.used,memory.free` [LIVE RE-BASELINE, 2026-07-08 03:45:27 local / 2026-07-07T22:45:27Z UTC] |
| Resident containers | **RE-BASELINED:** only `helixllm-coder` (Qwen3-Coder-30B-A3B, port 8080/50052 internal, single process `llama-server` 19426 MiB) is resident now. `helixllm_visiongen_visiongen_1` (same image, was host port `18439`) has been **TORN DOWN** since this plan was first drafted (it was co-resident at the time of the original capture above) | `podman ps` [LIVE RE-BASELINE, 2026-07-08 03:45:27 local] |
| Vision model actually serving | **`Qwen2.5-VL-3B-Instruct-Q4_K_M.gguf`** — `capabilities:["completion","multimodal"]`, `n_ctx:16384`, `n_ctx_train:128000`, `n_params:3,085,938,688` (~3.09B), size **1.92 GB** on disk. **NOTE (re-baseline):** this container is currently STOPPED (see Resident-containers row above) — config/model choice unchanged, restart on demand | `curl localhost:18439/v1/models` [LIVE, 2026-07-08 — pre-teardown capture] |
| VRAM broker code | `submodules/helix_llm/internal/vrambroker/broker.go` (204 lines) — `ClassCoder` (resident) / `ClassVLM` / `ClassImage` / `ClassVideo` (burst, single-owner) / `ClassTranslate` (warm) / `ClassEmbed` (CPU, 0-byte) — **CORE implemented, reviewed GO** (a12df57c); `admit(free, needBytes, headroom)` fail-closed on `free < need+2GiB`; **no eviction/pause-warm-tier logic yet** (honest gap, §2.3 of `IMAGE_GEN_PROVIDER.md`/`VIDEO_GEN_PROVIDER.md`) | Read `broker.go` this session |
| `submodules/helix_llm/docs/` | `API_CONTRACT.md` (+html/pdf), `OPERATOR_GUIDE.md` (+html/pdf), `VRAM_BROKER.md` (**.md only — verified on disk this pass: no `.html`/`.pdf` sibling exists**), plus `qa/`, `research/`, `specs/`, `schemas/` — the canonical HelixLLM contract now EXISTS (closes G-GAP-A / DZ-05 from the risk register) | `ls -la submodules/helix_llm/docs/` [LIVE, verified 2026-07-08] |
| cognee bug | `submodules/helix_agent/docs/COGNEE_BUG.md` — `AttributeError: 'str' object has no attribute 'nodes'` in `cognee/tasks/memify/extract_subgraph_chunks.py:9`; `/api/v1/search`, `/api/v1/add`, `/api/v1/memify` all **time out (30s)**; health + auth work; **STATUS: BLOCKED, waiting for upstream fix**, cognee container running but disabled in config | Read `COGNEE_BUG.md` this session + fresh web search (below) found no newer public fix for this exact trace |
| Which "ACP" | **Resolved operator decision**: Google **A2A**, NOT Zed ACP (`00_master/03_open_clarifications.md` C3, honoured in `ACP_A2A_PROVIDER.md`) | Read `ACP_A2A_PROVIDER.md` |

### 0.2 Readiness table (what the caller asked for)

| Capability | Readiness | Recommended local model (from cited research) | Owning doc |
|---|---|---|---|
| **Vision (VLM)** | **PROVEN** (co-resident, 3B, proven live on :18439) — **container currently STOPPED as of this revision's re-baseline** (§1.3, §0.1) — hardening/upsize is DESIGNED | live-when-running: `Qwen2.5-VL-3B-Instruct-Q4_K_M` GGUF via llama.cpp `-ngl 999 --ctx-size 16384`; upgrade path: `Qwen3-VL-8B-Instruct` (co-resident, ~12-16 GB — fits within the current ≈12.39 GB free at the low end, or needs a burst-tier swap once vision+coder+8B all co-reside; re-measure `Budget().free`, never assume 8 GB) or keep 3B as the always-warm default and add 8B as an on-demand `ClassVLM` warm-swap | `02_vision_generative.md` §1; `VIDEO_GEN_PROVIDER.md`/`broker.go` `ClassVLM` |
| **Generative — image** | **SCAFFOLD-COMPLETE** (helix_llm `0f07559`, port 18442, broker-integrated, self-validated CLIPScore analyzer, RED-first) — **RUNTIME PROOF PENDING** operator coder-pause (pause now only needed for the flagship tier — see §1.3 re-baseline) | co-reside fallback: SDXL fp16 / FLUX.1-schnell-Q4 GGUF (sd.cpp, ~7-9 GB) — **fits the live ≈12.4 GB free NOW** (vision container currently stopped; re-measure `Budget().free` at admission time, §1.3/§2.3 — residency is volatile); flagship: FLUX.1-schnell fp8 via ComfyUI (~16-20 GB, still needs full burst) | `IMAGE_GEN_PROVIDER.md` |
| **Generative — video (WAN+LTX)** | **SCAFFOLD-COMPLETE** (helix_llm `9145505`, port 18443, `ClassVideo` broker-integrated, liveness+CLIPScore analyzer) — **RUNTIME PROOF PENDING** operator coder-pause (pause now only needed for the flagship tier — see §1.3 re-baseline) | co-reside fast lane: LTX-Video 2B-distilled fp8 (~6-8 GB) or WAN 2.2 TI2V-5B (~7-9 GB) — **both fit the live ≈12.4 GB free NOW** (vision container currently stopped; re-measure `Budget().free` at admission time, §1.3/§2.3 — residency is volatile); flagship: WAN 2.2 A14B MoE fp8 (~14-20 GB, still needs full burst) | `VIDEO_GEN_PROVIDER.md` |
| **Vectorize (pixel→SVG)** | **NET-NEW** (design in `02_vision_generative.md` §4, not yet a P3-Tx spike doc) | vtracer (CPU, 0 GPU) raster-trace default — **also the real path for natural-image/illustration/pixelized-graphic vectorization**; StarVector-8B (VLM, ~12-16 GB) for a narrower smart-mode tier **CAVEAT: per its own model card, StarVector "will not work for natural images or illustrations, as they have not been trained on those images" — it excels only at icons, logotypes, technical diagrams, graphs, and charts** ([StarVector-8B model card](https://huggingface.co/starvector/starvector-8b-im2svg), accessed 2026-07-08); potrace for mono | `02_vision_generative.md` §4 |
| **Translation** | **DESIGNED** (P3-T5, no spike doc yet distinct from `03_translation.md`) | NLLB-200-3.3B int8 via CTranslate2 (~3-4 GB, CPU-first, **0 GPU** or tiny GPU footprint) as always-warm; TOWER+9B via vLLM (~18 GB, GPU, needs a warm-tier slot) for quality tier | `03_translation.md` §0/§1 |
| **STT (Whisper)** | **IMPLEMENTED, reviewed GO** (18437, per RESUME) | faster-whisper large-v3 (CTranslate2, GPU int8, ~2-3 GB) already live; Parakeet-TDT-0.6B-v3 recommended add-on for streaming/no-hallucination | `07_stt_ocr_whisper_tesseract.md` §1 |
| **OCR (Tesseract)** | **IMPLEMENTED, reviewed GO** (18438, per RESUME) | Tesseract 5.5 OEM1 `tessdata_best` tier-1; PaddleOCR/Surya confidence-triggered fallback | `07_stt_ocr_whisper_tesseract.md` §2 |
| **Embeddings + RAG** | **IMPLEMENTED, PROVEN** (55bdf9b6, bge-small via TEI, cos margin 0.3578) — Qdrant/rerank/HelixMemory-fusion still DESIGNED | Qwen3-Embedding-4B (general+code) + BGE-M3 (sparse) via TEI/Infinity; bge-reranker-v2-m3; Qdrant vector DB | `04_embeddings_rag.md`; `EMBEDDINGS_PROVIDER.md` |
| **HelixMemory / cognee** | **DESIGN DONE, IMPL BLOCKED** by §11.4.174 (concurrent QA track's foreign `helix_agent` go.mod/.qa_bak) AND by the live cognee upstream bug (§0.1) | Primary durable store recommendation UNCHANGED from research: **Zep/Graphiti** (temporal KG) + mem0 (extraction) via MCP — cognee is a SEPARATE, already-integrated-but-broken component in `helix_agent`, not the P-OQ2-A design target | `04_embeddings_rag.md` §5; `scratchpad/design_helixmemory_cognee.md` referenced in RESUME but **not found on disk this session** — treat that reference as **STALE/missing**, re-author before implementing (see §4.7) |
| **MCP** | **DESIGNED** (official Go SDK identified, beta SDKs for 2026-07-28 RC now published per fresh search) — HelixLLM/HelixAgent wiring NOT yet implemented | `github.com/modelcontextprotocol/go-sdk` (Google-co-maintained); stateless-first design | `05_mcp_acp_protocols.md` §1/§7 |
| **ACP → A2A** | **DESIGNED** (operator-resolved to Google A2A; Revision-2 spike distinguishes the pre-existing PROPRIETARY stub routes from the real A2A wire) | `a2aproject/A2A` v1.0.1 (Linux-Foundation), JSON-RPC+SSE+HTTP, official Go SDK | `ACP_A2A_PROVIDER.md` |
| **OKF (Google Open Knowledge Format)** | **DESIGNED as content, not protocol** — recommendation stands: OKF is the on-disk format for RAG/Skills knowledge, served through an MCP `resources` server | `05_mcp_acp_protocols.md` §3 |
| **codegraph as core provider** | **DESIGN COMPLETE + defects PROBED** (`.mcp.json` macOS-path rot, version drift 1.1.1↔1.2.0, index bloat 6.09 GB/102k files from vendored third-party) — wiring fix NOT YET LANDED | `codegraph` npm `1.2.0`, `serve --mcp` bare-command | `08_codegraph_opendesign.md` §1.1/§2/§4 |
| **opendesign as core provider** | **DESIGN COMPLETE, fully unwired** (disabled MCP entry, daemon down, `od`↔coreutils collision noted) | `open-design-mcp` npm `0.16.1`, daemon `:7456` | `08_codegraph_opendesign.md` §1.2/§3/§5.2 |
| **HelixQA vision/full-capability testing** | **DESIGN COMPLETE** (9 banks A–I, `ContentAssertingResolver` + conduit + `pkg/vision`/`pkg/recordingqa` already shipped in HelixQA) — bank YAML + per-capability analyzers NOT yet authored; blocked on HelixLLM contract confirmation (**now resolved, §0.1** — `API_CONTRACT.md` exists) | See `12_helixqa_testing.md` Parts 1-6; §5 of this document extends it | `12_helixqa_testing.md` |
| **Provider coverage (Poe/Perplexity/Sakana/Xiaomi/Tencent/…)** | Out of THIS document's scope (owned by `06_providers_coverage.md` / Phase 4) — noted for completeness only | — | `06_providers_coverage.md` |

---

## 1. Deep multi-angle research delta (§11.4.150 / §11.4.99) — what changed / was added since 2026-07-06/07

The 13-report Phase-R corpus is 24-48h old and already carries per-domain citations at the depth this task asks for. This section adds only the **genuinely new** facts found in this pass, each with URL + access date 2026-07-08.

### 1.1 MCP — beta SDKs for the 2026-07-28 RC now exist

- The MCP blog confirms the RC was **locked 2026-05-21**, finalizes **2026-07-28** (20 days from today), and a **companion post announces beta SDKs for the RC are now available** — closing part of the `05_mcp_acp_protocols.md` §1.3 "UNCONFIRMED: Go SDK Streamable-HTTP transport" gap. **Action for P4-T5:** before wiring, pull the current `go-sdk` release notes and confirm the beta explicitly ships a Streamable-HTTP **server** transport (not just client) — the beta-availability fact is confirmed, the exact transport coverage is not yet re-verified this session. ([MCP blog — 2026-07-28 RC](https://blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/), accessed 2026-07-08; [MCP blog — Beta SDKs for 2026-07-28 RC](https://blog.modelcontextprotocol.io/posts/sdk-betas-2026-07-28/), accessed 2026-07-08)
- The stateless-core summary is reconfirmed independently by three secondary sources (Stacktree, MCP.Directory, jsmanifest): *"The initialize/initialized handshake is gone. The session header is gone. Every request is now self-contained."* This validates the `05_mcp_acp_protocols.md` §7 design decision to build the HelixLLM/HelixAgent MCP gateway **stateless-first**, now with higher confidence since it is 20 days from finalization, not months. ([Stacktree](https://stacktr.ee/blog/mcp-2026-spec-changes); [MCP.Directory](https://mcp.directory/blog/mcp-2026-07-28-release-candidate); accessed 2026-07-08)

### 1.2 cognee — the upstream bug has no newer public fix found (negative finding, §11.4.99(B))

A targeted search for the exact trace (`extract_subgraph_chunks`, `AttributeError: 'str' object has no attribute 'nodes'`) against `topoteretes/cognee` returned no matching issue in the indexed search results (issue numbers near the guessed range are unrelated: #2090 request-full-document, #2560 governance-validation, #2119 a *different* hang-on-`cognify()` bug on macOS with a local OpenAI-compatible LLM — itself relevant as a **second known cognee/local-LLM interaction bug**, worth flagging for when cognee is eventually re-enabled against HelixLLM's local endpoint). **Recommendation (unchanged from RESUME, now reinforced):** do NOT re-attempt cognee wiring by hoping the bug is fixed; instead (a) pin and test a specific newer cognee release for this exact trace before any re-enable attempt, (b) treat `docs/topoteretes/cognee/issues` triage as its own tracked P-OQ2-B task, (c) proceed with the Zep/Graphiti + mem0 HelixMemory path (§4.7) as the primary durable-memory deliverable, independent of cognee's fate. ([topoteretes/cognee issues](https://github.com/topoteretes/cognee/issues), accessed 2026-07-08 — negative finding, exact trace not found; [topoteretes/cognee#2119 — cognify() hangs with local OpenAI-compatible LLM on macOS](https://github.com/topoteretes/cognee/issues/2119), accessed 2026-07-08, cited as an adjacent local-LLM-interaction risk)

### 1.3 Live-state reconciliation with the design docs (this session's own probe, corrected + re-baselined)

- `IMAGE_GEN_PROVIDER.md` and `VIDEO_GEN_PROVIDER.md` were both authored against a **~12.7 GB free** baseline (§0 of each doc, dated 2026-07-07). **A first live read this session** found `24047 MiB used / 8074 MiB free` — i.e. ≈23.48 GiB used / **≈7.89 GiB free** (the plan's original draft mislabeled this figure "8.07 GiB"; `8074 MiB ÷ 1024 = 7.885 GiB`, not `÷1000`; the used+free-vs-total gap — `24047+8074=32121` vs total `32607` — is a consistent ≈486 MiB reserved/driver-overhead artifact, reconciled below as an expected constant, not an anomaly), because the vision co-resident container (`helixllm_visiongen_visiongen_1`) was ALSO resident alongside the coder at that time (24047 MiB used vs the docs' assumed ~19.4 GB).
- **CRITICAL LIVE-STATE UPDATE (this revision pass, 2026-07-08 03:45:27 local / 2026-07-07T22:45:27Z UTC):** the vision co-resident container has since been **TORN DOWN** — `podman ps` now shows only `helixllm-coder` resident (single process `llama-server`, 19426 MiB). A fresh `nvidia-smi` read gives `19436 MiB used / 12685 MiB free` (total unchanged at 32607 MiB; gap `19436+12685=32121` vs `32607` = the same ≈486 MiB overhead as the first reading, confirming it is a stable driver-reservation constant rather than a measurement error) — i.e. **≈18.98 GiB used / ≈12.39 GiB free**, coder-only.
- **This re-baselines the co-reside admission math**: `needBytes + 2 GiB headroom ≤ 12.39 GiB` ⇒ `needBytes ≲ 10.39 GiB` — ROOMIER than both the stale ~12.7 GiB design-doc assumption's derived `≲10.7 GiB` and the mid-session tight reading's corrected `≲5.89 GiB` (fixing the draft's mislabeled `≲6.07 GiB`). Concretely, with vision now stopped:
  - Image co-reside fallback (`FLUX.1-schnell-Q4 GGUF ~7-9 GB` OR `SDXL ~7-9 GB`) **fits again** (`9+2=11 GiB ≤ 12.39 GiB free`) — the borderline/refused state found earlier this session (while vision was still resident) no longer applies at the current residency.
  - Video co-reside fast lane (`LTX-Video 2B-distilled ~6-8 GB`) **fits comfortably** (`8+2=10 GiB ≤ 12.39 GiB free`), as does `WAN 2.2 TI2V-5B ~7-9 GB` (`9+2=11 ≤ 12.39`) — no longer "tight."
  - Flagship tiers still do **NOT** fit co-resident: `FLUX.1-schnell fp8 (~16-20 GB)` and `WAN 2.2 A14B MoE fp8 (~14-20 GB)` both exceed the `10.39 GiB` ceiling even now — these still require a scheduled full-burst window pausing the coder (§11.4.122 operator authorization, since pausing `helixllm-coder` is a live user-visible-latency change).
  - **This is exactly the broker's job to catch** (fail-closed, `ErrBudgetExceeded`, never an OOM) — the practical effect is that the P3-T2/P3-T3 runtime-proof step no longer needs an operator-authorized *vision* pause for the fallback/fast-lane tiers (there is nothing to pause — vision is already stopped); it only needs the coder-pause path for the flagship tiers. **Action:** re-measure `Budget().free` immediately before every P3-T2′/T3′ admission attempt — do NOT carry forward either this session's mid-point tight reading or this re-baseline as a cached constant. Residency state (which containers are up) demonstrably changed within this single session, moving the ceiling from ≈5.89 GiB to ≈10.39 GiB — proof that the number can move in EITHER direction, not only downward (tracked as DZ-23, re-scored Med, §2.3/§2.6).

### 1.4 Vision hardening — is Qwen2.5-VL-3B "hardened" or does it need the Qwen3-VL upgrade the research recommended?

`02_vision_generative.md` §1.1 recommends **Qwen3-VL-8B** as "the local VLM to beat" and 3B only as an edge/fast tier. The LIVE deployment runs **Qwen2.5-VL-3B** (the prior generation, smallest tier) — a materially smaller/older model than the research's primary recommendation. This is not a defect (3B is genuinely co-resident-safe and already proven working — real evidence beats a bigger unproven model) but IS the concrete "harden + full exposure" gap the operator asked about:
1. **Accuracy gap** — 3B dense vs 8B dense: per `codersera` (cited in `02_vision_generative.md` §1.1), MathVista improved 68.2→85.2 across Qwen versions/sizes; a 3B-vs-8B gap on document/chart/OCR-heavy tasks is real and should be measured on HelixQA's own vision fixtures (Bank B, §5) before deciding whether to upgrade the always-warm default or add an on-demand higher-quality tier.
2. **Context** — the live model reports `n_ctx:16384` (server-configured) against `n_ctx_train:128000` — plenty of headroom to raise `--ctx-size` if larger images/longer prompts are needed; no model change required for this.
3. **`Qwen2.5-VL` vs `Qwen3-VL`** — Qwen3-VL is confirmed (per the existing research) to be the *2026 successor*; no new finding this session changes that recommendation. **Plan:** keep the live 3B as the always-resident default (it fits comfortably at either re-baselined free-VRAM figure captured this session — ≈7.89 GiB with vision co-resident, or ≈12.39 GiB now that vision is stopped, see §1.3), and add Qwen3-VL-8B (or -30B-A3B) as a `ClassVLM` **warm** tier loaded on-demand for quality-sensitive HelixQA/agent requests, exactly as `broker.go`'s `ClassVLM` already anticipates. No new engine — same llama.cpp GGUF+mmproj path, model-id swap only (config-injected, §CONST-046).

---

## 2. Multi-pass danger-zone analysis (§11.4.92 5-pass) — delta over `99_risk_bottleneck_analysis.md`

The existing risk register (DZ-01…DZ-22, AB-01…AB-14, V-01…V-20) is comprehensive and NOT re-litigated here. This section adds capability-extension-specific danger zones surfaced by grounding this plan in the LIVE host state.

### 2.1 Pass 1 (goal verification) — delta
All capability elements the operator asked for (vision, generative image/video/vector/translation, embeddings/RAG/HelixMemory, STT/OCR, codegraph/opendesign, HelixQA extension) have a researched landing place (§0.2 table). **No net-new capability gap.** The one FRESH gap: `scratchpad/design_helixmemory_cognee.md`, cited in RESUME as "design DONE," **was not found on disk this session** (`find` returned empty). Either it was never committed, lives in an uncommitted/gitignored scratchpad state, or the path changed. **Action:** treat the HelixMemory (Zep/Graphiti+mem0) design as **NOT YET AUTHORED** at a spike-doc level — `04_embeddings_rag.md` §5 has the recommendation but not the P3-T7-level implementation spec `EMBEDDINGS_PROVIDER.md`-style. This is now DZ-23 (below).

### 2.2 Pass 2 (regression / blast-radius) — delta
Two NEW touch points from this session's grounding:
- **BR-11 (new): the vision container (`helixllm_visiongen_visiongen_1`) is an unreviewed-by-this-document capability whose residency state is volatile — it was running when this plan was first drafted and has since been torn down (§1.3, re-confirmed via `podman ps` this revision pass).** Any VRAM-budget change to the coder or any new capability container MUST re-measure `Budget().free` against WHATEVER is actually resident at that moment, never assume either "vision is up" or "vision is down" from this document. Guard: every P3-Tx acceptance step re-reads live `nvidia-smi`+`podman ps` immediately before admission (already the broker's own discipline — this just flags that BOTH the design docs' arithmetic AND this plan's own §1.3 arithmetic go stale the moment residency changes, and must not be trusted at face value).
- **BR-12 (new): `submodules/helix_agent`'s COGNEE_BUG.md documents a cognee↔local-LLM interaction failure class** (`cognify()` hangs with a local OpenAI-compatible LLM on macOS, per the fresh search #2119) that is DIFFERENT from the already-known `extract_subgraph_chunks` bug. Any future cognee-re-enable attempt against HelixLLM's local OpenAI-compatible endpoint MUST test for BOTH failure modes, not just the one already documented.

### 2.3 Pass 3 (cross-feature interaction) — delta
**DZ-23 (new, re-scored Med — was High at first draft): live free-VRAM residency is VOLATILE and MUST always be re-measured, never assumed from a cached design-doc OR this plan's own prior draft.** This plan's first live read this session, taken while the vision co-resident container was still up, found free VRAM tighter than the design docs assumed (~7.89 GiB actual vs ~12.7 GiB assumed — corrected unit conversion, §1.3). **A second live read taken during this revision pass (`nvidia-smi`, 2026-07-08 03:45:27 local) found the vision container (`helixllm_visiongen_visiongen_1`, was on host port `18439`) has since been TORN DOWN** — only `helixllm-coder` is resident now, and free VRAM is currently ≈12.39 GiB (12685 MiB), roomier than either prior reading. New safe co-reside ceiling: `needBytes + 2 GiB headroom ≤ 12.39 GiB` ⇒ `needBytes ≲ 10.39 GiB`. This REOPENS the image co-reside fallback (~7-9 GB, `9+2=11 ≤ 12.39` → fits) and the video co-reside fast lane (~6-8 GB, `8+2=10 ≤ 12.39` → fits) and `WAN 2.2 TI2V-5B` (~7-9 GB → fits) WITHOUT needing to pause anything, since vision is already stopped. Flagship tiers (FLUX.1-schnell fp8 ~16-20 GB; WAN 2.2 A14B MoE fp8 ~14-20 GB) still exceed the 10.39 GiB ceiling and still require a scheduled full-burst window (pause the coder, §11.4.122 operator authorization). **Mitigation (unchanged in kind, re-targeted):** (a) every P3-T2/T3 runtime-proof attempt MUST re-read `Budget().free` immediately before acquiring the lease — this session is itself the proof that the number moves in EITHER direction between two reads taken hours apart, not only downward; (b) do NOT assume the vision container is either up or down — check `podman ps` at proof time, not this document; (c) the "always-warm 3B vision as a `ClassVLM` warm-swappable member" trade-off from the prior draft is now LOWER-URGENCY (headroom is currently ample without evicting vision) but remains worth raising with the operator as a resilience measure for whenever vision IS resident again, not resolved unilaterally (§11.4.101).

### 2.4 Pass 4 (deep-research validation gaps) — delta
- **V-21 (new):** Beta MCP Go SDK for the 2026-07-28 RC — confirm the SDK's Streamable-HTTP **server** transport coverage before committing P4-T5's gateway design to it (§1.1 above narrows but does not close the prior V-08 gap).
- **V-22 (new):** cognee's exact upstream-fix status for `extract_subgraph_chunks` — NOT found in this session's search; must be re-verified against `topoteretes/cognee`'s live issue tracker + CHANGELOG before any re-enable attempt (never assume fixed-by-now).
- **V-23 (new):** the live free-VRAM assumption in `IMAGE_GEN_PROVIDER.md`/`VIDEO_GEN_PROVIDER.md` (~12.7 GB) is a **moving target, not a fixed drift** — this session alone captured it at ≈7.89 GiB (coder+vision resident, first reading) and later, after the vision container was torn down, at ≈12.39 GiB (coder-only, re-baselined 2026-07-08 03:45:27 local) — nearly back to the design docs' original assumption. Every subsequent read of those docs' §1.5 VRAM tables MUST re-measure live, never trust a cached number in either direction; the docs should carry a "re-measure, do not trust this table" banner when this plan lands (tracked as a follow-up doc-hygiene task, not blocking).

### 2.5 Pass 5 (anti-bluff surface) — delta
- **AB-15 (new): "vision endpoint responds to `/v1/models`" is NOT proof the VLM correctly understands images.** The live-endpoint check this session (`curl localhost:18439/v1/models`) is a **metadata-only** probe — it proves the process is up and the model file loaded, nothing about actual multimodal correctness. This is EXACTLY the §11.4.107(10)/HelixQA-Bank-B self-validated-analyzer gap the plan closes in §5 below — flagging here so it is not mistaken for a capability proof in any status doc.
- **AB-16 (new): a resident container list (`podman ps`) is not proof of VRAM-budget correctness.** Two containers being "up" says nothing about whether their combined footprint already exceeds a safe margin for a third burst request — only a live `Budget().free` read + the broker's `admit()` decision is authoritative (this is what DZ-23 exists to guard).

### 2.6 Consolidated NEW danger-zone register (this session)

| ID | Title | Sev | Owning phase |
|---|---|---|---|
| DZ-23 | Live free-VRAM residency is volatile — re-measured 2026-07-08 03:45:27 local at ≈12.39 GiB (coder-only, vision container torn down since drafting), was ≈7.89 GiB (coder+vision) at first capture — co-reside fallback/fast-lane tiers now fit again; flagship tiers still need a full burst | **Med** (was High) | P3-T2/T3 runtime-proof step |
| DZ-24 | `scratchpad/design_helixmemory_cognee.md` referenced in RESUME not found on disk — HelixMemory (Zep/Graphiti+mem0) spike doc must be (re-)authored before P3-T7 implementation | Med | P3-T7 (this plan's Phase 3.7) |
| DZ-25 | cognee has a SECOND known local-LLM interaction bug (`cognify()` hangs on macOS with local OpenAI-compatible endpoints, #2119) distinct from the documented `extract_subgraph_chunks` bug — both must be tested before any re-enable | Med | Deferred, tracked (cognee is §11.4.174-blocked anyway) |
| AB-15 | `/v1/models` liveness ≠ VLM correctness — must not be cited as a vision-capability proof anywhere | Med | HelixQA Bank B (§5) |
| AB-16 | Container-up (`podman ps`) ≠ VRAM-budget safety — only `Budget().free`+`admit()` is authoritative | Med | Broker-integration acceptance steps (already designed in `IMAGE_GEN_PROVIDER.md` §2/`VIDEO_GEN_PROVIDER.md` §2, reinforced here) |

**Danger-zone count this pass: 5 new (DZ-23, DZ-24, DZ-25, AB-15, AB-16)**, on top of the 22 DZ + 14 AB + 20 V items already in `99_risk_bottleneck_analysis.md` (**56 total prior** — recounted 2026-07-08 via `grep -oE 'DZ-[0-9]+|AB-[0-9]+|V-[0-9]+' 99_risk_bottleneck_analysis.md | sort -u`: DZ-01…DZ-22 (22) + AB-01…AB-14 (14) + V-01…V-20 (20) = 56, correcting this document's earlier "41" miscount + 5 new = **61 enumerated danger zones/anti-bluff surfaces** across the whole programme as of this plan).

---

## 3. Runtime-signature contract per capability (§11.4.108) — quick index

Every capability below has its FULL runtime-signature spec already written in its owning sibling doc; this table is the one-hop index so implementation does not need to re-derive it.

| Capability | Runtime signature (definition of done) | Full spec |
|---|---|---|
| Vision | Golden-good/golden-bad VLM answer-correctness + hallucination/spatial/OCR-cross-check, self-validated analyzer | `12_helixqa_testing.md` Bank B; `02_vision_generative.md` §1 |
| Image-gen | CLIPScore prompt-image correspondence + semantic-order margin vs unrelated control + non-trivial-content check, golden-good/bad fixtures | `IMAGE_GEN_PROVIDER.md` §5 |
| Video-gen | ffprobe frame-count/codec/geometry + freeze-detection/frame-advance liveness + per-frame CLIPScore + semantic-order margin, golden-good/bad fixtures | `VIDEO_GEN_PROVIDER.md` §5 |
| Vectorize | Raster→SVG round-trip: re-rasterize the produced SVG, assert perceptual similarity to source within tolerance | `04_implementation_plan.md` P3-T4; NEW spike needed (§4.3) |
| Translation | COMET/CometKiwi QE gate + back-translation metamorphic (X→Y→X′ similarity) + entity/target-lang-preservation guard (catches a no-op "translator") | `03_translation.md` §1.3; `12_helixqa_testing.md` Bank E |
| Embeddings/RAG | Real vectors stored+queryable, stable dim, NaN-free; Recall@K/MRR/nDCG on a labelled set; RAGAS faithfulness with a negative (unsupported-claim) case | `EMBEDDINGS_PROVIDER.md`; `12_helixqa_testing.md` Bank F |
| STT | WER/CER vs ground-truth transcript, RMS-floor non-silence pre-check, Parakeet no-hallucination path for the assertion lane | `07_stt_ocr_whisper_tesseract.md` §1; `12_helixqa_testing.md` Bank G |
| OCR | CER vs ground-truth + per-word confidence floor + ROI, chrome-label denylist reused from §11.4.137 | `07_stt_ocr_whisper_tesseract.md` §2; `12_helixqa_testing.md` Bank H |
| codegraph | Cross-submodule symbol resolution probe (a fact obtainable ONLY via `codegraph_status`/`codegraph_explore`) + paired exclude-mutation (§11.4.79(5)) | `08_codegraph_opendesign.md` §4.2 |
| opendesign | `od_list_projects` returns the seeded `helixcode-brand` project + a codegraph fact in the SAME unforgeable challenge (§11.4.78-style) | `08_codegraph_opendesign.md` §4.4 |
| MCP/A2A | A live MCP `tools/list` round-trip returning real Helix tool schemas; a live A2A Agent-Card + task round-trip (not the pre-existing proprietary stub) | `05_mcp_acp_protocols.md` §7; `ACP_A2A_PROVIDER.md` §2 (rev 2 distinguishes real-wire from stub) |

---

## 4. Phased implementation plan (extends `04_implementation_plan.md` Phase 3/4/5 with fine-grained tasks/subtasks)

This section is additive to the existing Phase 0-9 plan (`00_master/04_implementation_plan.md`). Phases 0-2 (GPU foundation, serving core, gateway) are marked complete/GO per RESUME; this plan picks up at **Phase 3 (extended capabilities)** and **Phase 4/5 (protocols + core providers)** with line-item granularity, plus the **Phase 8 HelixQA extension** worked out to bank level (§5).

### Phase 3 — Extended capabilities (per-capability: model → container → broker class → gateway route → verifier probe → HelixQA bank → captured proof)

#### P3-T1′ — Vision hardening (extends the already-PROVEN 3B deployment)
- **T1′.1** Capture a `Budget().free` baseline with BOTH coder+vision resident (captured earlier this session: ≈7.89 GiB — corrected from the draft's mislabeled "8.07 GiB"; `MiB÷1024`, not `÷1000`). Document it as the new "vision-hardening worst-case baseline" in `submodules/helix_llm/docs/VRAM_BROKER.md` (correction commit, since the design docs assumed ~12.7 GB). **Also record the coder-only re-baseline** (≈12.39 GiB, captured this revision pass after the vision container was torn down, §1.3) — both numbers matter since residency is volatile (§2.3 DZ-23).
- **T1′.2** Add `Qwen3-VL-8B-Instruct` GGUF+mmproj as a SECOND, `ClassVLM`-**warm** (not resident) model behind the SAME `/v1/chat/completions` route, alias-selected (`model:"helix-vision-hq"` vs the existing default), config-injected model map (§CONST-046) — no new engine, no new port, a model-id/alias addition only.
- **T1′.3** Author the HelixQA Bank B golden fixture corpus (`data/vision_gt/*.png` + expected-facts JSON) — reuse the exact `red_stop_sign.png`-style pattern from `12_helixqa_testing.md` §Bank B.
- **T1′.4** Run Bank B against BOTH the 3B (always-warm) and 8B (on-demand) tiers; capture accuracy deltas as the FIRST real measured evidence closing the "3B vs 8B" question raised in §1.4.
- **Acceptance:** Bank B PASS on both tiers + captured accuracy-delta table under `docs/qa/<run-id>/vision_hardening/`; self-validated analyzer golden-bad fixture FAILs (proves the oracle is not itself a bluff).

#### P3-T2′ — Image generation runtime proof (the scaffold is DONE; this is the missing runtime step)
- **T2′.1** Re-measure `Budget().free` live (not the stale ~12.7 GB assumption) immediately before attempting the co-reside lane.
- **T2′.2** **Re-baselined 2026-07-08:** the vision co-resident container is currently STOPPED (torn down since this plan was drafted — §1.3), so live `free` is ≈12.39 GiB and the fallback tiers (SDXL/FLUX-schnell-Q4, ~7-9 GB) already admit without pausing anything. If a fresh `Budget().free` read at proof time nonetheless shows `free < needBytes+2GiB` (e.g. vision has been restarted since, or the attempt targets a flagship tier), request operator authorization (§11.4.122, since pausing a live capability is a user-visible-latency change) to temporarily stop whichever resident container(s) are blocking admission (`helixllm-coder` and/or a restarted vision container), OR schedule a full-burst window.
- **T2′.3** Run `docs/qa/phase4_*/harness/run_proof.sh admit-check` (per RESUME's documented entry point) then the full generate-and-verify cycle from `IMAGE_GEN_PROVIDER.md` §5 (CLIPScore matched/unrelated-control pair + golden-bad self-validation fixtures).
- **T2′.4** Capture the runtime signature to `docs/qa/<run-id>/image_gen/` per §11.4.83; register the guard into the §11.4.135 standing regression suite.
- **Acceptance:** the exact §5 signature from `IMAGE_GEN_PROVIDER.md` — real image, CLIPScore prompt-match, semantic-order margin, golden-good PASS + 4 golden-bad fixtures FAIL, paired §1.1 mutation on the analyzer itself.

#### P3-T3′ — Video generation runtime proof (mirrors P3-T2′)
- Same 4-subtask shape as P3-T2′ against `VIDEO_GEN_PROVIDER.md` §5's signature (ffprobe geometry + freeze/frame-advance liveness + per-frame CLIPScore + 5 golden-bad fixtures). Video's occupancy is MINUTES not seconds (§1.5 of that doc) — budget the operator-authorized window accordingly; this is an async-job route (§3 of that doc), never synchronous.
- **Acceptance:** the exact §5 signature from `VIDEO_GEN_PROVIDER.md`.

#### P3-T4′ — Vectorization (NET-NEW, no spike doc yet — author one before coding, per §11.4.6)
- **T4′.1** Author `submodules/helix_llm/docs/VECTORIZE_PROVIDER.md` (mirrors the `IMAGE_GEN_PROVIDER.md` template): engine choice (vtracer CLI-wrapped, CPU, `ClassEmbed`-style 0-GPU broker class — call it a new `ClassCPUTool` member or reuse `ClassEmbed`'s zero-reservation semantics) as the DEFAULT and the real path for natural-image/illustration/pixelized-graphic vectorization; StarVector-8B as an OPTIONAL, narrower smart/VLM-backed tier (co-resident-capable at ~12-16 GB, competes with the vision warm tier for the SAME budget — cross-check against P3-T1′'s `ClassVLM` warm slot, since both are VLM-shaped GPU consumers). **CAVEAT (§11.4.6/§11.4.99, confirmed via StarVector's own HF model card):** StarVector models explicitly do **NOT** work for natural images or illustrations — *"StarVector models will not work for natural images or illustrations, as they have not been trained on those images"* — they are scoped to icons, logotypes, technical diagrams, graphs, and charts only ([StarVector-8B model card](https://huggingface.co/starvector/starvector-8b-im2svg), accessed 2026-07-08). The operator's directive named "illustration" and "vectorizing pixelized graphics" as in-scope use cases — **these route to vtracer's raster-trace default, NOT StarVector.** StarVector's smart-mode tier should be routed ONLY for icon/logo/diagram/chart-class inputs.
- **T4′.2** API contract: `POST /vectorize {image, mode:"fast"|"smart"}` → `{svg, engine}` — `mode:"smart"` MUST fall back to the vtracer raster-trace path (not attempt StarVector) when the input is a natural image/illustration/photo rather than icon/logo/diagram/chart, per the caveat above.
- **T4′.3** Runtime signature: raster→SVG→re-rasterize→perceptual-similarity-to-source ≥ a calibrated floor (§11.4.107(13)); golden-bad fixture = a degenerate/empty SVG that must FAIL the re-rasterize check.
- **Acceptance:** signature above, captured under `docs/qa/<run-id>/vectorize/`.

#### P3-T5′ — Translation (DESIGNED in `03_translation.md`; author the P3-T5-level spike + land the CPU-first NLLB tier)
- **T5′.1** Author `submodules/helix_llm/docs/TRANSLATION_PROVIDER.md` (P3-T5 spike, mirroring `EMBEDDINGS_PROVIDER.md`'s CPU-tier-ships-first pattern): NLLB-200-3.3B int8 via CTranslate2 is **CPU-capable, `ClassTranslate`-warm OR zero-GPU**, so — like embeddings — it does NOT need to wait for a GPU burst window; recommend shipping it FIRST, same as embeddings did.
- **T5′.2** LibreTranslate-compatible `/translate` route + `/translate/document` (tokenize-protect-restore for markdown/code/HTML) + `/glossaries` CRUD.
- **T5′.3** QE gate: CometKiwi referenceless quality-estimation BEFORE any "professional quality" claim (closes AB-07/DZ-14 metric-gaming risk).
- **T5′.4** Runtime signature: back-translation metamorphic (X→Y→X′ similarity ≥ floor) + target-language/entity-preservation guard (catches a no-op identity "translator" — the exact mutation `12_helixqa_testing.md` Bank E specifies).
- **Acceptance:** Bank E (`helixllm_translation.yaml`) PASS + golden-bad no-op-translator fixture FAILs.

#### P3-T6′ — Embeddings/RAG completion (embeddings CPU-tier is PROVEN; this closes Qdrant + rerank + graph-fusion)
- **T6′.1** Stand up Qdrant (CPU, no GPU) via the containers submodule; wire named-vector hybrid (dense Qwen3-Embedding-4B/bge-small + sparse BGE-M3).
- **T6′.2** Wire bge-reranker-v2-m3 via TEI alongside the existing embeddings TEI container (same engine family, additive).
- **T6′.3** Fuse codegraph graph-expansion (callers/callees) into the retrieval pipeline per `08_codegraph_opendesign.md` §5.1 point 3 — AFTER the codegraph exclude-list fix lands (P5-T1′ below), never before (DZ-08 ordering rule).
- **Acceptance:** Bank F (`helixllm_embeddings_rag.yaml`) — Recall@K/MRR/nDCG on a labelled fixture set + RAGAS faithfulness with a negative case + a retrieval-label-shuffle golden-bad mutation.

#### P3-T7′ — HelixMemory (Zep/Graphiti + mem0) — RE-AUTHOR the missing spike doc first (closes DZ-24)
- **T7′.1** Author `submodules/helix_llm/docs/HELIXMEMORY_PROVIDER.md` from scratch (the RESUME-referenced `scratchpad/design_helixmemory_cognee.md` is not on disk — do not assume its content, re-derive from `04_embeddings_rag.md` §5's cited recommendation: Zep/Graphiti as the durable temporal-KG spine, mem0 as the lightweight extraction layer, exposed via MCP).
- **T7′.2** This is INDEPENDENT of cognee (which lives in `helix_agent`, is §11.4.174-blocked for pointer-bump reasons AND has its own upstream bug per §1.2/§2.6 DZ-25) — HelixMemory (Zep/mem0) is a SEPARATE, unblocked deliverable that does not wait on cognee's fate.
- **T7′.3** MCP exposure: `add`/`search`/`update` tools, project-agnostic namespacing (§11.4.28).
- **Acceptance:** injected-fact recall test (write a fact, retrieve it via a differently-worded query) + a temporal-validity test (a fact superseded by a later one is retrieved as the CURRENT value, not both) + faithfulness of retrieved-vs-injected content.

#### P3-T8′ — STT/OCR wiring-everywhere sweep (both engines PROVEN; this is the "wired everywhere it adds capability" completion)
- Per `07_stt_ocr_whisper_tesseract.md` §3 table: wire STT into (a) the media-validation pipeline's audio leg (§11.4.163), (b) `docs/qa/` raw-corpus transcription at release-prep (§11.4.128(3)), (c) an optional voice-input path for agent CLIs (Parakeet/Moonshine, deferred — not release-blocking); wire OCR into (a) the HelixQA pixel-oracle (§11.4.117, ALREADY the planned Bank-B/vision-testing substrate), (b) screenshot/error-dialog capture in workable-item tracking (nice-to-have, not release-blocking).
- **Acceptance:** Bank G + Bank H (already specced) PASS; the media-validation pipeline's audio leg demonstrably uses STT (not just a stub silence-check) on a real captured recording.

### Phase 4/5 — Protocols + core providers (fine-grained subtasks, extends `04_implementation_plan.md` P4/P5)

#### P4-T5′ — MCP gateway (re-verify the beta-SDK transport before committing)
- **T5′.1** Pull `github.com/modelcontextprotocol/go-sdk` latest release notes (post beta-SDK announcement, §1.1) — confirm Streamable-HTTP SERVER transport is present, not just client.
- **T5′.2** If present: build the gateway MCP-server role (HelixAgent/HelixLLM expose Helix tools) stateless-first per the RC's `server/discover` pattern.
- **T5′.3** If absent/UNCONFIRMED still: fall back to stdio-only MCP exposure for this cycle + `mark3labs/mcp-go` as the documented fallback, re-visit after 2026-07-28 finalization.
- **Acceptance:** a live `tools/list` round-trip via a real MCP client returns Helix's actual tool schemas (not a stub).

#### P4-T4′ — A2A (already designed to Revision 2; land the real wire, retire/replace the proprietary stub per an operator decision)
- Per `ACP_A2A_PROVIDER.md` Q6: the pre-existing `/api/v1/acp/{execute,broadcast,status}` canned-response routes are a KNOWN STUB (`cmd/api/main.go:211-215`,`:370-418`) — this is a §11.4.122 no-silent-removal case: **ask the operator** whether to (a) replace the stub in-place with the real A2A wire, or (b) add A2A alongside and deprecate the stub on a schedule. Do not unilaterally delete the stub routes.
- **Acceptance:** a live A2A Agent-Card fetch + a real task round-trip (not the canned-response path), captured evidence distinguishing it from the retired/replaced stub.

#### P5-T1′ — codegraph wiring fix (fully designed in `08_codegraph_opendesign.md` §4.1/§4.2/§4.3 — land it)
- **T1′.1** Rewrite `.mcp.json` codegraph entry to the portable bare-command form (§4.1 of that doc) — this is a small, well-specified, LOW-RISK fix; good candidate for the FIRST parallel stream dispatched off this plan.
- **T1′.2** Add the exclude patterns closing the 6.09 GB / 102k-file bloat (§4.2) — MUST land BEFORE P3-T6′.T6′.3 (codegraph→RAG fusion), per the existing DZ-08 ordering rule.
- **T1′.3** Reconcile the 1.1.1↔1.2.0 version drift; wire the weekly `codegraph_update_and_resync.sh` cadence (§11.4.80).
- **Acceptance:** `codegraph_validate.sh` cross-submodule probe PASSes + the exclude-then-restore paired §1.1 mutation FAILs when own-org content is wrongly excluded (§11.4.79(5)).

#### P5-T2′ — opendesign bring-up (fully designed in `08_codegraph_opendesign.md` §4.4/§5.2 — land it)
- **T2′.1** Install/start the opendesign daemon on `:7456`; health-gate before any use.
- **T2′.2** `.env` `HELIX_OD_BYOK_*` secrets (§11.4.10) — point BYOK at the local HelixLLM/Ollama endpoint for a fully-local design engine if feasible (avoids third-party egress, matches the offline posture the rest of this programme follows).
- **T2′.3** Seed the `helixcode-brand` project from `assets/` brand files.
- **T2′.4** Wire `internal/theme` (TUI) + a Fyne-theme adapter (desktop) to read the generated `tokens.css`.
- **Acceptance:** the §11.4.78-style unforgeable dual challenge (`od_list_projects` + `codegraph_status`) both resolve real facts in one script.

---

## 5. HelixQA extension — full test-bank + autonomous-session design (extends `12_helixqa_testing.md`)

`12_helixqa_testing.md` already specifies the complete design (Parts 1-6, banks A-I, the `ContentAssertingResolver`/conduit/`pkg/vision`/`pkg/recordingqa` substrate). This section adds the pieces that document identifies as still-needed plus the vision-specific emphasis the operator asked for ("ESPECIALLY vision").

### 5.1 What's already shippable RIGHT NOW (no blockers)

The `12_helixqa_testing.md` Risk-1 blocker — "HelixLLM per-capability API contract is UNCONFIRMED" — is **RESOLVED**: `submodules/helix_llm/docs/API_CONTRACT.md` now exists (confirmed on disk this session, §0.1). **First action of HelixQA-bank authoring is now unblocked**: read `API_CONTRACT.md`, fill every `# CONFIRM` placeholder in banks A-I with the real endpoint paths/schemas, THEN author the bank YAML files.

### 5.2 Vision-first bank authoring order (the operator's "ESPECIALLY vision" emphasis)

Sequenced by risk-descending priority (§11.4.132 — most-recently-worked, i.e. the JUST-PROVEN vision endpoint, goes first):

1. **Bank B (`helixllm_vision.yaml`) FIRST** — VIS-UNDERSTAND-001, VIS-COUNT-001, VIS-OCR-READ-001, VIS-SPATIAL-001, VIS-SELF-VALIDATE-001 (mandatory), VIS-LIVENESS-001. Ground-truth corpus (`data/vision_gt/*.png`) authored fresh against the LIVE 3B model's actual behaviour (never guessed) — run each candidate fixture against the live `:18439` endpoint during authoring to confirm the expected-answer baseline is genuinely achievable before committing it as a "must-match" fixture (prevents authoring an impossible fixture that would make the bank permanently RED for the wrong reason).
2. **Bank I (`helixllm_providers.yaml`)** next — cheap (mostly `requires_env` SKIP-gated), establishes the provider-coverage floor.
3. **Bank A (`helixllm_llm.yaml`)** — chat/tool-calling/streaming/concurrency, the coder-fleet's own correctness (already informally proven via RESUME's coder e2e evidence, but not yet a formal HelixQA bank).
4. **Bank G/H (STT/OCR)** — both engines are IMPLEMENTED+reviewed-GO, so these banks close the loop on already-shipped capabilities fastest.
5. **Banks C/D (image/video gen)** — gated on the P3-T2′/T3′ runtime-proof landing first (no bank can PASS against a capability whose runtime proof is still pending — author the bank YAML now, but its FIRST real run waits for P3-T2′/T3′).
6. **Bank E/F (translation/embeddings-RAG)** — gated on P3-T5′/T6′ landing.

### 5.3 Autonomous-session acceptance — additions to `12_helixqa_testing.md` Part 5

- **Vision-specific conduit evidence:** every Bank-B `challenge_verdict(PASS)` MUST be preceded by a `vision_call` conduit event (already a first-class event type per `pkg/conduit`) carrying model-id (3B vs 8B tier), latency, and token counts — so a live-tail of the autonomous session can distinguish "the 3B always-warm tier answered" from "the 8B on-demand tier answered," closing AB-15 (metadata-only vision-liveness bluff) by construction: the conduit event's presence + the `ContentAssertingResolver`'s content assertion together are the proof, never the bare `/v1/models` 200.
- **VRAM-aware bank scheduling:** because Banks C/D (image/video-gen) are `ClassImage`/`ClassVideo` **burst** classes that CANNOT run concurrently with each other (§11.4.119, `broker.go`'s `IsBurst()`), and this session's own finding (§2.6 DZ-23) shows they may ALSO now contend with the always-resident vision tier for headroom, the autonomous session's `--banks` invocation MUST serialize C and D relative to each other (already true per the broker) AND treat B (vision, resident) as a standing occupant when computing whether C/D can co-reside — i.e., the autonomous runner should call `Budget().free` before dispatching C or D and either proceed (co-reside fits) or honestly report a `202`/queued/SKIP-with-reason state rather than attempting an admission the broker will refuse.

### 5.4 Acceptance criteria for the HelixQA extension as a whole (unchanged from `12_helixqa_testing.md` Part 5, restated for completeness)

1. Fully self-driving end-to-end, re-runnable at `-count=3` with self-cleaning state (§11.4.98).
2. Live conduit trace: `challenge_start → llm_call/vision_call → evidence_captured(evidence_path) → challenge_verdict(PASS)`.
3. Every PASS verdict artefact exists, is non-empty, satisfies its content assertion.
4. Every self-validation case PASSes and its paired mutation FAILs (proof the oracles cannot bluff).
5. Curated evidence committed under `docs/qa/<run-id>/`, one subdir per capability, raw corpus git-ignored (§11.4.128).
6. Final verdict `PASS` with every capability GREEN or honestly SKIP/OPERATOR-BLOCKED with reasons — no capability silently absent (§11.4.118).

---

## 6. Summary of danger-zones enumerated (this document)

**5 NEW danger-zones/anti-bluff-surfaces** (DZ-23, DZ-24, DZ-25, AB-15, AB-16) added on top of the **56** already enumerated in `99_risk_bottleneck_analysis.md` (recounted 2026-07-08: DZ-01…DZ-22 = 22, AB-01…AB-14 = 14, V-01…V-20 = 20 → 22+14+20=56, correcting this document's earlier "41"/"22+14+~5" self-contradicting miscount) — **61 total enumerated across the programme as of 2026-07-08**.

---

## Sources verified 2026-07-08

- MCP 2026-07-28 RC (re-confirmed + beta SDKs): https://blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/ ; https://blog.modelcontextprotocol.io/posts/sdk-betas-2026-07-28/ ; https://stacktr.ee/blog/mcp-2026-spec-changes ; https://mcp.directory/blog/mcp-2026-07-28-release-candidate (all accessed 2026-07-08)
- cognee bug search (negative finding — exact trace not publicly indexed as of 2026-07-08): https://github.com/topoteretes/cognee/issues (accessed 2026-07-08); adjacent local-LLM interaction bug cited: https://github.com/topoteretes/cognee/issues/2119 (accessed 2026-07-08)
- Live host facts: `nvidia-smi`, `podman ps`, `curl localhost:18439/v1/models`, `ls submodules/helix_llm/docs/`, `wc -l submodules/helix_llm/internal/vrambroker/broker.go`, `find … design_helixmemory_cognee.md` (empty), `cat submodules/helix_agent/docs/COGNEE_BUG.md` — all captured this session, 2026-07-08.
- **Independent-review remediation pass (this revision, 2026-07-08 03:45:27 local / 2026-07-07T22:45:27Z UTC), §11.4.134/§11.4.150:** fresh `nvidia-smi --query-gpu=memory.total,memory.used,memory.free` + `podman ps` re-read (confirmed `helixllm_visiongen_visiongen_1` torn down since the plan was drafted; only `helixllm-coder` resident, 19436 MiB used / 12685 MiB free ≈ 12.39 GiB free) — re-baselines all §1.3/§2.3/§2.6/P3-T1′/P3-T2′ VRAM co-reside arithmetic; recount of `docs/research/07.2026/99_risk_analysis/99_risk_bottleneck_analysis.md` via `grep -oE 'DZ-[0-9]+|AB-[0-9]+|V-[0-9]+' | sort -u` → 22 DZ + 14 AB + 20 V = 56 prior items (corrects this document's earlier "41" miscount); `ls -la submodules/helix_llm/docs/` (confirmed `VRAM_BROKER.md` has no `.html`/`.pdf` sibling, unlike `API_CONTRACT.md`/`OPERATOR_GUIDE.md`); [StarVector-8B model card](https://huggingface.co/starvector/starvector-8b-im2svg) (accessed 2026-07-08 — confirms StarVector does not support natural-image/illustration vectorization).
- Prior Phase-R/Phase-P corpus (not re-cited individually; each sibling doc listed in the header carries its own full source list dated 2026-07-06/07):
  `docs/research/07.2026/00_master/{00_programme_master_plan,04_implementation_plan}.md`,
  `docs/research/07.2026/99_risk_analysis/99_risk_bottleneck_analysis.md`,
  `docs/research/07.2026/02_vision_generative/02_vision_generative.md`,
  `docs/research/07.2026/00_master/{IMAGE_GEN_PROVIDER,VIDEO_GEN_PROVIDER,EMBEDDINGS_PROVIDER,ACP_A2A_PROVIDER}.md`,
  `docs/research/07.2026/03_translation/03_translation.md`,
  `docs/research/07.2026/04_embeddings_rag/04_embeddings_rag.md`,
  `docs/research/07.2026/05_mcp_acp_protocols/05_mcp_acp_protocols.md`,
  `docs/research/07.2026/07_stt_ocr_whisper_tesseract/07_stt_ocr_whisper_tesseract.md`,
  `docs/research/07.2026/08_codegraph_opendesign/08_codegraph_opendesign.md`,
  `docs/research/07.2026/12_helixqa_testing/12_helixqa_testing.md`,
  `docs/research/07.2026/00_master/RESUME.md`.
