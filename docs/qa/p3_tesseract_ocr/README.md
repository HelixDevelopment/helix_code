# P3 — Tesseract OCR in rootless Podman, with golden self-validation

Phase-3 (STT/OCR) deliverable: a **real, self-validated** local OCR service that
produces genuine per-word `conf`+`bbox` evidence — no metadata-only claims, no bluff
(§11.4.6 / §11.4.107(10) / §11.4.123). Advances the HelixQA §11.4.117 / §11.4.137
pixel-oracle dependency. Grounded in
`docs/research/07.2026/07_stt_ocr_whisper_tesseract/07_stt_ocr_whisper_tesseract.md` §2.

## What this is

- **Rootless Podman** container (`Containerfile.tesseract`) running Tesseract (OEM 1 /
  LSTM) behind a tiny FastAPI **`/v1/ocr`** endpoint (`ocr_service.py`) returning the
  report's unified contract `{engine, words:[{text,conf,bbox,line,block}], full_text, mean_conf}`
  via `pytesseract.image_to_data`.
- **Golden self-validation** (`gen_fixtures.py` + `validate.py`, §11.4.107(10)):
  - **golden-good** — a 2-line text block with KNOWN tokens `HELIX OCR TEST 42`;
    ASSERT the OCR output contains all 4 tokens AND `mean_conf > floor`.
  - **golden-bad** — a white+faint-noise image with NO text; ASSERT the known tokens
    are NOT returned — proving the analyzer cannot bluff.
- CPU-only by design (the GPU is reserved for the fleet-model proof).

## How to run (rootless, no sudo)

```bash
cd docs/qa/p3_tesseract_ocr
./run.sh            # build -> capture version -> gen fixtures -> boot service -> validate
# exit 0 == SELF-VALIDATION PASS
```

Manual equivalent:
```bash
podman build -f Containerfile.tesseract -t localhost/helix-ocr:tesseract5 .
# fixtures MUST be generated inside the container (pinned DejaVu font + PIL):
podman run --rm --user 0 -v "$PWD":/work:z -w /work localhost/helix-ocr:tesseract5 python3 gen_fixtures.py
podman run -d --name helix-ocr-run -p 18080:8080 localhost/helix-ocr:tesseract5
OCR_BASE_URL=http://127.0.0.1:18080 python3 validate.py    # host needs Pillow/numpy/requests
curl -s http://127.0.0.1:18080/health
podman rm -f helix-ocr-run
```

## Calibrated confidence floor (§11.4.6 / §11.4.107(13))

The floor is **80** (`OCR_CONF_FLOOR`, default in `validate.py`). It is calibrated on
the **actual golden-good output**, NOT guessed from literature: the real per-word
confidences measured on our fixture are **HELIX=94, OCR=96, TEST=96, 42=96**
(`mean_conf = 95.5`). 80 sits comfortably below the observed cluster while rejecting
the low-conf noise Tesseract emits on the golden-bad image (all conf < 0 → filtered →
`mean_conf = 0`). Service-side per-word keep-floor is `OCR_MIN_CONF=60`.

## Captured evidence (real endpoint responses)

| File | Content |
|---|---|
| `evidence/container_tesseract_version.txt` | `id` (uid 10001, non-root) + `tesseract --version` from inside the container |
| `evidence/health.json` | live `/health` — engine + version + config |
| `evidence/golden_good_response.json` | real `/v1/ocr` response: `full_text="HELIX OCR TEST 42"`, per-word conf 94-96 + bboxes, `mean_conf=95.5` |
| `evidence/golden_bad_response.json` | real `/v1/ocr` response on the no-text image: `words=[]`, `full_text=""`, `mean_conf=0`, 48 raw noise tokens, **zero** known-token leak |
| `evidence/self_validation_verdict.json` | `"PASS": true` — both asserts satisfied |
| `fixtures/golden_good.png`, `fixtures/golden_bad.png` | the deterministic test inputs |

Verdict: **PASS**, reproduced **3/3** consecutive runs (determinism §11.4.50).

## Honest deviations / gaps (§11.4.6 / §11.4.99(B))

- **Tesseract version: 5.3.0, NOT 5.5.** The Debian `bookworm-backports`
  `tesseract-ocr` package resolves to **5.3.0** (backports does not carry 5.5.x). The
  report's "5.5" target would require Debian trixie / Ubuntu 24.10+ / a source build.
  5.3.0 is still Tesseract 5.x with the **OEM 1 (LSTM)** default the report mandates;
  the per-word `conf`+`bbox` (`image_to_data`) contract is identical. Wiring a HelixQA
  gate on a specific 5.5 behaviour would need the version bump first — **UNCONFIRMED**
  whether any 5.5-only behaviour is needed.
- **Fixtures MUST be generated in-container.** The host's DejaVu font paths differ, so
  a host re-render silently produces a broken tiny-font image (this was the actual
  root cause of two early FAILs — `validate.py` no longer regenerates on the host).
- **CPU-only.** GPU (CDI/rootless) OCR is out of scope here; Tesseract is CPU-bound
  anyway. The STT leg (faster-whisper/Parakeet, GPU) from the report is separate.

## Anti-bluff notes

Every number above comes from a real HTTP call to the real Tesseract engine on real
pixels — no simulated responses, no hardcoded OCR output. The golden-bad case is the
load-bearing anti-bluff proof: the analyzer returns NOTHING on a blank image, so a
green golden-good result cannot be a fabricated constant.
