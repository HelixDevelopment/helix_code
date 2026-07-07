# HelixLLM Phase-3 CPU Translation (NMT) — END-TO-END PROOF (§11.4.108)

| | |
|---|---|
| **Status** | **COMPLETE — ALL-GREEN.** Real CPU translation service booted via the containers submodule, anti-gaming triple + determinism + analyzer self-validation all PASS, torn down single-owner, coder untouched. |
| **Run-id** | `phase3_translation_20260707` |
| **Date** | 2026-07-07T07:05:03Z → 07:05:59Z (UTC), host `x86_64`, podman 5.7.1, rootless |
| **Branch / Track** | `feature/helixllm-full-extension` · `(T1/main)` |
| **Design** | `docs/research/07.2026/00_master/TRANSLATION_PROVIDER.md` (commit `c9ac8683`) §4 acceptance signature |
| **Pattern mirrored** | `docs/qa/phase3_embeddings_20260706/harness/` (embeddings proof, commit `e46df52c`) |
| **Lane** | **stock LibreTranslate** (Argos/OPUS-MT on CTranslate2, CPU) — the design's DOCUMENTED FALLBACK lane. Honest §11.4.6 substitution recorded in `23_substitution.txt` + §"Lane decision" below. |

---

## 1. Lane decision — honest substitution (§11.4.6)

- **Design PRIMARY (deferred):** NLLB-200-distilled-600M CT2 int8 behind a bespoke
  LibreTranslate-shaped `/translate` FastAPI shim (`TRANSLATION_PROVIDER.md` §1.2).
  That lane needs a custom image build (python + ctranslate2 + sentencepiece +
  FastAPI + a language detector) **plus** an offline HF→CT2-int8 model conversion
  — a substantial artefact deferred as its own work item.
- **Lane PROVEN here:** stock **LibreTranslate** — the design's own §1.2/§3.1
  **documented fallback lane**: the *identical* LibreTranslate `/translate`
  contract on the *same* CTranslate2 engine the primary would use, delivered as a
  single pre-built CPU image (615 MB), so it boots reliably with zero new shim
  code. The task explicitly sanctions this as "the simplest reliable CPU lane."
- **This is a REAL NMT service** (real Argos/CTranslate2 translation). The
  anti-gaming triple below proves genuine translation — **not** an identity /
  passthrough bluff. A faked/identity-copy PASS is impossible here (proven by the
  RED baseline and the golden-bad self-validation).

---

## 2. What was booted, and how (§11.4.76 / §11.4.161, NO GPU)

- Booted **through the containers-submodule `compose.Orchestrator`** (`digital.vasic.containers/pkg/compose`)
  — NOT ad-hoc `podman`/`docker`. Rootless podman. Compose file
  `harness/compose.phase3translate.yml` carries **no literal** host/port/model/
  language value (§CONST-045/046); every value is env-injected by `run_proof.sh`.
- **NO GPU** in the run spec (no `--device nvidia.com/gpu`, no GPU flag) — the
  structural guarantee this ships before the P0/P1 GPU chain. `compute_type` stays
  CPU (Argos/CT2 CPU kernels).
- Image `docker.io/libretranslate/libretranslate:latest` (615 MB), host port
  **18436** (distinct from coder `:18434`, embeddings `:18435`), container port 5000.
- `LT_LOAD_ONLY=en,fr,de` → `/languages` served exactly `[en, fr, de]`
  (`25_languages.json`), so cold boot fetched only the en/fr/de Argos packages.
- Persistent external Argos-model cache volume `helixllm-libretranslate-cache`
  (§11.4.77 re-obtainable; survives teardown so re-runs skip the download).
