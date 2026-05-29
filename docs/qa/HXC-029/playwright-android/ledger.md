# HXC-029 §11.4.98 — Playwright (browser) + Android Banks Ledger

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-05-29 |
| Last modified | 2026-05-29 |
| Status | active |
| Tracked item | HXC-029 (§11.4.98 full-automation anti-bluff forward sweep) |
| Batch | Playwright (browser) + Android banks in `helix_qa/banks/` |
| Pass type | **SCOPE classification + live web-UI surface probe + read-only adb device probe. NO real device driven. NO bank source modified.** |
| Drivers present | chromium + `~/.local/bin/playwright` + cached chromium-1217; adb 37.0.0 + 2 connected devices |

## Headline result

**ZERO of the Playwright/Android banks in this batch are HelixCode-scope.** Every
browser bank and every Android bank targets a DIFFERENT project (Catalogizer,
Yole) or HelixQA's OWN internal automation engine (`pkg/nexus`, `pkg/capture`) —
not the HelixCode platform. HelixCode itself is **API-only** (no web UI, no
Android app), proven by live route probing. Therefore there is **no in-scope
HelixCode Playwright surface to convert** and **no in-scope HelixCode Android app
to drive**. Honest classification per §11.4.6 (no-guessing) and §11.4.90 — NOT a
fabricated PASS.

## Evidence: HelixCode serves NO web UI (Playwright has no in-scope target)

`docs/qa/HXC-029/playwright-android/helixcode-webui-probe.txt` (live, server up):

```
GET /                 -> HTTP 404  text/plain
GET /api/v1/health    -> HTTP 200  application/json
GET /health           -> HTTP 200  application/json
GET /dashboard        -> HTTP 404  text/plain
GET /ui               -> HTTP 404  text/plain
GET /login            -> HTTP 404  text/plain
GET /index.html       -> HTTP 404  text/plain
GET /static/          -> HTTP 404  (empty)
GET /web              -> HTTP 404  text/plain
GET /metrics          -> HTTP 404  text/plain
```

Only the two JSON health endpoints answer 200. There is no HTML/dashboard/login
surface. Playwright (chromium driver present + cached) has nothing HelixCode-side
to drive — running the browser banks would exercise Catalogizer's React frontend
(`localhost:3000`) or HelixQA's own Go engine, both out of HXC-029 scope.

## Evidence: read-only adb device probe (Android — classification only)

`docs/qa/HXC-029/playwright-android/android-driver-probe.txt`:

```
adb version 1.0.41 / 37.0.0  (driver present)
2 devices: 66ff9c4f51f00ee7 + 998fd36615e99484  -> model:ATMOSphere product:shiba
```

The 2 physical devices are **ATMOSphere** (shiba) hardware — the Arvus/ATMOSphere
project, NOT HelixCode. No `install` / `am start` / `settings put` / `input` was
issued (only `adb devices`). Per the batch safety rule, real devices are NOT
auto-driven; in-scope Android work (there is none here) would be
`operator_attended`.

## Per-bank ledger

Scope grep evidence: `docs/qa/HXC-029/playwright-android/bank-scope-evidence.txt`.

