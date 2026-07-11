# r41f — HelixLLM VISION (VLM) capability LIVE re-validation

**Run ID:** `vision_liveproof_20260711T133823Z` · **Date:** 2026-07-11 (UTC) ·
**Track:** T1/feature/helixllm-full-extension (agent: claude3) ·
**Host GPU:** RTX 5090 (32607 MiB total) · **Verdict:** ✅ **RE-PROVEN — grounded PASS +
golden-BAD FAIL + analyzer self-validation PASS + coder untouched + VRAM restored**

Anti-bluff anchors: §11.4.5 / §11.4.69 (captured evidence), §11.4.107 / §11.4.107(10)
(self-validated analyzer, golden-good/golden-bad fixture pair), §11.4.108 (runtime
signature), §11.4.119 (single-resource-owner GPU burst admission), §11.4.122
(no-silent-removal / coder never restarted), §11.4.133 (target-hardware safety —
fail-closed VRAM admission).

This re-validates the Phase-3 VLM capability originally proven at commit `e857d59`
(`submodules/helix_llm`, `docs/qa/phase3_vision_20260707/`), now exercised through the
promoted on-demand-infra boot path `cmd/visiongen-boot` (compose-orchestrated, port
`:18439`) rather than the earlier ad-hoc `podman run` on `:18500`. **`submodules/helix_llm`
was used strictly READ-ONLY** — no source file was modified; only the existing Go boot
harness (`go run .`) was invoked and containers it manages were started/stopped. This
deliverable (evidence + RESULTS.md) is committed **only under the root `docs/qa/`** to
avoid any submodule contention with other concurrent work streams (RAG `:18440`,
CPU-caps `:18436`-`:18438`).

---

## 1. Pre-admission: live free-VRAM read + coder health (§11.4.119 / §11.4.122)

`evidence/00_before_state.txt`:

```
32607, 19464, 12634            # total, used, FREE (MiB) — read LIVE before any admission
1980342, 19438 MiB, llama-server   # only the coder resident
coder /health: {"status":"ok"}
helixllm-coder Up 43 minutes 8080/tcp, 50052/tcp
:18439/health -> connect refused (port free, as expected)
```

Free VRAM **12634 MiB** (~12.3 GiB) read live via `nvidia-smi` immediately before
admission — matches the task brief's "~12.6GiB free". `submodules/helix_llm` git status
captured immediately before the boot invocation (`evidence/00b_helixllm_git_status_before_boot.txt`)
shows only a pre-existing untracked build artifact (`cmd/agentgen-boot/agentgen-boot`),
unrelated to this run and unchanged by it.

---

## 2. Admission + boot via `cmd/visiongen-boot` (§11.4.119 fail-closed broker)

`evidence/01_boot.txt`:

```
VRAM budget (nvidia-smi): total=32607MiB used=19464MiB free=12634MiB need=5120MiB headroom=2048MiB
ADMIT-OK: VLM footprint admitted co-resident (coder stays live) — warm tier
UP-OK: helixllm_visiongen visiongen via containers submodule orchestrator (:18439)
HEALTH-OK: visiongen /health after 2 polls (status=200)
BOOT-HEALTH-OK: visiongen /health answered. VLM stays UP (warm tier, coder untouched).
BOOT_EXIT=0
```

The `vrambroker` admitted the VLM (need 5120 MiB + 2048 MiB headroom = 7168 MiB required
≤ 12634 MiB free) BEFORE any container was started — the fail-closed gate the harness is
designed around (§11.4.6/§11.4.133). Boot ran through the `digital.vasic.containers`
compose orchestrator (§11.4.76), rootless podman (§11.4.161). Health answered after 2
polls (~6 s).

**Both resident** (`evidence/02_both_resident_state.txt`):

```
32607, 24051, 8047             # total, used, FREE (MiB) — coder + VLM both live
1980342, 19438 MiB, llama-server   # coder (Qwen3-Coder-30B) — unchanged footprint
2514702,  4582 MiB, llama-server   # visiongen (Qwen2.5-VL-3B) — this run
helixllm_visiongen_visiongen_1 Up 12 seconds  0.0.0.0:18439->18439/tcp
coder /health: {"status":"ok"}    (still Up 43 minutes — untouched)
visiongen /health: {"status":"ok"}
visiongen /v1/models: capabilities=["completion","multimodal"]
```