- **Boot lessons reused from the embeddings proof:** no `WithForceRecreate`
  (leaves the pod unstarted on this host's podman-compose shim); unique per-run
  project name + pre-clean `boot-down` for a genuinely fresh clean-target; health
  poll fast-fails **only** on container `state=exited` (created/missing keep
  polling); persistent external model-cache volume. Boot became healthy
  (`/languages` → 200) after **11 polls** (`21_health.txt`).

---

## 3. §11.4.108 RUNTIME SIGNATURE — the CX-05 anti-gaming triple (GREEN)

For each pair, the harness drove the REAL service: forward `POST /translate`,
`POST /detect` on the forward text, then a back `POST /translate`, and asserted
**all three** criteria (no single one trusted — the CX-05 anti-gaming construction):

1. **Not-identity + detected-target-language** — `forward ≠ source` AND `/detect(forward) == target`.
2. **Forward adequacy** — `chrF(forward, independent golden reference) ≥ 0.30`.
3. **Back-translation metamorphic** — `chrF(back, source) ≥ 0.40`.

chrF = sacrebleu-style character-n-gram F-score (char_order=6, β=2, whitespace-stripped);
deterministic, reference-based, language-agnostic. Thresholds calibrated on this
project's own fixtures (§11.4.107(13)) — real output clears them with wide margin,
every golden-bad falls far below.

### Captured results (real `/translate` + `/detect` responses)

| Pair | source | forward (real Argos output) | detected | back | fwd chrF (floor 0.30) | back chrF (margin 0.40) | verdict |
|------|--------|-----------------------------|----------|------|----------------------:|------------------------:|---------|
| **en→fr** | "The book is on the table." | "Le livre est sur la table." | `fr` (conf 100) | "The book is on the table." | **1.0000** | **1.0000** | **PASS** |
| **en→de** | "The book is on the table." | "Das Buch liegt auf dem Tisch." | `de` (conf 100) | "The book is on the table." | **0.7519** | **1.0000** | **PASS** |

`notIdentity=true`, `targetMatch=true` for both. Evidence: `11_green_proof_en_fr.txt`,
`11_green_proof_en_de.txt`, raw records `green_record_en_{fr,de}_1.json`.

> The en→de forward "Das Buch **liegt** auf dem Tisch." ("lies") differs from the
> golden "Das Buch **ist** auf dem Tisch." ("is") — both are correct German; chrF
> 0.7519 correctly scores it well above floor. This is genuine calibrated adequacy,
> not a tuned-to-match golden.

## 4. Determinism (§11.4.50) — GREEN

Two byte-for-byte identical requests per pair produced **byte-identical** forward +
back text:
- `[DETERMINISM] PASS ... (en->fr)` — `11_green_proof_en_fr.txt`
- `[DETERMINISM] PASS ... (en->de)` — `11_green_proof_en_de.txt`

## 5. RED baseline (§11.4.115) — defect reproduced, analyzer is not a bluff gate

`10_red_baseline.txt`: the identity-passthrough "warming" bluff (gateway echoes `q`
untranslated — the exact defect design §2.5 forbids) was fed to the analyzer with
`RED_MODE=1` and correctly **FAILED** (exit 1):
```
[RUNTIME-SIGNATURE(en->fr)] FAIL notIdentity=false targetMatch=false fwdChrF=0.2242(floor 0.30) backChrF=1.0000
    reason: identity/passthrough: forward "The book is on the table." == source "..."
    reason: detected language "en" != requested target "fr"
    reason: forward chrF 0.2242 < floor 0.3000
```
Note: back-translation alone (chrF 1.0000) would PASS the identity copy — criteria 1
and 2 are what defeat it. This is the CX-05 anti-gaming guarantee in action.

## 6. Golden-good / golden-bad analyzer self-validation (§11.4.107(10)) — GREEN

`12_self_validation.txt` — golden-good = the REAL captured en→fr record; every
golden-bad variant MUST FAIL (proves the analyzer itself cannot be fooled):

| Fixture | expect | fwd chrF | back chrF | result |
|---------|--------|---------:|----------:|--------|
| GOLDEN-GOOD (real en→fr) | PASS | 1.0000 | 1.0000 | **PASS** ✓ |
| BAD identity/passthrough | FAIL | 0.2242 | 1.0000 | **FAIL** ✓ (crit 1+2) |
| BAD wrong-language (de for fr) | FAIL | 0.0873 | 0.1859 | **FAIL** ✓ (crit 1+2+3) |
| BAD garbage (**with faked** target-match) | FAIL | 0.0500 | 0.0269 | **FAIL** ✓ (crit 2+3 catch it despite faked crit 1) |
| BAD empty | FAIL | 0.0000 | 0.0000 | **FAIL** ✓ (all) |

The garbage fixture deliberately **fakes** `targetMatch=true` to prove the forward-
chrF + back-metamorphic criteria independently catch it — no single criterion is
trusted (the §4.1 paired-mutation guarantee). Floor 0.30 cleanly separates real
(0.75–1.00) from every bad fixture (0.00–0.22).

## 7. Single-owner teardown + coder untouched (§11.4.119)

`29_teardown.txt` / `29b_post_teardown.txt`:
- `DOWN-OK` via the containers-submodule orchestrator; `phase3translate_lt`
  containers removed (`(none — removed)`).
- **`helixllm-coder` — `Up 17 hours` before, during, and after** — never touched.
- A concurrent, unrelated helixllm work-stream container (`llamacpp-router` VLM
  model-download) was observed on the host and left untouched (§11.4.174 —
  ownership verified before any action; never mine to stop).

## 8. Four-layer verification map (§11.4.108)

1. **SOURCE** — `harness/{main.go,compose.phase3translate.yml,run_proof.sh}` committed (route/contract driven).
2. **ARTIFACT** — `01_image_pull.txt` (615 MB image pulled by digest tag); `/languages` = `[en,fr,de]`.
3. **RUNTIME-ON-CLEAN-TARGET** — pre-clean `boot-down` + unique project name ⇒ fresh container; the §3 triple verified against it. **This is the definition of done.**
4. **USER-VISIBLE** — a real client POST to `/translate` returns a correct, right-language translation (`Le livre est sur la table.` / `Das Buch liegt auf dem Tisch.`); `/detect` confirms the target language; the round-trip recovers the source.

## 9. Honest boundary (§11.4.6)

- This proves a **real CPU translation service** passing the anti-gaming triple +
  self-validation for **two** pairs (en→fr, en→de). It does **not** prove the
  design PRIMARY (NLLB-200-CT2 shim) — that lane is deferred (see §1).
- chrF is a character-n-gram adequacy proxy vs a golden reference; it is the
  deterministic automated floor the design §4 mandates (COMET/QE is a later
  additive signal, never a replacement).
- Two independent human golden references were authored **before** observing model
  output; en→fr happened to match exactly (chrF 1.0), en→de scored 0.75 against a
  correct-but-differently-phrased reference — i.e. the floor is a genuine adequacy
  separator, not a tuned-to-model artefact.

## 10. Reproduce

```bash
cd docs/qa/phase3_translation_20260707/harness
./run_proof.sh          # builds harness, pulls image, boots via containers
                        # submodule, proves, tears down single-owner. Regenerates
                        # every evidence file in the parent dir.
```
Config knobs (env, all default-injected): `LT_HOST_PORT` (18436), `LT_LOAD_ONLY`
(en,fr,de), `CHRF_FLOOR` (0.30), `BACK_MARGIN` (0.40), `LT_MEM_LIMIT`, `LT_CPUS`.

## 11. Evidence index

| File | What |
|------|------|
| `00_preflight.txt` | host, coder-untouched, free port, config |
| `01_image_pull.txt` | image pull (ARTIFACT layer) |
| `10_red_baseline.txt` | §11.4.115 RED — identity passthrough FAILs |
| `11_green_proof_en_fr.txt` / `_en_de.txt` | §11.4.108 triple + determinism per pair |
| `12_self_validation.txt` | §11.4.107(10) golden-good/bad analyzer self-validation |
| `13_verdict.txt` | ALL-GREEN |
| `20_boot.txt` / `21_health.txt` | boot via orchestrator + health OK (11 polls) |
| `23_substitution.txt` | honest lane substitution note (§11.4.6) |
| `24_container_state.txt` / `25_languages.json` | running container + served languages |
| `29_teardown.txt` / `29b_post_teardown.txt` | single-owner teardown + coder untouched |
| `30_probe_en_{fr,de}.txt` | raw probe transcripts (source→forward→detect→back) |
| `green_record_en_{fr,de}_{1,2}.json` | raw `/translate`+`/detect` responses (determinism pair) |
| `red_identity_record.json` | the RED identity-passthrough fixture |
| `harness/` | `main.go` (analyzer+client), `compose.phase3translate.yml`, `run_proof.sh`, `go.mod`, `.gitignore` |
