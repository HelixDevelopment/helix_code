# RESUME — HelixLLM Full-Extension Programme (session-resumption file, §11.4.131)

> **Fresh session? Paste the SHORT prompt below.** This file is the single out-of-the-box
> entry point. Always current; re-read on every new session. Track `(T1/main)`.

## SHORT resumption prompt (paste into a fresh session)

> Read `docs/research/07.2026/00_master/RESUME.md` + `.superpowers/sdd/progress.md` (the live
> SDD ledger), run `git fetch --all`, and continue the HelixLLM full-extension programme.
> The branch `feature/helixllm-full-extension` (HEAD `4d58464c`, 34 commits over merge-base
> `31cde9a1`) is **RELEASE-READY**: Phase 0 (GPU) + Phase 1 (fleet) + Phase 2 (HelixAgent→HelixLLM
> e2e) + Phase 3 (embeddings/vision/translation-NLLB/Whisper/Tesseract/RAG/A2A/network-provider/
> VRAM-broker/LLMsVerifier chain) all PROVEN + individually review-GO; the whole-branch SDD
> end-gate returned **GO across all 3 independent lenses** (anti-bluff, security/§11.4.174,
> integration/release-readiness). §11.4.40 pre-tag sweep RUN (`qa-results/pretag_verify_*.log`):
> branch is regression-clean — QA-evidence 0 warnings; the only sweep FAILs are PRE-EXISTING
> governance debt on submodules my branch never touched (G1 streaming/watcher inheritance pointers
> + doc_processor/llm_orchestrator/llm_provider/vision_engine anti-bluff anchors, from refactor
> `1422f7da`, not this branch) + third-party vendored Continue.dev `.skip()` (vendor-exempt §11.4.29).
> **RELEASE PUBLISHED ✅ (operator-authorized 2026-07-07, completed 2026-07-08).** Tag
> `helix-code-1.0.0-dev-0.0.1` (§11.4.151 prefix `helix-code`, kebab; supersedes old `helixcode-vN`)
> created + pushed across main-repo HEAD `10c40c85` (tag object `b7e78c3a`, on github Helix-CLI +
> gitlab HelixCode) AND all 7 owned submodule HEADs at their published SHAs — helix_llm(071c1223),
> doc_processor(b918111), llm_provider(4db6c49), llm_orchestrator(ee229a7, CONSTITUTION
> merge-onto-latest), llms_verifier(c696c5db), vision_engine(a97df79), helix_agent(cfa94f2f, foreign
> go.mod/.qa_bak never staged §11.4.174). Feature branch pushed (NO trunk merge, §11.4.167). Pre-tag
> §11.4.40 sweep GREEN on owned scope: governance-cascade 0 failures, anti-bluff anchor present in all
> 18 files across 13 repos; residual no-silent-skips confined to vendored Continue.dev (§11.4.29 exempt).
> **NEXT (post-tag scope, endless loop §11.4.126 continues)** — operator UNLOCKED (all 3): GPU gen
> (FLUX-NVFP4/WAN/LTX — quant footprints RESOLVED; GPU-gen SCAFFOLD-COMPLETE + reviewed GO: image-gen
> helix_llm `0f07559` port 18442 + video-gen `9145505` port 18443, both local-no-push, broker-fail-closed,
> RED-first self-validated analyzers; RUNTIME PROOF PENDING operator coder-pause §11.4.122 — run each
> `docs/qa/phase4_*/harness/run_proof.sh admit-check` then boot+generate in an authorized window),
> broad-provider live proofs (needs API keys §11.4.10), HelixMemory/cognee wire (design DONE
> `scratchpad/design_helixmemory_cognee.md` — P-OQ2-A Postgres-row persistence is container-independent +
> lands first; IMPL BLOCKED by §11.4.174 until the concurrent QA track's foreign helix_agent go.mod/.qa_bak clears).
> Honor anti-bluff §11.4, subagent-driven §11.4.70, `(T1/<branch>)` labels §11.4.182, one canonical
> branch §11.4.181, no-force-push §11.4.113, §11.4.174 shared-host ownership (helix_agent go.mod/.qa_bak
> FOREIGN — never stage), coder container live at 18434 (never restart §11.4.122).

## Current phase + immediate next action

- **BRANCH STATUS: RELEASE-READY.** HEAD `4d58464c`, 34 commits over merge-base `31cde9a1`. Whole-branch
  SDD end-gate returned **GO across all 3 independent lenses** (anti-bluff · security/§11.4.174 · integration/
  release-readiness — the last NO-GO'd on 3 missing sibling exports, fixed in `4d58464c`, re-reviewed GO).
- **Phase 0 (GPU):** ✅ rootless CDI passthrough + sm_120 build + real 30B inference PROVEN.
- **Phase 1 (fleet):** ✅ 30B coder live; Containerfile + claude_toolkit fixes, all re-reviewed **GO**.
- **Phase 2 (HelixAgent→HelixLLM e2e):** ✅ **PROVEN + GO** — real generate + Postgres/Redis persistence.
- **LLMsVerifier chain:** ✅ C1 C2 C4 C5 C3 + advisories landed, combined review **GO**.
- **Phase-3:** ✅ embeddings IMPL PROVEN (55bdf9b6) · VRAM broker CORE GO (a12df57c) · translation NLLB-CT2
  (18436) · Whisper STT (18437) · Tesseract OCR (18438) · RAG-TEI (18440) · ACP→A2A (18441) · network-provider
  LAN/VPN — all landed + individually review-GO, QA siblings complete (9/9 RESULTS.md have .html+.pdf).
- **§11.4.40 PRE-TAG SWEEP RUN** (`qa-results/pretag_verify_20260707_222458.log`, exit-captured):
  QA-evidence §11.4.83 = 0 warnings (PASS). The 3 sweep FAILs are ALL PRE-EXISTING, regression-isolated
  (`git diff 31cde9a1..HEAD` on those paths = EMPTY): G1 (streaming/watcher inheritance pointers) +
  doc_processor/llm_orchestrator/llm_provider/vision_engine anti-bluff anchors (added by refactor `1422f7da`,
  not this branch) + third-party vendored Continue.dev `.skip()` (§11.4.29 vendor-exempt). None block THIS
  branch's tag; all are honest-tracked cross-cutting governance debt (own separate work stream).
- **Governance:** constitution `5074d606` (through §11.4.183). Quant-footprint UNCONFIRMED → RESOLVED
  (a93140b1): ≤10.4 GiB co-resident FLUX exists but MUST be NVFP4 on the 5090 (Blackwell cc10.x → fp4, not int4).
- **RELEASE PUBLISHED ✅ (2026-07-08).** `helix-code-1.0.0-dev-0.0.1` on main-repo HEAD `10c40c85`
  (tag object `b7e78c3a`, github Helix-CLI + gitlab HelixCode) + all 7 owned submodule HEADs. Main
  feature branch pushed (github+gitlab, NO trunk merge §11.4.167). Pre-tag §11.4.40 sweep GREEN on
  owned scope (governance-cascade 0 failures; anti-bluff anchor in all 18 files/13 repos).
- **IMMEDIATE NEXT = next-tag scope (endless loop §11.4.126 continues past release).** Non-blocked:
  vision local-serving impl (Qwen2.5-VL-7B co-resident, design `scratchpad/design_vision_local_serving.md`,
  §11.4.167 own feature work-stream), Stream-C `install_helix_path.sh` review (§11.4.142 + verify against
  §11.4.177 no-project-tooling-on-shared-PATH). OPERATOR-GATED (honest blocks, not autonomously takeable):
  GPU gen runtime proofs (need coder-pause §11.4.122), provider live proofs (need API keys §11.4.10).
  Keep ≥3 parallel streams on non-blocked work per §11.4.103.
- **Terminal goal (§11.4.126):** a fully-validated, prefixed release tag (§11.4.151) published across
  main + all owned submodules; local HelixLLM on the RTX 5090 exposed via HelixAgent to HelixCode/CLI agents.

## LIVE SERVER (operator-testable, running now)

**Qwen3-Coder-30B-A3B serving on `http://localhost:18434/v1`** (OpenAI-compatible) — container
`helixllm-coder` (`podman ps`), `--network=host`, image `localhost/helixllm/llamacpp-router:cuda12.8-sm120`,
8 parallel slots, 24k ctx, q8_0 KV, ~19.4 GB VRAM. PROVEN: single-stream ~220 tok/s (coder_live_e2e log;
RESUME's older ~322 tok/s was a different --jinja/q8_0-KV run) + 8 concurrent agents @85–96 tok/s; real
coding output (`is_palindrome`, `func Add`). Restart: `podman start helixllm-coder`.

> **ENDPOINT GOTCHA (load-bearing, proven in Phase-2 `docs/qa/phase2_e2e_20260706/12_endpoint_finding.txt`):**
> raw `curl` uses `http://localhost:18434/v1/chat/completions`. A client/SDK that APPENDS
> `/v1/chat/completions` (incl. HelixAgent via `HELIX_LLM_LOCAL_OPENAI_ENDPOINT`) MUST use the BASE
> `http://localhost:18434` (NO `/v1`) — else `.../v1` → double `/v1/v1` → **HTTP 404**.
> (This SUPERSEDES the `:18434/v1` value in `10_llmsverifier_helixagent/PHASE2_BLOCKERS_INVESTIGATION.md`
> OQ1 — that doc line is pending a correction commit in the release-gate sweep.)
> NOTE: latest llama.cpp `-fa` takes a value (`-fa on`).

## Live-state anchors (facts, §11.4.6)

| Anchor | Value |
|--------|-------|
| Constitution HEAD followed | `5074d606` (through §11.4.183 — maximal multi-agent + full-constitution + zero-bluff, cascaded to CLAUDE/AGENTS/QWEN/GEMINI; pointer bumped commit 0a469883) |
| Host | ALT Workstation 11.1; RTX 5090 32 GB; driver 570.169; CUDA 12.8; podman 5.7.1 rootless; 64 cores / 251 GiB |
| Canonical branch (§11.4.181) | `feature/helixllm-full-extension` — ACTIVE (no upstream tracking configured yet) |
| Router image (built+proven) | `localhost/helixllm/llamacpp-router:cuda12.8-sm120` — latest llama.cpp, sm_120, OpenSSL/curl (`-hf` HTTPS proven), ships `rpc-server` |
| Release prefix (§11.4.151) | `helix-code` (operator decision 2026-07-07; `.env` `HELIX_RELEASE_PREFIX=helix-code`; form `helix-code-1.0.0-dev-0.0.1`, supersedes old `helixcode-vN`) |
| Shared-host caution (§11.4.174) | helix_agent checkout carries a CONCURRENT QA/dep track's uncommitted go.mod/go.sum + `.qa_bak` — NOT ours; do not sweep on pointer-bump |

## Done so far this session (real, evidence-backed commits)

- helix_llm `13d2d27` Containerfile: hard-fail `ggml-rpc-server` copy (§11.4.122) + OpenSSL/curl — re-review **GO**; `d8b3fa2` phase-1 QA evidence (`-hf` 469MB HTTPS download + live coder e2e); `3f85e3d5` OPERATOR_GUIDE.
- claude_toolkit `ef77b19` loud-fail resolution + discriminating test; `9d12347` C1 regression guard (37/0, §1.1 mutation-proven) — **REVIEW-3 CLEAN**.
- llms_verifier `09f9533c` C4 · `28e6625a` C5 · `ad18e91f` C3 · `d1f04e5c` advisories (detector gate + sentinel oracle) — **chain + advisories complete, combined review GO**.
- helix_llm `a12df57c` **VRAM broker CORE** (Budget/admission/single-owner, review GO — unblocks GPU tiers).
- helix_agent `17f08ba9` Phase-2 live tests (provider e2e + redis) — pointer NOT yet bumped (§11.4.174).
- helix_code `cf26b813` embeddings design · `9bf4c3da` Phase-2 stack design · `da0fabae` blockers investigation · `5223d10d` C3 handoff · `278df582` Phase-2 e2e evidence · `c9ac8683` translation design · `1e6f3347` provider-coverage design · `55bdf9b6` **embeddings IMPL proven** (bge-small TEI, cos margin 0.3578).

## Next actions (in order)

1. Land the combined **C4+C5+C3 review** to GO (adjudicate `detector.go:61` no-probe self-cert).
2. Correct `PHASE2_BLOCKERS_INVESTIGATION.md` OQ1 verdict → base `http://localhost:18434`.
3. Release-prep: bump main-repo submodule pointers (helix_llm, claude_toolkit, llms_verifier; helix_agent
   carefully per §11.4.174) → §11.4.40 full-suite pre-tag sweep → prefixed tag (§11.4.151) → publish via
   merge-onto-latest (§11.4.113, NO force-push §2.1 multi-upstream).
4. Post-release follow-ups (tracked): P-OQ2-A wire `cognee_memory_repository` + P-OQ2-B re-verify cognee 1.2.2 bug;
   `detector.go:61` no-probe self-cert; VRAM broker IMPL (design 5102607 done) to unlock GPU vision/image/video;
   Phase-3 CPU embeddings + translation NMT impl (designs done).

## Binding constraints (do not violate)

Anti-bluff §11.4 (real captured proof, no metadata PASS) · runtime-signature = done §11.4.108 ·
no-force-push merge-onto-latest §11.4.113 · rootless podman via containers submodule §11.4.76/§11.4.161 ·
no CI §11.4.156 · no silent removal §11.4.122 / investigate-before-remove §11.4.124 · deep multi-angle
research per change §11.4.150 · independent review to GO §11.4.125/§11.4.142/§11.4.134 · one canonical
branch §11.4.181 · `(T<N>/<branch>)` labels §11.4.182 · shared-host ownership §11.4.174.
