# Phase-1 HelixQA Vision Bank (Bank B) — Run Results

**Run ID:** `phase1_helixqa_vision_20260708T061809Z`
**Track:** `(T1/feature/helixllm-full-extension)`
**Date:** 2026-07-08
**Scope:** `docs/research/07.2026/00_master/MASTER_IMPLEMENTATION_PLAN.md` §6.5 +
`docs/research/07.2026/02_vision_generative/CAPABILITIES_MASTER_PLAN_v2.md` §5.2 +
`docs/research/07.2026/12_helixqa_testing/12_helixqa_testing.md` Part 3 "Bank B".

## 1. Vision VLM boot (broker-admission-gated, §11.4.76/§11.4.119/§11.4.161)

Live VRAM re-read immediately before boot (§11.4.6, DZ-23):

```
memory.total [MiB], memory.used [MiB], memory.free [MiB]
32607 MiB, 19430 MiB, 12691 MiB
```

`podman ps` before boot: only `helixllm-coder` resident.

```
$ visiongen-boot admit-check
VRAM budget (nvidia-smi): total=32607MiB used=19430MiB free=12691MiB need=5120MiB headroom=2048MiB
ADMIT-OK: VLM footprint admitted co-resident (coder stays live) — warm tier
EXIT=0

$ visiongen-boot boot compose.vision.yml helixllm_visiongen
VRAM budget (nvidia-smi): total=32607MiB used=19430MiB free=12691MiB need=5120MiB headroom=2048MiB
ADMIT-OK: VLM footprint admitted co-resident (coder stays live) — warm tier
UP-OK: helixllm_visiongen visiongen via containers submodule orchestrator (:18439)
HEALTH-OK: visiongen /health after 2 polls (status=200)
BOOT-HEALTH-OK: visiongen /health answered. VLM stays UP (warm tier, coder untouched).
EXIT=0
```

Post-boot `podman ps`: both `helixllm-coder` (untouched, `Up 3 minutes` at boot time,
container id unchanged throughout) and `helixllm_visiongen_visiongen_1` resident.
`nvidia-smi` post-boot: `24017 MiB used / 8104 MiB free` (both models co-resident).

**Real multimodal completion confirmed** (not metadata-only — closes AB-15,
capabilities plan §2.5): a red-square PNG generated with ImageMagick, POSTed to
`http://localhost:18439/v1/chat/completions` with prompt "What color is the square
shape in this image?", returned `"content":"Red"` — a genuine, correct, non-simulated
answer from the live Qwen2.5-VL-3B-Instruct-Q4_K_M model.

## 2. Ground-truth fixture authoring (§11.4.6 — grounded against the LIVE model)

Four candidate image+prompt fixtures were generated (ImageMagick shapes/text) and
EACH was run against the live `:18439` endpoint during authoring to confirm the
expected answer is genuinely achievable — twice each, to confirm determinism at
`temperature=0` (§11.4.50) — BEFORE being committed as a "must-match" assertion in
the bank YAML.

| Fixture | Prompt (abridged) | Live model's real answer | Repeat #2 |
|---|---|---|---|
| `red_circle.png` | shape+color of main object | "The main object in the image is a circle, and it is red." | "...circle, and it is red in color." (both facts present) |
| `three_blue_circles.png` | how many blue circles | `"3"` | `"3"` |
| `text_helix.png` | what word is written | `"HELIX"` | (deterministic, re-confirmed) |
| `spatial_left_right.png` | red square left or right of blue | `"left"` | `"left"` |

All four were genuinely achievable — none guessed. Ground-truth PNGs are committed
under `submodules/helix_qa/data/vision_gt/` (copies also in `ground_truth/` here).

## 3. Bank authored: `submodules/helix_qa/banks/helixllm_vision.yaml` (copy alongside this file)

Analyzer: `submodules/helix_qa/cmd/helixqa-verify-vision` (copy of `main.go` alongside
this file as `helixqa-verify-vision_main.go.txt`) — a thin, project-agnostic
`dispatches_to` Go binary (CONST-051(B), no HelixQA core-engine change) that POSTs
image+prompt to the vision endpoint, strictly fact-matches the response
(case-insensitive substring, ALL expected tokens required for `pass=true`), and
writes an auditable verdict JSON embedding the raw response + expected facts + metric.

