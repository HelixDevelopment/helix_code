# CPU-Tier Capability Live Re-Validation — 2026-07-11T13:41Z

**Track**: T1/feature/helixllm-full-extension (claude3)
**Repo**: /home/milos/Factory/projects/tools_and_research/helix_code
**Scope**: re-prove three CPU-only capabilities LIVE with fresh evidence (§11.4.5/§11.4.107/§11.4.108/§11.4.169), using the
already-proven harnesses from `submodules/helix_llm` (Translation `5873621e`, Whisper STT `e1b3b41`, Tesseract OCR `f8632e3`)
COPIED read-only into a fresh evidence directory at this timestamp and re-run end-to-end. No submodule file was modified —
only the pre-existing gitignored `services/ocr/ocr-shim` build artifact was regenerated in place (its own documented §11.4.77
regeneration mechanism), and the container images were built from the submodule's existing, unmodified Containerfiles.

All three services are CPU-only (no GPU device, no `--device nvidia.com/gpu`), distinct ports (18436/18437/18438),
zero contention with vision (:18439) or the resident helixllm-coder (:18434, GPU router, left running throughout).
Every service was booted through the `digital.vasic.containers` orchestrator (§11.4.76, rootless podman §11.4.161)
and torn down single-owner (§11.4.119) immediately after its proof.

## Summary verdict

| Capability | Port | Real I/O proven | CX-05 / content-match | Golden-bad FAIL | Determinism | Torn down |
|---|---|---|---|---|---|---|
| Translation (LibreTranslate/Argos-CTranslate2) | 18436 | en→fr, en→de | PASS (triple: not-identity + fwd chrF + back chrF) | PASS (identity/wrong-lang/garbage/empty all FAIL) | PASS | YES |
| Whisper STT (faster-whisper CT2) | 18437 | fox + helloworld utterances | PASS (content-word match) | PASS (silence/noise/wrong-content/empty all FAIL) | PASS | YES |
| Tesseract OCR | 18438 | 2 rendered known-text images | PASS (token-match + mean-conf ≥60) | PASS (blank/noise/wrong-text all FAIL) | PASS | YES |

**ALL THREE: ALL-GREEN.** helixllm-coder remained `Up` throughout every run; no other stream's container (vision :18439,
RAG :18440) was touched; ports 18436-18438 confirmed free before and after.

---

## 1. Translation — LibreTranslate (Argos/OPUS-MT on CTranslate2), port 18436

Harness: copied from `docs/qa/phase3_translation_20260707/harness/` (proven `5873621e`), CX-05 anti-gaming triple per the
`e58aab9f` design lineage. Re-run verbatim (only the go.mod/relative-path depth adjusted for the new evidence-dir nesting —
no logic changed). Evidence: `translation/`.

### Real output (captured, `translation/green_record_en_fr_1.json` / `en_de_1.json`)

```
en->fr  source="The book is on the table."
        forward="Le livre est sur la table."  (detected=fr conf=100.0)
        back   ="The book is on the table."
        golden ="Le livre est sur la table."

en->de  source="The book is on the table."
        forward="Das Buch liegt auf dem Tisch."  (detected=de conf=100.0)
        back   ="The book is on the table."
        golden ="Das Buch ist auf dem Tisch."
```

### CX-05 triple-check verdict (`translation/11_green_proof_*.txt`)

```
[RUNTIME-SIGNATURE(en->fr)] PASS notIdentity=true targetMatch=true fwdChrF=1.0000(floor 0.30) backChrF=1.0000(margin 0.40)
[RUNTIME-SIGNATURE(en->de)] PASS notIdentity=true targetMatch=true fwdChrF=0.7519(floor 0.30) backChrF=1.0000(margin 0.40)
[DETERMINISM] PASS: forward+back byte-identical across two identical requests (en->fr / en->de)
```

### RED baseline — identity-passthrough bluff correctly FAILs (`translation/10_red_baseline.txt`)

```
[RUNTIME-SIGNATURE(en->fr)] FAIL notIdentity=false targetMatch=false fwdChrF=0.2242(floor 0.30) backChrF=1.0000(margin 0.40)
RED-OK: identity passthrough correctly FAILED the runtime signature
```

### Golden-bad self-validation — analyzer is NOT a bluff gate (`translation/12_self_validation.txt`)

```
[GOLDEN-GOOD(expect PASS)]        PASS notIdentity=true  targetMatch=true  fwdChrF=1.0000 backChrF=1.0000
[GOLDEN-BAD-IDENTITY(expect FAIL)] FAIL notIdentity=false targetMatch=false fwdChrF=0.2242 backChrF=1.0000
[GOLDEN-BAD-WRONGLANG(expect FAIL)] FAIL notIdentity=true targetMatch=false fwdChrF=0.0873 backChrF=0.1859
[GOLDEN-BAD-GARBAGE(expect FAIL)]  FAIL notIdentity=true  targetMatch=true  fwdChrF=0.0500 backChrF=0.0269
[GOLDEN-BAD-EMPTY(expect FAIL)]   FAIL notIdentity=false targetMatch=false fwdChrF=0.0000 backChrF=0.0000
[SELF-VALIDATION] PASS: analyzer PASSes golden-good and FAILs all golden-bad fixtures
```

