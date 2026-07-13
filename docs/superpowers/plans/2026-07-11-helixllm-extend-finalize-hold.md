# HelixLLM Full-Extension — EXTEND → FINALIZE → HOLD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan is a **roadmap over already-landed work** — it does NOT re-derive detail already written in the three GO domain plans (`01_local_models_serving/IMPLEMENTATION_PLAN_v2.md`, `02_vision_generative/CAPABILITIES_MASTER_PLAN_v2.md`, `06_providers_coverage/EXPANSION_PLAN_v2.md`) or the consolidated `00_master/MASTER_IMPLEMENTATION_PLAN.md`; it cites them (§11.4.74 anti-duplication). Read the cited source before implementing a task.

**Goal:** Take the released HelixLLM full-extension (`feature/helixllm-full-extension`, tag `helix-code-1.1.0-dev-0.0.1`, pushed to github+gitlab) through one more cycle: **(A) EXTEND** the remaining capabilities, **(B) FINALIZE** (review + cleanup + refresh), **(C) HOLD** at a clean release terminal — per operator directive 2026-07-11 ("Do 3, then 1, and finally 1").

**Architecture:** Local-first multi-instance LLM serving on one RTX 5090 (32 GB) via rootless podman + the `containers` submodule (§11.4.76/§11.4.161). Single always-on coder (Qwen3-Coder-30B-A3B @ `:18434`, ~19.4 GiB) is Lane A; every other capability is admission-gated against live free VRAM by the VRAM broker (§11.4.119 single-owner burst). LLMsVerifier is the single source of truth for model/provider/capability metadata (CONST-036/040). Every capability ships with a self-validated golden-good/golden-bad analyzer + paired §1.1 mutation (§11.4.107/.137/.163).

**Tech Stack:** Go 1.26 (inner `dev.helix.code`), llama.cpp (cuda12.8-sm120), podman rootless + CDI GPU passthrough, TEI (embeddings), faster-whisper CT2, Tesseract, FLUX/WAN/LTX (generative), go-sdk MCP v1.6.1 (Streamable-HTTP), a2a-go SDK, PostgreSQL/pgvector + Redis, pandoc+weasyprint+mmdc (doc export).

---

## Global Constraints

*Every task's requirements implicitly include this section. Copied from the constitution + reconciled ground truth.*