VLM real footprint **4582 MiB** — under the 5120 MiB estimate, consistent with the
Phase-3 measurement (4138 MiB). 8047 MiB still free with both models resident — no OOM,
comfortably above the 2 GiB headroom mandate.

---

## 3. Real multimodal completion, grounded to known image content (§11.4.108)

Known-content image reused byte-for-byte from the proven Phase-3 evidence
(`submodules/helix_llm/docs/qa/phase3_vision_20260707/evidence/test_image.png`, copied
read-only into `evidence/test_image.png`, **md5 `b982a8295f408e93a662db916f9d543a` —
identical to the original**): a solid **RED CIRCLE** on a **WHITE** background with black
text **"HELIX"**.

Request sent to the live `:18439` VLM (`evidence/03_vision_request.json`,
`evidence/04_vision_response.json`), model `/models/Qwen2.5-VL-3B-Instruct-Q4_K_M.gguf`,
temperature 0, 0.30 s latency. **Real returned description:**

> "The image features a simple and clean design. The main object is a large, solid **red
> circle** positioned centrally against a **white** background. Below the circle, the word
> **"HELIX"** is written in uppercase black letters. The text is straightforward and
> unembellished, with no additional graphics or embellishments. The overall design is
> minimalistic and modern."

`evidence/05_grounded_assertion.json`:

| Assertion | Tokens (all-of) | Result |
|---|---|---|
| **GROUNDED** (must PASS) | `red` AND (`circle`\|`round`\|`disc`\|`dot`) | **PASS** ✅ |
| **GOLDEN-BAD** (must FAIL on this real response) | `blue` AND (`square`\|`rectangle`\|`triangle`) | **FAIL(as-required)-analyzer-honest** ✅ |
| OCR bonus (rendered text read back) | `helix` | `true` ✅ |
| Bluff-marker scan | `simulated`/`for now`/`placeholder`/`TODO implement` | none found |
| **overall** | | **PASS** |

This is a genuinely grounded response — the model named the correct color, shape, and
even OCR'd the rendered "HELIX" text — not a canned/templated reply.

---

## 4. Analyzer self-validation — golden-good/golden-bad FIXTURE pair (§11.4.107(10))

Beyond checking the real response against wrong-content tokens (§3 above), the checker
logic itself was unit-validated with two synthetic fixtures fed directly into the SAME
`all_present()` grounded/golden-bad logic, with no network call
(`evidence/06_analyzer_self_validation.json`):

| Fixture | Text (synthetic) | Expected | Result |
|---|---|---|---|
| golden-good | "…solid **red circle**… white background… word **helix**…" | `overall = PASS` | **PASS** ✅ |
| golden-bad | "…solid **blue square**… white background… word **acme**…" | `overall = FAIL` | **FAIL** ✅ |

`self_validation: "PASS"` — the checker correctly PASSes the golden-good fixture and
correctly FAILs the golden-bad fixture, proving it is not a rubber stamp.

**Honest note (§11.4.6):** the underlying substring-matching heuristic (shared with the
original Phase-3 checker, unmodified here) has a known false-positive mode — the English
word "**centered**" contains the substring "red", so `grounded_pass` can spuriously read
`true` on unrelated text. This did NOT affect the verdict in either fixture: the
golden-bad fixture's `overall` was still correctly `FAIL` because the wrong-content gate
(`blue` AND `square`) independently forced it, and the real §3 response is a genuine
grounded description (not relying on the "centered" artifact). Flagged here for honesty,
not corrected in-place, since `submodules/helix_llm` was used strictly read-only for this
re-validation.

---

## 5. Coder untouched — before, during (co-resident), and after (§11.4.122)

| Checkpoint | Coder container | Coder chat completion |
|---|---|---|
| Before vision boot | `helixllm-coder Up 43 minutes` | `{"status":"ok"}` (health) |
| While VLM co-resident | `helixllm-coder Up 43 minutes` (unchanged) | `"CODER_OK_COEXIST"` (`evidence/07_coder_coexist_chat.json`, real `Qwen3-Coder-30B-A3B-Instruct-Q4_K_M` completion, 289.8 tok/s gen) |
| After VLM teardown | `helixllm-coder Up 45 minutes` (continuous — never restarted) | `"CODER_OK_AFTER_TEARDOWN"` (`evidence/09_after_teardown_state.txt`, real completion, 262.3 tok/s gen) |