| bank | scope | driver | classification | evidence-path |
|---|---|---|---|---|
| `full-qa-web.json/.yaml` | **other-project (Catalogizer)** | playwright | out-of-scope (targets Catalogizer React @ `localhost:3000`) | `bank-scope-evidence.txt` (meta `app: "Catalogizer"`, 22× `localhost:3000`) |
| `fixes-validation-browser.yaml` | **other (HelixQA own engine)** | go-build/test | out-of-scope (tests HelixQA's `pkg/nexus/browser` engine, target `catalogizer.local`) | `bank-scope-evidence.txt` (6× `pkg/nexus`) |
| `nexus-browser.json/.yaml` | **other (HelixQA own engine)** | go-build/test | out-of-scope (`feature: helix-nexus-browser`; `go build ./pkg/nexus/...`) | `bank-scope-evidence.txt` (`helix-nexus`, `pkg/nexus`) |
| `full-qa-android.json/.yaml` | **other-project (Catalogizer)** | adb/uiauto | out-of-scope (Catalogizer Android phone app) | `bank-scope-evidence.txt` (meta `app: "Catalogizer"`) |
| `full-qa-androidtv.json/.yaml` | **other-project (Catalogizer)** | adb/DPAD | out-of-scope (`com.catalogizer.androidtv` on Xiaomi Mi Box; `localhost:8080` is the user-typed *Catalogizer backend* URL field) | `bank-scope-evidence.txt` (55× `com.catalogizer.androidtv`) |
| `full-qa-androidtv-challenges.json` | **other-project (Catalogizer)** | adb/DPAD | out-of-scope (Catalogizer Android TV challenge mirror) | `bank-scope-evidence.txt` (Catalogizer, Mi Box, Xiaomi) |
| `validation-androidtv-focus.json` | **other-project (Catalogizer)** | adb/DPAD | out-of-scope (Catalogizer AndroidTV channels/deep-links) | `bank-scope-evidence.txt` (meta `app: "Catalogizer"`) |
| `file-browser.yaml` | **other-project (Yole)** | adb/uiauto + browser | out-of-scope (Yole file browser GUI) | `bank-scope-evidence.txt` (meta `app: "Yole"`) |
| `app-navigation.yaml` | **other-project (Yole)** | adb/uiauto | out-of-scope (Yole navigation) | `bank-scope-evidence.txt` (meta `app: "Yole"`) |
| `editor-operations.yaml` | **other-project (Yole)** | adb/uiauto | out-of-scope (Yole editor GUI) | `bank-scope-evidence.txt` (meta `app: "Yole"`) |
| `all-formats.yaml` | **other-project (Yole)** | adb/uiauto | out-of-scope (Yole editor formats) | `bank-scope-evidence.txt` (meta `app: "Yole"`) |
| `cloud-storage-operations.yaml` | **other-project (Yole)** | adb/uiauto | out-of-scope (Yole cloud storage GUI) | `bank-scope-evidence.txt` (meta `app: "Yole"`) |
| `storage-configuration.yaml` | **other-project (Yole)** | adb/uiauto | out-of-scope (Yole storage config GUI) | `bank-scope-evidence.txt` (meta `app: "Yole"`) |
| `edge-cases-stress.yaml` | **other-project (Yole)** | adb/uiauto | out-of-scope (Yole GUI stress) | `bank-scope-evidence.txt` (24× Yole) |
| `nexus-mobile-android.yaml` | **other (HelixQA own engine)** | go/appium | out-of-scope (`feature: helix-nexus-mobile-android`; Appium engine) | `bank-scope-evidence.txt` (`helix-nexus`) |
| `fixes-validation-mobile.yaml` | **other (HelixQA own engine)** | go/appium | out-of-scope (HelixQA `pkg/nexus` mobile regressions) | `bank-scope-evidence.txt` (4× `pkg/nexus`) |
| `image-quality-gate.yaml` | **other-project (Catalogizer)** | adb/screencap | out-of-scope (Catalogizer AndroidTV image-quality) | `bank-scope-evidence.txt` (meta `app: "Catalogizer"`, androidtv) |
| `full-qa-cross-platform.yaml` | **other-project (Catalogizer)** | playwright/adb | out-of-scope (Catalogizer cross-platform login flow) | `bank-scope-evidence.txt` (meta `app: "Catalogizer"`) |
| `capture-android.yaml` | **other (HelixQA own device-capture)** | adb/scrcpy | out-of-scope (HelixQA `pkg/capture/android` device capture — Arvus/ATMOSphere surface) | `bank-scope-evidence.txt` (11× `pkg/capture`) |
| `capture-linux.yaml` | **other (HelixQA own device-capture)** | x11/portal | out-of-scope (HelixQA `pkg/capture/linux` desktop capture) | `bank-scope-evidence.txt` (11× `pkg/capture`) |

## Counts

| Metric | Count |
|---|---|
| Banks examined (browser + android + capture) | 20 |
| **HelixCode-scope** | **0** |
| Other-project / other-component (out of HXC-029 scope) | 20 |
| Playwright verified-3x (HelixCode) | 0 (no HelixCode web UI exists) |
| Android operator-attended (HelixCode) | 0 (no HelixCode Android app exists) |
| Obsolete | 0 (banks are valid — for THEIR own projects) |
| Out-of-scope | 20 |

Out-of-scope breakdown: Catalogizer = 8 (full-qa-web, full-qa-android,
full-qa-androidtv, full-qa-androidtv-challenges, validation-androidtv-focus,
image-quality-gate, full-qa-cross-platform); Yole = 7 (file-browser,
app-navigation, editor-operations, all-formats, cloud-storage-operations,
storage-configuration, edge-cases-stress); HelixQA own engine/capture = 5
(fixes-validation-browser, nexus-browser, nexus-mobile-android,
fixes-validation-mobile, capture-android, capture-linux — note nexus-browser +
nexus-mobile share the engine).

## Anti-bluff posture

Per §11.4.6 (no-guessing) and §11.4.51/§11.4.79 (HelixQA banks stay decoupled,
project-not-aware): these banks are NOT broken and NOT obsolete — they correctly
validate Catalogizer / Yole / HelixQA's own engine. Converting them to target
HelixCode would be incorrect (it would couple a reusable QA bank to a specific
consuming project) AND out of HXC-029 scope. The honest classification is
**out-of-scope for HXC-029**, with the underlying reason captured live: HelixCode
has no browser UI and no Android app for these driver classes to target.

## What genuinely remains for HXC-029

Nothing automatable in THIS batch. The HelixCode-relevant automatable surface
(API + CLI banks) was handled by the prior 7/18 verified banks. The
`manual-review-required` prose steps catalogued in `bank-classification.md`
(server-API + CLI sub-groups) are the remaining HelixCode-scope conversion work
and belong to the API/CLI batches, not this one. This Playwright/Android batch
**concludes with no further automatable HelixCode work** — driving the real
ATMOSphere devices or other-project apps is explicitly out of scope and would be
operator-attended for those projects.

## Sources verified 2026-05-29
- Live HelixCode server route probe (`localhost:8080`) — captured in `helixcode-webui-probe.txt`
- `helix_qa/banks/*.{yaml,json}` metadata + target greps — captured in `bank-scope-evidence.txt`
- `adb devices -l` (read-only) — captured in `android-driver-probe.txt`
