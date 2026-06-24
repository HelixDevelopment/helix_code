# HelixCode V&V Coverage-Completeness Map (§11.4.118 honesty layer)

| Property | Value |
|---|---|
| Date | 2026-06-24 |
| Server | `http://localhost:18080` (fresh, autoboot PG/Redis/Ollama) |
| Method | READ-ONLY: read `docs/qa/helixcode_vnv_execution_plan.md`, the two Makefiles, `helix_code/qa-results/*` cycle logs, `docs/features/Status.md` |
| Author | coverage-completeness subagent, data-only (§11.4.6 / §11.4.3) |
| Purpose | Make this cycle's V&V boundary EXPLICIT. "No other issues" is only credible with this enumerated set. NOT a claim of completeness. |

This map enumerates what ran, what was honestly SKIP-justified, and what was
left un-attempted (SHOULD-RUN). It cross-references the executed `qa-results`
logs against the full test-type catalogue in the execution plan.

---

## 0. Honesty correction vs the cycle brief (§11.4.6 — data over narrative)

The originating brief described "unit wave (133 ok)" and "constitution sweep
16/16". The captured logs say otherwise — recorded here so the cycle is not
misrepresented as cleaner than it is:

- **Unit wave (`helixcode_unit_wave_20260624_110425.log`):** 133 packages `ok`,
  **but the overall run FAILED** — `dev.helix.code/internal/cognee` FAILs (60.4s),
  and 5 hardware GPU-probe unit tests FAIL on this host:
  `TestGetGPUUsage_ProbeChain_NvidiaPreferred`, `…_FallsBackToAMD`,
  `TestProbeAppleGPU_ParsesIoregOutput`, `…_HandlesMultipleGPUs`,
  `…_HandlesWhitespaceVariants`. The GPU-probe failures are host-hardware
  dependent (no nvidia-smi/rocm; Apple ioreg variant parsing) → should be
  `uname -s`-gated SKIPs per §11.4.81, not hard FAILs. `cognee` FAIL needs a
  §11.4.102 root-cause (it also recurs in the stress/chaos `cognee_*` dirs).
- **Constitution sweep (`constitution-sweep.log`):** **13 PASS / 3 FAIL**, NOT
  16/16. Failing gates are all doc-sync/governance, not product: **G7** (feature
  commits lack `docs/qa/<run-id>/` evidence), **G12** (Issues/Fixed summary docs
  stale), **G14** (`docs_chain verify --all` drift). G16 (challenge matrix) PASS.
- **e2e (phase3) PASS** confirmed (`e2e_20260624_112208.log`, EXIT 0).
- **e2e-challenges FAILED** (`e2e_challenges_…/runner.log`): 0.00% success, 4
  failed + 2 validation-failed, **Total Tokens: 0** — the 0-token figure means
  no real LLM generation tokens flowed through the challenge path (timeout bug
  under fix; treat as a live defect, not a pass).
- **HelixQA api: 169 PASS / 25 SKIP / 18 FAIL** confirmed. NOTE: the
  `helixqa_findings_triage.md` triaged only **7** of the 18 FAILs (2 bank-drift +
  5 `/server/info` exposure gaps). The remaining FAILs include **auth-login 401
  "invalid credentials"** (e.g. HXC-WRK-010) — a server seed/credential mismatch,
  NOT yet triaged; un-triaged FAILs are an open coverage gap inside the bank run.

---

## 1. RAN this cycle (verdict captured)