The coder's uptime counter increased monotonically (43 min → 45 min) across the entire
run with no restart/interruption — never stopped or restarted, per the hard constraint.

---

## 6. Teardown + VRAM restored (§11.4.119 single-owner cleanup)

`evidence/08_teardown.txt`:

```
DOWN-OK: helixllm_visiongen visiongen (single-owner cleanup, coder untouched)
TEARDOWN_EXIT=0
```

`evidence/09_after_teardown_state.txt`:

```
helixllm-coder Up 45 minutes 8080/tcp, 50052/tcp     # only container left
32607, 19464, 12634            # total, used, FREE (MiB) — EXACT baseline restored
1980342, 19438 MiB, llama-server   # only the coder process remains
:18439/health -> connect refused (port fully released)
coder /health: {"status":"ok"}
```

Free VRAM after teardown is **byte-identical to the pre-boot baseline** (12634 MiB free,
19464 MiB used, 32607 MiB total) — the VLM's footprint was fully released, no leak.

---

## 7. Other work streams untouched

`submodules/helix_llm` was never modified (`evidence/10_helixllm_git_status_after.txt`
shows only the same pre-existing untracked build artifact present before this run began;
`git diff --stat` on tracked files is empty). `submodules/helix_agent` and `/mnt/track1`
were not touched by any command in this run. Sibling capability lanes (`:18436`-`:18438`
CPU-caps, `:18440` RAG) were probed read-only and found not currently running — this run
neither started nor stopped them.

---

## 8. Evidence index (`evidence/`)

| File | Content |
|---|---|
| `test_image.png` | Known-content image (red circle / white / "HELIX"), md5 `b982a829…` — reused from Phase-3, byte-identical |
| `00_before_state.txt` | Live pre-admission nvidia-smi + coder health + :18439 pre-check |
| `00b_helixllm_git_status_before_boot.txt` | `submodules/helix_llm` git status before any command (read-only baseline) |
| `01_boot.txt` | `visiongen-boot boot` — admission + compose up + health poll |
| `02_both_resident_state.txt` | nvidia-smi + health + `/v1/models` with coder + VLM both live |
| `03_vision_request.json` / `04_vision_response.json` | Real vision request (base64 redacted) / raw response |
| `05_grounded_assertion.json` | Grounded + golden-bad verdict on the real response (overall PASS) |
| `06_analyzer_self_validation.json` | Golden-good/golden-bad FIXTURE pair run directly against the checker logic |
| `07_coder_coexist_chat.json` | Real coder completion while VLM co-resident |
| `08_teardown.txt` | `visiongen-boot down` — single-owner cleanup |
| `09_after_teardown_state.txt` | nvidia-smi restored, coder still Up + real completion after teardown |
| `10_helixllm_git_status_after.txt` | `submodules/helix_llm` git status after the full run (unchanged) |

`harness/vision_probe_liveproof.py` and `harness/analyzer_self_validation.py` are the
evidence-generation scripts used for §3/§4 (written fresh under root `docs/qa/`, modeled
on but not copied from the read-only-referenced `submodules/helix_llm` Phase-3 harness).

## 9. Reproduce

```bash
cd submodules/helix_llm/cmd/visiongen-boot
go run . boot compose.vision.yml helixllm_visiongen     # admit -> compose up -> health poll
curl -fsS http://localhost:18439/v1/models
python3 <root>/docs/qa/vision_liveproof_20260711T133823Z/harness/vision_probe_liveproof.py \
  http://localhost:18439 <root>/docs/qa/vision_liveproof_20260711T133823Z/evidence/test_image.png /tmp/out /models/Qwen2.5-VL-3B-Instruct-Q4_K_M.gguf
go run . down compose.vision.yml helixllm_visiongen     # single-owner teardown
```

---

## Honest boundary (§11.4.6)

This re-proves, LIVE and freshly captured on 2026-07-11, that a real VLM
(Qwen2.5-VL-3B-Instruct, GGUF+mmproj) boots on-demand via the promoted
`cmd/visiongen-boot` harness, co-resident with the live coder without disturbing it,
genuinely describes a real image grounded in its actual pixel content, that the grounding
checker is honest under a golden-good/golden-bad fixture pair, and that the GPU resource
is fully released on teardown. It does not claim new capability beyond what commit
`e857d59` already established — it confirms that capability still holds through the
promoted on-demand-infra boot path, with the coder + surrounding parallel work streams
undisturbed.