- **Anti-bluff (§11.4/§11.4.5/§11.4.6/§11.4.107/§11.4.123):** every PASS carries captured PHYSICAL evidence from a real run; metadata-only / config-only / absence-of-error / grep-without-runtime PASS is a violation. Unclear-how-to-test ⇒ deep web research first (§11.4.150), never a metadata pass. Honest SKIP-with-reason (§11.4.3), never a fake PASS.
- **CODER IS DOWN (2026-07-11 FACT):** `helixllm-coder` will not boot — CDI pins stale `/dev/dri/card0`; host has `card1`+`renderD128` (§11.4.111). No CDI spec generated (`nvidia-ctk cdi list`=0; `/etc/cdi` absent). **Task A0 (coder restore) gates every live-coder task.** Until A0 lands, no task may claim a live-`:18434` proof.
- **§11.4.174 foreign-work exclusion:** NEVER touch `submodules/helix_agent` go.mod/go.sum/.qa_bak (foreign QA-track dirt), and NEVER touch `/mnt/track1` or its 3 adb device serials (`66ff9c4f…`, `93f4f1fd…`, `998fd36…` — the ATMOSphere T1 project's live device recording).
- **No force-push (§11.4.113):** every push is ff-only merge-onto-latest-main; outward push is operator-authorized (R35 decision-2: github+gitlab; GitFlic+GitVerse not yet configured).
- **No silent removal (§11.4.122):** the A2A stub, any capability, or any component may not be removed without an explicit operator keep/remove decision via §11.4.66.
- **Coder-never-casually-restart (D8/§11.4.122):** all Lane-A-affecting changes batch into ONE operator-authorized coder-pause window.
- **VRAM volatility (DZ-23):** re-read `nvidia-smi`/`Budget().free` live immediately before EVERY admission decision; never trust a cached number.
- **Every change reviewed (§11.4.142) + iterate-to-GO (§11.4.134):** independent reviewer distinct from author; loop until zero findings + zero warnings.
- **Real-content media/journey tests (§11.4.136/§11.4.143), window-scoped project-prefixed recordings (§11.4.154/§11.4.155/§11.4.159), device-independent host-rendered UI proof (§11.4.170)** where applicable.
- **Naming/version (§11.4.29/§11.4.151):** lowercase snake_case; release tags prefixed `helix-code-` (from `HELIX_RELEASE_PREFIX` else lowercased root dir).
- **Manual QA-team final confirmation (§11.4.185)** is the terminal sufficiency gate before any scope is "fully done."

---

## Reconciled Baseline — LANDED + PROVEN (do NOT re-plan — §11.4.74)

Per `00_master/MASTER_IMPLEMENTATION_PLAN.md` §1 + git log through HEAD `536ac9c6`, ALREADY landed, reviewed-GO, pushed, and tagged (`helix-code-1.0.0-dev-0.0.1` @ `10c40c85`, `helix-code-1.1.0-dev-0.0.1` @ `dfa6a2c2`):

- GPU foundation (rootless CDI sm_120 + real 30B inference); coder fleet @ `:18434`; HelixAgent→HelixLLM e2e (Postgres/Redis persistence); LLMsVerifier C1→C5 chain (fail-closed resolver).
- Capabilities PROVEN: embeddings (bge-small/TEI), vision-VLM (Qwen2.5-VL-3B), translation-NLLB (`:18436`), Whisper STT (`:18437`), Tesseract OCR (`:18438`), RAG-TEI (`:18440`), ACP→A2A (`:18441`), network-provider (LAN/VPN), VRAM broker CORE (`ClassCoder/VLM/Image/Video/Translate/Embed/Agent`), Lane-B Mistral-Nemo-12B (163 tok/s co-resident), MCP-gateway (Bearer + coder-proxy), dual-wire facade (OpenAI+Anthropic) with auth.
- **Jul-9 additions (already committed, under review by R41 stream a98894382fe3bbcf8):** FLUX.1-schnell image-gen proof (`d8593b63`), WAN2.2 TI2V-5B video-gen proof (`f2c71d07`), HelixQA concurrency/chaos/DDoS/memory/benchmark banks (`dfa6a2c2`/`536ac9c6`/`54e91e8a`), provider live-proofs Groq/Mistral/Codestral/Cohere/Cerebras (`4b07bb6f`/`f8c38181`), i18n + security-test §11.4.120 reconciliations.

**Implication:** the image/video HF_TOKEN gate and much of Phase-1 are already cleared. What remains is (A0) coder restore, a thin set of coder-independent-now items, coder-up hardening, operator-gated bursts/keys, and the finalize/hold convergence.

---

## Gating Map (the honest reality — read before picking a task)

| Bucket | What | Blocked by |
|---|---|---|
| **Now, coder-independent** | R41 fleet (delta review, doc integrity, governance, external provider proofs), codegraph reindex verify, provider §11.4.99 currency audit | nothing — dispatchable |
| **A0 — coder restore** | regenerate CDI spec + recreate container device binding | host GPU + root/sudo → **operator** (§11.4.133) |
| **Coder-up, non-operator-gated** | Lane-B production wiring, fast-lane image/video runtime proof (fits free VRAM), vision 8B hardening, HelixQA live banks vs model, MCP-gateway live e2e, dual-wire live round-trip | A0 |
| **Operator-gated** | flagship image/video (coder-pause), Lane-A `--kv-unified`/batch tuning (coder-pause), broad provider proofs (API keys §11.4.10), cognee wire (§11.4.174), A2A stub retire (§11.4.122), merge→main + prod tag (§11.4.167), GitFlic/GitVerse remotes | **operator** |

---

# PHASE A — EXTEND (operator option 3)

Build the remaining capability surface. Ordered by dependency; each task cites its source-plan detail.

## Task A0: Restore the live coder (CDI passthrough fix) — **PREREQUISITE for all live-coder tasks**

**Root cause (diagnosed 2026-07-11, §11.4.111):** the container's CDI device list references `/dev/dri/card0`; the DRM node re-enumerated as `card1` on the Jul-9 reboot, and no CDI spec is currently generated.

**Files/host:** `/etc/cdi/nvidia.yaml` (to regenerate), the `helixllm-coder` container device binding.

- [ ] **A0.1** Capture baseline: `ls -la /dev/dri`, `ls -la /dev/nvidia*`, `nvidia-ctk cdi list`, `podman inspect helixllm-coder --format '{{json .HostConfig.Devices}}{{json .Config.Annotations}}'` → evidence file.
- [ ] **A0.2** Regenerate the CDI spec so it resolves by GPU identity, not stale index (needs root): `sudo nvidia-ctk cdi generate --device-name-strategy=index --output=/etc/cdi/nvidia.yaml` then `nvidia-ctk cdi list` (expect ≥1 device).
- [ ] **A0.3** Recreate/refresh the coder's device binding to `nvidia.com/gpu=all` (CDI name, not `/dev/dri/card0` index), or recreate the container from its Containerfile with the regenerated spec. **Reversible** — preserve the old container config first (§9.2).
- [ ] **A0.4** `podman start helixllm-coder`; poll `:18434/v1/models` until the 30B model loads; `nvidia-smi` shows ~19.4 GiB resident.
- [ ] **A0.5 Acceptance (§11.4.5):** `POST :18434/v1/chat/completions` with a fresh nonce returns real Go output echoing the nonce, real tok/s + usage; capture to `docs/qa/`. Container up, GPU resident.
- [ ] **A0.6** Add a permanent §11.4.111/§11.4.135 guard: a boot script/preflight that resolves the DRM node by identity (not `card0`) and regenerates CDI if absent, so the next reboot doesn't re-break this.

**Gating:** A0.2/A0.3 need root (host GPU, §11.4.133) → **operator-run** (one-liner surfaced separately) OR conductor if passwordless sudo is available and reversible. **Acceptance evidence is mandatory before any A2/A3 task claims a live proof.**

## Task A1: Coder-independent hardening (dispatchable NOW — real evidence, no coder)

These are in flight or immediately dispatchable; none needs the coder.

- [ ] **A1.1 Delta independent review** (§11.4.142) of `10c40c85..536ac9c6` — anti-bluff spot-check the Jul-9 headline claims against committed evidence. *(R41 stream `a98894382fe3bbcf8` — in flight.)* Acceptance: verdict GO/GO-WITH-FINDINGS + per-claim REAL/BLUFF table.
- [ ] **A1.2 Exported-doc integrity** (§11.4.168) — pdftotext leak-check + regen of committed PDFs. *(R41 stream `aee493fe18836074a` — in flight.)* Acceptance: 0 raw-Mermaid leaks, images present, mtime parity.
- [ ] **A1.3 Governance sweep + 5-carrier lockstep** (§11.4.32/§11.4.157). *(R41 stream `adfa11ec20cf72570` — in flight.)* Acceptance: sweep pass/fail enumerated, carriers in lockstep, debt closed-or-honest-tracked.
- [ ] **A1.4 External provider live-proof re-run** (§11.4.99) — fresh nonce-challenged proofs for key-present providers. *(R41 stream `a343804597c06cab9` — in flight.)* Acceptance: per-provider LIVE/SKIP table with redacted raw output, retired-model currency notes.
- [ ] **A1.5 codegraph reindex verify (HXC-041)** — CPU/disk, no coder. Confirm own-org submodules resolve, third-party purged, `codegraph.json` is the effective config. Acceptance: `codegraph_validate.sh` cross-submodule probe PASS + exclude-mutation FAIL-then-restore.
- [ ] **A1.6 Provider config §11.4.99 currency audit** — static audit of the 13 config rows + 10 CONST-039 providers for retired/renamed default models (e.g. DeepSeek-V4 mapping); flag without inventing endpoints (§11.4.6). Acceptance: audit doc listing each row's advertised-vs-configured model.

## Task A2: Coder-up capability hardening (gated on A0 only — non-operator)

Cite `01_.../IMPLEMENTATION_PLAN_v2.md` + `02_.../CAPABILITIES_MASTER_PLAN_v2.md` for full task detail. Each re-reads `Budget().free` live before admission (DZ-23).

- [ ] **A2.1 Lane-B production wiring** — promote Mistral-Nemo-12B from benchmark spike to a broker-admission-gated production instance via `ClassAgent` (serving-plan Task 2.2). Acceptance: co-resident with coder, benchmark p50/p95/p99 + tool-calling correctness, teardown clean, coder untouched.
- [ ] **A2.2 Fast-lane image/video runtime proof** (fits free VRAM, no pause — capabilities-plan P3-T2′/T3′) — FLUX-schnell-Q4 / SDXL image + LTX-Video/WAN-TI2V-5B video; real generate → CLIPScore + freeze-liveness (§11.4.107) + golden-bad. Acceptance: real artifact + analyzer verdict, registered as §11.4.135 guard.
- [ ] **A2.3 HelixQA live banks vs model** — run the vision + capability banks against the restored live services; self-validated analyzers + §1.1 mutation. Acceptance: real Dispatcher PASS/SKIP with mutation proofs.
- [ ] **A2.4 MCP-gateway + dual-wire live e2e** — real go-sdk MCP client round-trip (401 → tools/list → generate) + facade `/v1/chat/completions` + `/v1/messages` live nonce round-trip through the restored coder. Acceptance: captured real responses, auth fail-closed proven.
- [ ] **A2.5 Vision 8B warm-tier hardening** (capabilities-plan P3-T1′) — re-measure 8B VRAM; admit only if fits ceiling. Acceptance: real multimodal completion or honest SKIP if over-budget.

## Task A3: Operator-gated extend (surface, do not force)

- [ ] **A3.1** Flagship image/video (FLUX-fp8 / WAN-A14B) — coder-pause burst window (§5 decision 1).
- [ ] **A3.2** Lane-A `--kv-unified` + batch/parallel tuning + GGUF re-pull — single coder-pause window (§5 decision 2).
- [ ] **A3.3** Broad provider coverage completion — needs API keys (§5 decision 3, §11.4.10).
- [ ] **A3.4** cognee wire / HelixMemory-only path (§5 decision 5, §11.4.174-blocked).
- [ ] **A3.5** A2A stub retirement — §11.4.122 keep/replace operator decision (§5 decision 9).

---

# PHASE B — FINALIZE (operator option 1, first pass)

Converge everything Phase A produced into a clean, reviewed, taggable state.

## Task B1: Close the R41 review loop
- [ ] **B1.1** Ingest each R41 stream's report; for any Critical/Important finding, dispatch a fix subagent (§11.4.134 iterate-to-GO), re-review until zero findings + zero warnings.
- [ ] **B1.2** Fold Minor findings into the whole-branch review triage list.

## Task B2: Whole-branch final review (SDD end-gate)
- [ ] **B2.1** `git merge-base main HEAD`; generate the review package for `<merge-base>..HEAD`.
- [ ] **B2.2** Dispatch an independent whole-branch reviewer (most-capable model) — 3 lenses: anti-bluff, security (§11.4.174 no foreign sweep, no unauth routes, no secrets), integration/release-readiness. Iterate to GO.
- [ ] **B2.3 Acceptance:** VERDICT GO, 0 push-blockers; captured review evidence.

## Task B3: Working-tree hygiene
- [ ] **B3.1** Confirm (from R41 audit) that all genuine QA evidence is committed; the 35 `phase1_fullhttp_e2e` loop dirs + superseded coder-bench/provider re-runs are redundant residue whose canonical evidence already landed.
- [ ] **B3.2** With operator sign-off (§11.4.124/§9.2 — do not `git clean` unreviewed), remove the confirmed-redundant residue OR relocate to a gitignored scratch archive. Do NOT delete anything with unique uncommitted content.
- [ ] **B3.3** Verify tree quiescence (§11.4.84): no mutation markers, only intended files, foreign `helix_agent` untouched.

## Task B4: Submodule pointer bumps (§11.4.98 re-runnability)
- [ ] **B4.1** For each owned submodule advanced past its gitlink (helix_llm, llms_verifier, helix_qa — NOT foreign helix_agent), bump the root gitlink to the reviewed HEAD in one commit. §11.4.174-careful: gitlink-only, never sweep foreign uncommitted work.
- [ ] **B4.2 Acceptance:** `git submodule status` shows no owned submodule lagging its reviewed HEAD.

## Task B5: RESUME + ledger refresh (§11.4.131)
- [ ] **B5.1** Refresh `docs/research/07.2026/00_master/RESUME.{md,html,pdf}` to true HEAD/state (coder-boot status, what's proven, what's operator-gated) with §11.4.65 siblings.
- [ ] **B5.2** Append the final R41 outcome to the SDD ledger.

---

# PHASE C — HOLD / RELEASE TERMINAL (operator option 1, second pass)

The §11.4.126 release-scope terminal. Every item here is operator-gated by design.

## Task C1: Pre-tag validation (if a new tag is in scope)
- [ ] **C1.1** §11.4.40 full-suite pre-tag sweep on owned scope; capture the log; regression-isolate any FAIL against `merge-base..HEAD` (empty diff ⇒ pre-existing, not a blocker).
- [ ] **C1.2** Live coder re-proven at tag time (§11.4.5) — requires A0 done.

## Task C2: Merge-to-main + production tag — **operator decision (§11.4.167)**
- [ ] **C2.1** On explicit operator approval only: merge `feature/helixllm-full-extension` → `main` via merge-onto-latest (§11.4.113 ff-only, no force), cascade to touched owned submodules at reviewed HEADs.
- [ ] **C2.2** Cut the next prefixed tag (`helix-code-<version>`, §11.4.151) across main repo + owned submodules identically.
- [ ] **C2.3** Push to all configured remotes (§2.1); if operator provides GitFlic/GitVerse URLs, add them first (§6.W allows 4).

## Task C3: Manual QA-team final confirmation — **operator-side (§11.4.185)**
- [ ] **C3.1** Hand off the released build to the QA team for manual final confirmation; the agent WAITS (does not self-certify), while progressing any other non-blocked item.
- [ ] **C3.2 Terminal:** scope is "fully done" only after QA-team manual sign-off.

## Task C4: Session handoff (§11.4.127/§11.4.131)
- [ ] **C4.1** Ensure the standing resumption file is moment-valid so a fresh session resumes with zero loss.

---

## Self-Review (writing-plans checklist)

- **Spec coverage:** every master-plan §2 phase + §5 operator decision + §6 immediate item maps to a task above (A0–A3, B1–B5, C1–C4). Gaps: none new — the master plan's own gated items are surfaced in A3/C, not silently dropped.
- **Placeholder scan:** no "TBD/handle-edge-cases" — gated tasks explicitly cite their source plan for full task detail (§11.4.74), which is a citation, not a placeholder.
- **Type/reference consistency:** `ClassAgent`, `Budget().free`, the port map (18434 coder / 18435 Lane-B / 18436 translate / 18437 whisper / 18438 tesseract / 18439 vision / 18440 RAG-TEI / 18441 A2A / 18442 image / 18443 video / MCP-gw) are consistent with the broker source + ledger.
- **Honest boundary (§11.4.6):** this plan does NOT claim autonomous progress on coder-up or operator-gated tasks — those are explicitly parked on A0 / operator input. The genuinely-now-actionable set is Task A1 (the 4 R41 streams + A1.5/A1.6) plus Phase B items that don't need the coder.

## Sources
- `docs/research/07.2026/00_master/MASTER_IMPLEMENTATION_PLAN.md` (+ the 3 GO domain plans it consolidates)
- `.superpowers/sdd/progress.md` (R1–R41 ledger, reconciled ground truth)
- Live diagnosis 2026-07-11: `nvidia-smi`, `podman ps -a`, `ls /dev/dri`, `nvidia-ctk cdi list`, `git log/tag/remote`