7 test cases: `VIS-UNDERSTAND-001`, `VIS-COUNT-001`, `VIS-OCR-READ-001`,
`VIS-SPATIAL-001`, `VIS-SELF-VALIDATE-001-GOOD`, `VIS-SELF-VALIDATE-001-BAD`
(mandatory §11.4.107(10)), `VIS-LIVENESS-001` (honest SKIP — no `$DISPLAY` on this
headless authoring host).

## 4. Bank run — through the REAL `pkg/testbank.Dispatcher` mechanism

A throwaway Go harness (NOT committed — removed before landing, §11.4.84) loaded
`banks/helixllm_vision.yaml` via `testbank.LoadFile` and ran every case through the
actual `testbank.Dispatcher` (`os/exec` `DeviceExec` + `ContentAssertingResolver`
evidence resolver) — proving the BANK ITSELF (not just manual CLI calls) is wired
correctly end-to-end.

### 4a. Run with the correct (unmutated) analyzer

```
Loaded bank "HelixLLM Vision (VLM) Capability — Bank B" (1.0) — 7 test cases
[VIS-UNDERSTAND-001] PASS  evidence=map[qa-results/helixllm_vision/understand_001_verdict.json | json:case_result==true | json:matched_facts==2 | json:hallucinated==false:[...]]
[VIS-COUNT-001] PASS  evidence=map[qa-results/helixllm_vision/count_001_verdict.json | json:case_result==true | json:matched_facts==1 | json:hallucinated==false:[...]]
[VIS-OCR-READ-001] PASS  evidence=map[qa-results/helixllm_vision/ocr_read_001_verdict.json | json:case_result==true | json:matched_facts==1 | json:hallucinated==false:[...]]
[VIS-SPATIAL-001] PASS  evidence=map[qa-results/helixllm_vision/spatial_001_verdict.json | json:case_result==true | json:matched_facts==1 | json:hallucinated==false:[...]]
[VIS-SELF-VALIDATE-001-GOOD] PASS  evidence=map[qa-results/helixllm_vision/self_validate_001_golden_good_verdict.json | json:case_result==true | json:pass==true | json:matched_facts==2:[...]]
[VIS-SELF-VALIDATE-001-BAD] PASS  evidence=map[qa-results/helixllm_vision/self_validate_001_golden_bad_verdict.json | json:case_result==true | json:pass==false | json:matched_facts==0:[...]]
[VIS-LIVENESS-001] SKIP-OK: requires_env HELIXQA_LAB_HAS_DISPLAY unset (honest §11.4.3 skip)

TOTAL: pass=6 fail=0 skip=1
overall exit=0
```

Per-fixture PASS verdict JSONs are in `verdicts/` alongside this file (copied from
`submodules/helix_qa/qa-results/helixllm_vision/*.json`, which is git-ignored raw
evidence per §11.4.128 — this curated copy is the committed record per §11.4.83).

### 4b. Paired §1.1 mutation proof — the analyzer's discriminator is load-bearing

The fact-matching assertion in `cmd/helixqa-verify-vision/main.go` —

```go
v.Pass = v.ExpectedCount > 0 && v.MatchedFacts == v.ExpectedCount && !v.Hallucinated
```

— was replaced with an unconditional `v.Pass = true // MUTATED for paired §1.1
mutation test — always pass`, rebuilt, and swapped into `bin/helixqa-verify-vision`.
The SAME bank was re-run through the SAME `Dispatcher`:

```
[VIS-UNDERSTAND-001] PASS  ...
[VIS-COUNT-001] PASS  ...
[VIS-OCR-READ-001] PASS  ...
[VIS-SPATIAL-001] PASS  ...
[VIS-SELF-VALIDATE-001-GOOD] PASS  ...
[VIS-SELF-VALIDATE-001-BAD] FAIL  reason=dispatch_exit_1 output=FAIL: VIS-SELF-VALIDATE-001-BAD matched=0/2 hallucinated=false expect_fail=true raw_pass=true response="The main object in the image is a circle, and it is red in color."
[VIS-LIVENESS-001] SKIP-OK: requires_env HELIXQA_LAB_HAS_DISPLAY unset

TOTAL: pass=5 fail=1 skip=1
overall exit=1
```

**The mutation was caught**: with the real discriminator removed, the analyzer's raw
`pass` field wrongly flips to `true` on the golden-bad fixture (facts "square"/"blue"
genuinely absent from the response), and `VIS-SELF-VALIDATE-001-BAD`'s
`--expect-fail` inversion (mirroring the existing
`helixcode-ensemble-members.yaml` `HXC-ENS-003` `panoptic-validate-recording
--expect-fail` convention) correctly turns that bluff-PASS into a bank-level FAIL.
This is the proof the constitution's §11.4.107(10) self-validated-analyzer mandate
requires: a bank that would pass its own golden-bad fixture is a bluff gate; this
one does not.

