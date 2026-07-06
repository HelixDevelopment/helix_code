# RESUME — HelixLLM Full-Extension Programme (session-resumption file, §11.4.131)

> **Fresh session? Paste the SHORT prompt below.** This file is the single out-of-the-box
> entry point. Always current; re-read on every new session. Track `(T1/main)`.

## SHORT resumption prompt (paste into a fresh session)

> Read `docs/research/07.2026/00_master/RESUME.md` + `.superpowers/sdd/progress.md` (the live
> SDD ledger), run `git fetch --all`, and continue the HelixLLM full-extension programme.
> Phase 0 (GPU) + Phase 1 (fleet + fixes) + Phase 2 (HelixAgent→HelixLLM e2e) are PROVEN; the
> LLMsVerifier capability-verification chain (C1→C2→C4→C5→C3) is fully landed and under a final
> combined independent review. Next: land the review to GO, then the release-prep pass
> (pointer bumps + prefixed release tag §11.4.151 via merge-onto-latest §11.4.113, NO force-push).
> Honor anti-bluff §11.4, subagent-driven §11.4.70, `(T1/<branch>)` labels §11.4.182, one canonical
> branch `feature/helixllm-full-extension` §11.4.181, §11.4.174 shared-host process/tree ownership.

## Current phase + immediate next action

- **Phase 0 (GPU foundation):** ✅ COMPLETE — rootless CDI passthrough + sm_120 build + real 30B inference PROVEN.
- **Phase 1 (fleet + fixes):** ✅ 30B coder live; Containerfile + claude_toolkit fixes landed, all re-reviewed **GO**.
- **Phase 2 (HelixAgent→HelixLLM e2e):** ✅ **PROVEN + review GO** — real generate + Postgres/Redis persistence (cognee/vector honest SKIP, OQ2).
- **LLMsVerifier chain:** ✅ C1 C2 C4 C5 C3 + advisories all landed, combined review **GO** — release-ready.
- **Phase-3:** ✅ embeddings IMPL **PROVEN** (55bdf9b6, real bge-small TEI, cos margin 0.3578) · ✅ VRAM broker CORE **GO** (a12df57c — unblocks GPU tiers) · ✅ designs done: embeddings (cf26b813), translation (c9ac8683), provider-coverage (1e6f3347, 0 new wire adapters).
- **Immediate next (fresh session):** (a) the pending §11.4.142 reviews are DONE for broker; run reviews of the 3 design docs + embeddings if desired; (b) GPU tiers now unblocked — vision (VLM)/image/video gen via the broker; (c) provider config-rollout (13 providers, config-only); (d) translation IMPL (design c9ac8683); (e) cognee P-OQ2-A wire; (f) small follow-ups: OQ1 doc correction in PHASE2_BLOCKERS_INVESTIGATION.md, P3-EMB-1 golden-good dim-aware fixture, detector.go:61; (g) release-prep + prefixed tag when scope-complete.
- **Terminal goal (this scope):** a fully-validated, prefixed release tag (§11.4.151) published across
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
| Constitution HEAD followed | `0882b9e` (through §11.4.182) |
| Host | ALT Workstation 11.1; RTX 5090 32 GB; driver 570.169; CUDA 12.8; podman 5.7.1 rootless; 64 cores / 251 GiB |
| Canonical branch (§11.4.181) | `feature/helixllm-full-extension` — ACTIVE (no upstream tracking configured yet) |
| Router image (built+proven) | `localhost/helixllm/llamacpp-router:cuda12.8-sm120` — latest llama.cpp, sm_120, OpenSSL/curl (`-hf` HTTPS proven), ships `rpc-server` |
| Release prefix (§11.4.151) | `HELIX_RELEASE_PREFIX` else `helix_code` |
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