| Test type | Command | Verdict (from log) | Evidence |
|---|---|---|---|
| Compile / `go vet` | `make verify-compile` / vet | PASS | `go-vet.log`, `helixcode_govet_20260624_110433.log` |
| Unit (hermetic) | `make test` | **133 ok but FAIL** (cognee + 5 GPU-probe) | `helixcode_unit_wave_20260624_110425.log` |
| Verifier unit | `make test-verifier-unit` | ran (PASS) | `verifier-unit.log` |
| Integration | `make test-integration-full` | PASS (incl. IDOR green) | `integration_*_20260624_11*.log` |
| E2E (phase3) | `make test-e2e-full` (phase3) | PASS | `e2e_20260624_112208.log` |
| E2E challenges | `tests/e2e/challenges -all` | **FAIL** (0%, 0 tokens, timeout bug) | `e2e_challenges_20260624_112611/runner.log` |
| HelixQA api banks | `helixqa http --banks …api…` | 169 PASS / 25 SKIP / 18 FAIL | `helixqa_api_20260624_112930/helixqa.log` |
| Constitution sweep | governance sweep | **13/16 PASS** (G7/G12/G14 FAIL) | `constitution-sweep.log` |
| Benchmark sanity | `make test-benchmark` (sanity) | ran | `bench-sanity.log`, `bench-sanity-challenges.log` |
| Challenges anti-bluff | scan + anchors | ran | `challenges-anti-bluff-scan.log`, `…-anchors.log` |
| Stress + chaos | `stress-chaos` (partial) | **PARTIAL** — ~1500 scenario evidence dirs present (`client_http_*`, `cognee_*` etc.) but no captured single suite verdict; `cognee` chaos overlaps the unit FAIL | `qa-results/20260624T09*Z/` dirs |

---

## 2. SKIP-justified (§11.4.3 — honest, missing dep/SDK/credential named)

| Item | Why SKIP-justified | Un-SKIP needs |
|---|---|---|
| Real cloud-LLM generation (OpenAI/Anthropic/Gemini/Groq/Mistral/Cohere/Azure/Bedrock) | `.env.full-test` ships **mock** keys → mock-llm-server; no real wire calls | real API keys in non-mock `.env` |
| Verifier integration (`make test-verifier-integration`) | LLMsVerifier service @ `:8081` not booted in this run | boot verifier service (own submodule — borderline-cheap, see §4) |
| HelixQA mobile/TV banks (`full-qa-android*`, `nexus-mobile-ios`, `capture-android`) | no Android/iOS device/emulator/simulator + ADB/Xcode; iOS has no Xcode project | device/emulator + platform SDK |
| HelixQA desktop banks (`nexus-desktop-macos/windows/linux`) | desktop GUI build + host display required | per-OS desktop binary on a display |
| Mobile client tests (Android/Aurora HAP/Harmony HAP) | device/emulator required | device/emulator |
| Desktop (Fyne) GUI runtime tests | host display server required (CI headless) | display (`desktop-nogui` build is partial proxy) |
| Xiaomi provider challenge (`xiaomi_provider_challenge.go`, `qa-integration/xiaomi_test_bank.yaml`) | real Xiaomi provider credential | real Xiaomi key |
| Security scanners snyk / sonarqube | need `SNYK_TOKEN` / `SONAR_TOKEN` | tokens |
| Sibling-project HelixQA banks (`atmosphere*`, `boba-*`, `openclawing2-*`, `yole-*`, `nexus-*`) | not HelixCode — different SUT | out of scope |

---

## 3. SHOULD-RUN — un-attempted, NOT a justified SKIP (un-exercised but runnable)

These had no missing dependency this cycle; they were simply not run.

| Item | Command | Why it should have run | Cost vs current infra |
|---|---|---|---|
| **Security suite** | `make test-security-full` | core security test type, infra present | CHEAP (server-touching, serial on :18080) |
| **Stress + chaos (complete)** | `make stress-chaos` + `make stress-chaos-infra` | only partial evidence dirs exist, no captured suite verdict | CHEAP (unit hermetic; infra needs PG+Redis, present) |
| **Load / perf** | `make test-load-full` | perf/load test type entirely absent | CHEAP (server-touching, serial) |
| **Coverage (-race)** | `make test-coverage` | no coverage profile captured this cycle | CHEAP (hermetic) |
| **Static security scanners** | `make scan-gosec scan-trivy scan-grype` (root) | open-source, no creds | CHEAP (no infra) |
| **Remaining HelixQA api banks** | `helixqa http --banks <not-yet-run>` | only one api invocation captured; mcp-fs / persistent-memory / protected-subresources / static-routes / cors banks not confirmed run | CHEAP (serial on :18080) |
| **HelixQA web banks** | `helixqa run --platform web --browser-url …` | web client browser flows have ZERO V&V this cycle | BORDERLINE (needs selenium :4444 / chromedp :9222 up + browser-url reaching the server) |
| **Challenges submodule full** | `cd submodules/challenges && make qa-all` (+ `new-capabilities`) | only anti-bluff scan + bench-sanity ran | CHEAP for `qa-all` (hermetic); capability runs need :8080 |
| **Unit FAIL remediation** | fix/SKIP-gate the 5 GPU-probe tests (§11.4.81) + root-cause `cognee` | unit wave is not green | CHEAP (unit-level) |