A standalone (non-Dispatcher) run of the mutated binary against the golden-bad
fixture was also captured (`mutation_proof/self_validate_001_golden_bad_MUTATED_verdict_STANDALONE.json`):
`"pass": true, "expect_fail": true, "case_result": false` — i.e. the raw verdict
bluff-PASSed (`pass=true` on content that says "circle"/"red", not "square"/"blue"),
and `case_result=false` (would exit 1) confirms the inversion catches it at both the
standalone-CLI layer and the full-Dispatcher layer.

The mutation was then **reverted** (`cp` from a pre-mutation backup), rebuilt, and
re-run — confirmed `grep -c "MUTATED for paired" main.go` == 0 (zero residue,
§11.4.84 quiescence) and the bank re-ran clean (§4a numbers, `pass=6 fail=0 skip=1`).
No mutated code was ever committed to the repository at any point.

## 5. Teardown (§11.4.119 single-owner cleanup) — coder confirmed untouched

Before teardown: `helixllm-coder` (Up 16-17 min, unchanged container id
`1484eac80acb`) + `helixllm_visiongen_visiongen_1` (Up 13 min) both resident;
`nvidia-smi` `24097 MiB used / 8024 MiB free`.

```
$ visiongen-boot down compose.vision.yml helixllm_visiongen
DOWN-OK: helixllm_visiongen visiongen (single-owner cleanup, coder untouched)
```

After teardown: `podman ps` shows ONLY `helixllm-coder` (same container id
`1484eac80acb`, `Up 17 minutes`, `host` network mode, `running` since
`2026-07-08 11:01:24`) — the vision container is fully removed from `podman ps -a`
as well (compose-down semantics, not merely stopped). `nvidia-smi` returns to the
pre-boot baseline exactly: `19430 MiB used / 12691 MiB free`.

**Coder confirmed still genuinely serving** (not just "container running" —
a real, non-simulated model response): `curl http://localhost:18434/v1/models`
returns `Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf` with real GGUF metadata
(`n_params:30532122624`, `n_ctx_train:262144`). See `boot_teardown/before_after.txt`
for the full captured transcript.

## 6. Summary

| Item | Result |
|---|---|
| Vision VLM booted | YES — admission-gated, real `/health` 200, real multimodal completion confirmed |
| Bank authored | `banks/helixllm_vision.yaml`, 7 cases, grounded fixtures (§11.4.6) |
| Bank run (correct analyzer) | 6 PASS / 0 FAIL / 1 honest SKIP (no `$DISPLAY`) |
| Self-validation (§11.4.107(10)) | golden-good PASS + golden-bad FAIL (raw) / PASS (case, via `--expect-fail`) |
| Paired §1.1 mutation | Confirmed caught (mutated analyzer → `VIS-SELF-VALIDATE-001-BAD` FAILs the bank) |
| Mutation residue | Zero — reverted + re-verified before commit |
| Vision teardown | YES — single-owner, `podman ps -a` confirms full removal |
| Coder (`helixllm-coder`) | Untouched throughout — same container id, uptime monotonic, genuine `/v1/models` response before AND after |
| VRAM | Returns to exact pre-boot baseline (12691 MiB free) after teardown |

## Sources / evidence paths

- `verdicts/*.json` — per-fixture verdict artefacts from the correct-analyzer run.
- `ground_truth/*.png` — the 4 ImageMagick-generated ground-truth fixtures.
- `conduit/conduit.events.jsonl` + `conduit.status.json` — §11.4.116 real-time
  conductor channel trace from the correct-analyzer run (challenge_start →
  vision_call → evidence_captured → challenge_verdict per case).
- `mutation_proof/self_validate_001_golden_bad_MUTATED_verdict_STANDALONE.json` —
  standalone mutated-analyzer run against the golden-bad fixture.
- `boot_teardown/before_after.txt` — `podman ps` / `podman ps -a` / `nvidia-smi` /
  coder `/v1/models` transcripts before and after teardown.
- `helixllm_vision.yaml` — the committed bank (copy for this evidence bundle).
- `helixqa-verify-vision_main.go.txt` — the committed analyzer source (copy).
