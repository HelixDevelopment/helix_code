# HXC-108 — HelixCode iOS App Fully-Automatic Simulator Recording Evidence

| | |
|---|---|
| Revision | 1 |
| Created | 2026-06-23 |
| Last modified | 2026-06-23 |
| Status | active |
| Status summary | PASS — app built, launched, recorded, OCR-validated on iOS Simulator |
| Feature class | §11.4.69 `video_display` (UI render) + §11.4.158 intensive recording coverage |

## Table of contents

- [Summary](#summary)
- [Environment (captured)](#environment-captured)
- [Buildability](#buildability)
- [Build-blocker root cause and reversible fix](#build-blocker-root-cause-and-reversible-fix)
- [Recording + OCR validation](#recording--ocr-validation)
- [Analyzer self-validation](#analyzer-self-validation)
- [Cleanup / reversibility](#cleanup--reversibility)
- [Verdict](#verdict)

## Summary

The HelixCode iOS app (`helix_code/applications/ios/HelixCode.xcodeproj`, scheme
`HelixCode`) was built, installed, launched, and screen-recorded **fully
automatically** on the iOS Simulator — no TCC grant, no human interaction. The
recording and screenshots show the **real** app UI rendering **live data from
the embedded Go core** (`HelixCore.xcframework` over cgo): `Go core OK — themes:
3, tasks: 2` plus two tasks seeded through the core. Content was verified by
macOS Vision OCR, and the OCR analyzer was self-validated (golden-good accepted,
golden-bad black frame rejected) per §11.4.107(10).

This was NOT a clean first build — a real CoreSimulator/environment blocker had
to be root-caused and surmounted with a reversible host-side fix (documented
below). No step was faked.

## Environment (captured)

- Xcode 16.4 (Build 16F6); iOS Simulator SDK 18.5.
- Simulator: **iPhone 16, iOS 18.5** (UDID `DC8B1905-F2E5-425D-A2DE-42C27B390AC5`).
- Host: arm64 (Apple Silicon). `HelixCore.xcframework` ships an
  `ios-arm64_x86_64-simulator` slice (arch `x86_64 arm64`) — compatible.
- Project: single scheme `HelixCode`; `CODE_SIGNING_ALLOWED=NO`; bundle id
  `dev.helix.code.ios`; embeds `HelixCore.xcframework` (the real Go mobile core).

## Buildability

**PASS.** Build command (final, succeeding):

```
cd helix_code/applications/ios && \
xcodebuild -project HelixCode.xcodeproj -scheme HelixCode -configuration Debug \
  -destination 'platform=iOS Simulator,id=DC8B1905-F2E5-425D-A2DE-42C27B390AC5' \
  -derivedDataPath /tmp/helixcode_ios_dd CODE_SIGNING_ALLOWED=NO build
# => ** BUILD SUCCEEDED **  (EXIT=0)
```

Produced `.app` (real, well-formed):

- `HelixCode` — Mach-O 64-bit executable arm64 (simulator slice).
- `Frameworks/HelixCore.framework` embedded.
- `HelixCode.debug.dylib` 51.5 MB (the substantial Go core — not a stub).
- `Assets.car` (284 KB), `Info.plist` (`CFBundleIdentifier=dev.helix.code.ios`).

Swift compilation, the gomobile Go-core link, and `__preview.dylib` all
succeeded on the very first attempt; only the asset-catalog stage failed
initially (see below).

## Build-blocker root cause and reversible fix

**Initial failure (EXIT=65):** `actool` (asset-catalog `CompileAssetCatalogVariant
thinned`) could not create its transient `IBSimDeviceTypeiPad3x` render device for
iOS 18.5 — "Device was allocated but was stuck in creation state."

**Root cause (FACT, from `~/Library/Logs/CoreSimulator/CoreSimulator.log`):**

```
Error copying sample content to path
  /Volumes/T7/Library-Developer-Xcode/UserData/IB Support/Simulator Devices/<uuid>/data :
  NSCocoaErrorDomain Code=513 "You don't have permission to save the file ..."
  NSUnderlyingError ... NSPOSIXErrorDomain Code=1 "Operation not permitted"
```

- `~/Library/Developer/Xcode` is a symlink to `/Volumes/T7/Library-Developer-Xcode`,
  so the IB device set physically lives on the **T7 external volume**, mounted
  `hfs, local, noowners` (`mount | grep /Volumes/T7`).
- Under `noowners`, the sandboxed `CoreSimulatorService` daemon cannot create the
  device's `data` dir (`Operation not permitted`), even though the shell user can
  write there. A normal `simctl create` into the *default* device set
  (`~/Library/Developer/CoreSimulator/Devices`, internal disk) succeeds — proving
  the failure is specific to the relocated IB set on the noowners volume.
- The project's `TARGETED_DEVICE_FAMILY="1,2"` forces actool to need an iPad-3x
  render device; `TARGETED_DEVICE_FAMILY=1`, `IBSC_SIMULATOR_DEVICE_SET_PATH`, and
  `ASSETCATALOG_COMPILER_SKIP_APP_STORE_DEPLOYMENT=YES` overrides were all tried
  and did **not** bypass it (the path is baked into actool's default device-set).

**Reversible fix applied:** relocated the IB `Simulator Devices` directory off the
`noowners` T7 volume onto an internal owners-enforced path
(`/System/Volumes/Data/...`, daemon-writable) via a symlink, preserving the
original as `Simulator Devices.t7bak`. Rebuild then **succeeded**. Fix fully
reverted afterward (symlink removed, original restored — see Cleanup). This is a
**host-environment** blocker, not a HelixCode source defect.

## Recording + OCR validation

- Recording: `helixcode-ios-launch-render-20260623-141821.mp4`
  - `/Volumes/T7/Downloads/Recordings/` (raw corpus, gitignored per §11.4.128).
  - md5 `e565c6bde216926cbb3d21990bfd6c5d`; H.264; 1178x2556; **144 frames**;
    **11.945 s**; 3.17 MB (via `simctl io recordVideo` — window-scoped to the
    simulator device, no desktop, §11.4.159(A)).
  - §11.4.155 project-name prefix `helixcode-ios-` (from lowercased project root).
- Curated still: `helixcode-ios-launch-render-20260623-141821.png`.

**Liveness (§11.4.107):** per-frame avg luma varies across the timeline
(44 → 118 light-appearance toggle → 78 → ~16 relaunch transition → back to 44);
6+ distinct frame hashes — genuine motion, NOT a frozen/black frame.

**Content (macOS Vision OCR — robust on dark UI where tesseract returned empty):**
9 of 12 sampled recording frames (f_001–f_008, f_012) contain the full app UI;
f_009–f_011 are the relaunch black-launch-screen transition (matching the luma
dip). OCR of an app frame / screenshot yields, verbatim:

```
HelixCode
HelixCode iOS
Disconnected
User: (none)
Go core OK - themes: 3, tasks: 2
Connect
Build iOS client - created
Wire Go core — created
```

These are §11.4.2 real outputs — `Go core OK — themes: 3, tasks: 2` and the two
task rows are **live data from the Go core** (`getAvailableThemes()`,
`getTasks()`, `createTask()`), not hardcoded/simulated. `Disconnected` is correct
(no server runs in the simulator). No "simulated"/"placeholder"/"TODO" text.

All §11.4.159(K) expected patterns present: `HelixCode iOS`, `Disconnected`,
`Go core OK - themes: 3, tasks: 2`, `Connect`, `Build iOS client`, `Wire Go core`.

## Durable evidence (committed) — rotation-proof anchors (§11.4.83, HXC-108 audit F2 fix)

The raw corpus (`/Volumes/T7/Downloads/Recordings/`, §11.4.128/.154-rotatable) is the
secondary location. The load-bearing key still frame is **copied into the committed
tree**; the 3.17 MB MP4 exceeds the per-byte commit budget, so its identity is pinned
by a committed ffprobe+md5 stamp (the still frame is the durable visual evidence):

| Committed artifact | sha256 | raw-corpus md5 (verified byte-identical pre-copy) |
|---|---|---|
| `docs/qa/HXC-108_ios/helixcode-ios-launch-render-20260623-141821.png` | `4467408ec80b9097a7f7461915f32046a9eb2153367354cae1093b50e8ac22b3` | — |
| `docs/qa/HXC-108_ios/helixcode-ios-launch-render-20260623-141821.mp4.ffprobe-stamp.txt` (pins the raw MP4: h264 1178×2556 144f 11.945s, md5 `e565c6bde216926cbb3d21990bfd6c5d`) | (text stamp) | `e565c6bde216926cbb3d21990bfd6c5d` |

## Analyzer self-validation

Per §11.4.107(10) — the OCR analyzer provably cannot bluff:

- golden-bad (synthetic black frame) → `HelixCode iOS` matches = **0** (rejected).
- golden-good (app screenshot) → `HelixCode iOS` matches = **1** (accepted).
- Verdict: **ANALYZER SELF-VALIDATION PASS.**

## Cleanup / reversibility

- App terminated; simulator `DC8B1905-...` powered off — restored to its
  pre-run `Shutdown` state (§11.4.14; we booted it, so we shut it down). Other
  simulators untouched.
- IB `Simulator Devices` relocation fully reverted: symlink removed, original
  T7 directory restored from `.t7bak`.
- Raw recording corpus under `/Volumes/T7/Downloads/Recordings/` is gitignored
  per §11.4.128; this evidence doc is the curated committable record (§11.4.83).
- No commit/push performed — returned for conductor review (§11.4.142).

## Verdict

**PASS — fully-automatic iOS Simulator build + launch + recording achieved with
captured, OCR-validated, self-validated evidence of the real HelixCode iOS app
rendering live Go-core data.** The only obstacle was a host-environment
CoreSimulator permission blocker (noowners external volume), root-caused and
surmounted reversibly — not a code defect, not faked.