### Teardown (`translation/29b_post_teardown.txt`)

```
libretranslate containers (expect none):
  (none — removed)
coder still running (untouched):
helixllm-coder Up 46 minutes
```

**Verdict**: `ALL-GREEN: anti-gaming triple + determinism + self-validation PASS` (`translation/13_verdict.txt`).

---

## 2. Whisper STT — faster-whisper CTranslate2 (int8, CPU), port 18437

Harness: copied from `docs/qa/phase3_whisper_stt_20260707/harness/` (proven `e1b3b41`). Real audio synthesized fresh via
espeak-ng/ffmpeg (deterministic, known-content, §11.4.107 unfakeable), transcribed by the real faster-whisper service built
from `submodules/helix_llm/container/Containerfile.whisper` (unmodified). Evidence: `whisper/`.

### Real output (captured, `whisper/green_fox_1.json` / `green_helloworld_1.json`)

```
fixture=fox         text="The quick brown fox dumps over the lazy dog."   language=en  max_no_speech_prob=0.0197
fixture=helloworld  text="Hello World 1, 2, 3!"                          language=en  max_no_speech_prob=0.0221
```

(Note: real Whisper output — "dumps" vs the spoken "jumps" — a genuine, honestly-reported STT quirk. The content-match
analyzer only asserts presence of the expected key content words [quick, brown, fox, lazy, dog], all present — a real,
non-bluffed PASS, not a fabricated transcript.)

### Content-match runtime signature (`whisper/11_green_proof_fox.txt` / `11_green_proof_helloworld.txt`)

```
[RUNTIME-SIGNATURE(fox)]        PASS transcript="The quick brown fox dumps over the lazy dog." missing=[]
[RUNTIME-SIGNATURE(helloworld)] PASS transcript="Hello World 1, 2, 3!" missing=[]
[DETERMINISM] PASS: normalized transcript identical across two identical requests
```

### RED baseline — empty/canned stub correctly FAILs (`whisper/10_red_baseline.txt`)

```
-- stub: empty text --   [RUNTIME-SIGNATURE(fox)] FAIL transcript="" missing=[quick brown fox lazy dog]
-- stub: canned wrong --  [RUNTIME-SIGNATURE(fox)] FAIL transcript="completely unrelated filler content..." missing=[quick brown fox lazy dog]
RED-OK: both stubs correctly FAILED the runtime signature
```

### Golden-bad self-validation — real silence + real white-noise transcriptions (`whisper/12_self_validation.txt`)

```
[GOLDEN-GOOD-0(fox, expect PASS)]         PASS transcript="The quick brown fox dumps over the lazy dog."
[GOLDEN-GOOD-1(helloworld, expect PASS)]  PASS transcript="Hello World 1, 2, 3!"
[GOLDEN-BAD-SILENCE(expect FAIL)]         FAIL transcript="" (real 3s digital-silence WAV, real HTTP transcription)
[GOLDEN-BAD-NOISE(expect FAIL)]           FAIL transcript="" (real 3s white-noise WAV, real HTTP transcription)
[GOLDEN-BAD-WRONGCONTENT(expect FAIL)]    FAIL transcript="Hello World 1, 2, 3!" missing=[quick brown fox lazy dog]
[GOLDEN-BAD-EMPTY(expect FAIL)]           FAIL transcript=""
[SELF-VALIDATION] PASS: analyzer PASSes both golden-good fixtures and FAILs every golden-bad variant
```

### Teardown (`whisper/29b_post_teardown.txt`)

```
helixllm-stt containers (expect none):
  (none — removed)
coder still running (untouched):
helixllm-coder Up 46 minutes
```