---

## 4. Unknown-unknown surface — subsystems with NO V&V evidence this cycle

Cross-referencing `docs/features/Status.md` Areas (257 service rows, 114
infrastructure, 134 submodule, 30 cli / 7 tui / 4 web / 6 desktop / 8 mobile)
against what actually ran, these had NO runtime V&V this cycle (at most hermetic
unit coverage, which the §11.4.108 four-layer rule does not count as
runtime-on-clean-target evidence):

1. **Web client (browser flows)** — api-only HelixQA ran; no selenium/chromedp
   browser session drove the web UI.
2. **Desktop (Fyne) GUI runtime** — no run (display gap).
3. **Mobile clients (Android/iOS/Aurora/Harmony)** — no run (device/SDK gap).
4. **LSP / skills / ensemble capabilities** — `internal/lsp`, `internal/skills`,
   `internal/ensemble` packages are ABSENT (per triage); banks assert their
   `*_enabled` flags → these are real gaps with NO working V&V, only FAILing
   assertions. `plugins` + streaming exist but are not advertised in
   `/server/info` (info-exposure gaps).
5. **~65 owned submodules** — of the 134 submodule rows, only `helix_qa` (used as
   the framework) and `challenges` (anti-bluff scan) were exercised. The rest
   (security, helix_agent, helixspecifier, panoptic, containers, docs_chain,
   llms_verifier runtime, etc.) had no V&V dispatched in this cycle.
6. **cli_agents ported capabilities** — Status.md lists a "Ported cli_agents"
   slice; no dedicated port-capability run this cycle.
7. **Many internal services covered only by hermetic unit tests** (no real-infra
   integration/e2e this cycle): e.g. `deployment`, `discovery`, `hardware`
   (GPU-probe FAILing), `focus`, `editor`, `repomap`, `template`, `rules`,
   `hooks`, `version`. Their "working" status rests on unit tests, not runtime
   evidence.
8. **Real provider matrix** — only Ollama (local) + mock-llm-server exercised;
   the CONST-039 "all providers" surface (OpenAI/Anthropic/Gemini/DeepSeek/Groq/
   Mistral/xAI/OpenRouter/Llama.cpp) has no real-wire V&V (mock keys).

Honest boundary (§11.4.6): this list reduces the unknown-unknown surface; it does
NOT prove the un-exercised subsystems work. They are honest coverage gaps, not
implied-clean.

---

## 5. Prioritized "cheaply-runnable-now" candidates (additional V&V wave)

Against the CURRENT booted infra (`:18080` + PG/Redis/Ollama, no new deps),
serialized on the single server per §11.4.119:

1. **`make test-security-full`** — highest value, server-touching, serial.
2. **`make stress-chaos` then `make stress-chaos-infra`** — complete the partial
   run + capture a single suite verdict (and re-confirm the `cognee` chaos vs
   unit FAIL relationship — likely one root cause, §11.4.102).
3. **`make test-load-full` + `make test-benchmark`** — fills the entirely-absent
   perf/load type; load is serial, benchmark hermetic [bg].
4. **Remaining HelixQA api banks** in ONE `helixqa http` invocation (the api
   banks not yet confirmed run) — serial on :18080.
5. **`cd submodules/challenges && make qa-all`** — hermetic, parallel-safe [bg].
6. **Root static scanners `make scan-gosec scan-trivy scan-grype`** — no infra,
   parallel-safe [bg].
7. **`make test-coverage`** — hermetic [bg].
8. **Unit-level fixes:** `uname -s`-gate the 5 GPU-probe tests (§11.4.81) and
   root-cause the `cognee` unit FAIL — cheap, makes the unit wave green.

**Genuinely blocked (stay §11.4.3 SKIP this wave):** real cloud-LLM generation
(no keys), mobile/desktop client tests (devices/SDK/display), Xiaomi (real key),
snyk/sonarqube (tokens). **Borderline (cheap only if their service/containers are
booted):** verifier-integration (`:8081`, own submodule — boot it to un-SKIP) and
HelixQA web banks (selenium :4444 / chromedp :9222 + browser-url reaching the
server on the host-remapped :18080).