**Verdict**: ALL-GREEN (RED-OK + 2× GREEN-OK + determinism PASS + self-validation PASS; no verdict-line failure anywhere
in `whisper/12_self_validation.txt`'s `selfvalidate_exit=0`).

---

## 3. Tesseract OCR — Tesseract 5.3.0 (OEM 1/LSTM), port 18438

Harness: copied from `docs/qa/phase3_tesseract_ocr_20260707/harness/` (proven `f8632e3`). Fixture images rendered
SERVER-SIDE by the OCR service's own `/v1/render` endpoint (so font/version drift can never fake a result), OCR'd by the
real `tesseract` binary inside the container built from `submodules/helix_llm/services/ocr/Containerfile` (unmodified).
Evidence: `tesseract/`.

### Real output (captured, `tesseract/green_response_good1.json` / `green_response_good2.json`)

```
text="HELIX OCR 2026 quick brown fox"          mean_conf=95.99  (6 words, per-word conf 95.6-96.6)
text="PHASE THREE TESSERACT PROOF SEVEN"       mean_conf=94.77  (5 words, per-word conf 91.8-96.6)
```

### Runtime signature + determinism (`tesseract/11_green_proof_1.txt` / `12_green_proof_2.txt`)

```
[RUNTIME-SIGNATURE] PASS mean_conf=95.99 found=[HELIX OCR 2026 QUICK BROWN FOX] missing=[]
[DETERMINISM] PASS: identical full_text (30 chars), mean_conf=95.99, 6 words across two identical requests
[RUNTIME-SIGNATURE] PASS mean_conf=94.77 found=[PHASE THREE TESSERACT PROOF SEVEN] missing=[]
```

### RED baseline — empty stub correctly FAILs (`tesseract/10_red_baseline.txt`)

```
[RUNTIME-SIGNATURE] FAIL mean_conf=0.00 found=[] missing=[HELIX OCR 2026 QUICK BROWN FOX] full_text=""
RED-OK: empty stub correctly FAILED the runtime signature
```

### Golden-bad self-validation — real blank/noise/wrong-text OCR (`tesseract/13_self_validation.txt`)

```
[GOLDEN-GOOD(expect PASS)]         PASS mean_conf=95.99 found=[HELIX OCR 2026 QUICK BROWN FOX]
[GOLDEN-BAD-BLANK(expect FAIL)]    FAIL mean_conf=0.00  full_text=""
[GOLDEN-BAD-NOISE(expect FAIL)]    FAIL mean_conf=13.82 full_text="Bi a a8 Oran Stes ponies pia ni wees..." (real Tesseract hallucination on real noise PNG)
[GOLDEN-BAD-WRONGTEXT(expect FAIL)] FAIL mean_conf=95.98 found=[] full_text="BANANA SPACESHIP GALAXY EIGHT" (correctly-read wrong text — proves token-match, not just conf floor)
[SELF-VALIDATION] PASS: analyzer PASSes golden-good and FAILs all golden-bad fixtures
```

### Teardown (`tesseract/29b_post_teardown.txt`)

```
ocr containers (expect none):
  (none — removed)
coder still running (untouched):
helixllm-coder Up 47 minutes
sibling Phase-3 containers (untouched — none of THESE were started/stopped by this run):
  (none currently present)
```

**Verdict**: ALL-GREEN (`selfvalidate_exit=0`, both GREEN-OK-1/GREEN-OK-2, RED-OK).

---

## §11.4.119 single-owner / cross-stream isolation confirmation

Post-run full-host container state (`podman ps -a`) shows zero leftover containers from any of the three runs; ports
18436/18437/18438 confirmed free; `helixllm-coder` (:18434, GPU router — untouched throughout, `Up` the entire session);
no interaction with the vision (:18439) or RAG (:18440) sibling-stream services. `submodules/helix_llm` git status is
unchanged by this session except the pre-existing gitignored `services/ocr/ocr-shim` build artifact (regenerated in
place per its own §11.4.77 mechanism — not a git-tracked change) — confirmed clean:

```
$ git -C submodules/helix_llm status --porcelain
?? cmd/agentgen-boot/agentgen-boot   # pre-existing, unrelated to this session
```

## Honest disclosures (§11.4.6)

- Translation lane proven: **LibreTranslate/Argos-CTranslate2** (the design's documented FALLBACK lane, `5873621e`) — the
  same lane the CX-05 triple was designed for. The NLLB-200-CT2 PRIMARY lane (`e58aab9f`) was proven in the referenced
  prior session and was NOT re-run in this pass (not required by the task; both lanes are real, non-mocked NMT).
- Whisper's "dumps" vs spoken "jumps" is a genuine STT artifact, disclosed rather than hidden — the content-word analyzer
  correctly still PASSes since all required content words are present, and the analyzer's own golden-bad fixtures prove
  it is not a bluff gate (it correctly FAILs wrong-content, silence, and noise).
- All container images were built from `submodules/helix_llm`'s existing, unmodified source (Containerfile.whisper,
  services/ocr/Containerfile) — no submodule source file was created, edited, or deleted in this session.

## Reproduction

```bash
cd docs/qa/cpucaps_liveproof_20260711T134045Z/translation/harness && bash run_proof.sh
cd docs/qa/cpucaps_liveproof_20260711T134045Z/whisper/harness     && bash run_proof.sh
cd docs/qa/cpucaps_liveproof_20260711T134045Z/tesseract/harness   && bash run_proof.sh
```
