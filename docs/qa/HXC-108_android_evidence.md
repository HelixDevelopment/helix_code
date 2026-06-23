# HXC-108 — Android Client Recording Evidence (§11.4.158 / §11.4.143)

**Run-id:** HXC-108_android · **Date:** 2026-06-23
**Gate host (§11.4.119 single-owner):** thinker.local — Linux 6.17 x86_64, 16 cores,
`/dev/kvm` present, podman 4.9.3 (rootless, §11.4.161).
**Sanctioned path (§6.X):** `helix_code/scripts/run-challenge-matrix.sh` →
Android emulator INSIDE a rootless podman container (image
`localhost/lava-android-emulator:api34-x86_64`); install/launch/screenrecord driven
in-container via `podman exec` / `adb -e` / `podman cp`. No host-direct `emulator`/`adb`.
**Recording prefix (§11.4.155):** `helixcode`. **Window scope (§11.4.154):** the emulator
device frame (1080×1920), not a desktop.

> Durable evidence committed under `docs/qa/HXC-108_android/` per §11.4.83 (NOT the
> rotatable git-ignored raw corpus — the exact §11.4.154-rotation bluff the HXC-107
> audit caught; see `docs/qa/HXC-107_ledger_audit.md`).

---

## Durable artifacts (committed)

| File | What |
|------|------|
| `docs/qa/HXC-108_android/helixcode-android-client.mp4` | The recording — H.264, 1080×1920, 33 frames (ffprobe-verified). |
| `docs/qa/HXC-108_android/helixcode-android-MainActivity.png` | Key frame — the genuine HelixCode Android client. |

Raw run artifacts (also on disk, git-ignored §11.4.128): `/Volumes/T7/Downloads/Recordings/helixcode-android-client-20260623-190817.{mp4,png,logcat.txt}`; on thinker `~/helixcode-recordings/`.

## Content verification (§11.4.158 / §11.4.107 / §11.4.123) — PASS

The recorded frame shows the **real HelixCode Android client MainActivity** running on
Android 14 — NOT a black / boot-animation / crash / stub frame:
- Green **"HelixCode"** title bar (brand).
- The HelixCode spiral logo rendered.
- Status **"Disconnected"** in red — a REAL runtime connection state, not placeholder text.
- A **"User:"** label.
- A green **CONNECT** button.
- Android 14 system UI chrome (status bar clock 4:09, wifi/signal/battery icons).

Corroborating runtime signal (§11.4.108): `dumpsys` reported
`ResumedActivity: dev.helix.code/.MainActivity` (the app is genuinely foregrounded).
APK install reported `Success`; the installed APK md5 (`e85b19af9cdd292b6150bffab2dc277c`)
matched the macOS-built `app-debug.apk` (real native Go core embedded).

The conductor independently re-verified: `ffprobe` confirms h264 1080×1920 / 33 frames,
and visually read the committed PNG (the description above is what is genuinely on screen).

## Blockers hit + resolved during the run (FACTs, §11.4.6)

1. **Rootless `/dev/kvm` denial** — `ProbeKVM` failed (`This user doesn't have permissions
   to use KVM`): image runs as `USER emulator` (uid 1000) → host subuid lacks the `/dev/kvm`
   ACL. Fixed with `--userns=keep-id` (container-uid-1000 → host user who holds the ACL).
   Verified `Boot completed in 29201 ms`.
2. **`podman cp $CN:/sdcard/...` "no such file"** — `/sdcard` is in the emulator GUEST, not
   the container fs. Fixed with a two-hop pull: `adb -e pull` guest→container `/tmp`, then
   `podman cp` container→host.
3. **Intermittent early container exit** — emulator (PID 1) died if a screencap ran first.
   Fixed by record-first ordering. No leftover `helixcode-emu` containers after teardown.

## Submodule gap surfaced (§11.4.74 extend-don't-reimplement) — follow-up

`submodules/containers/pkg/emulator/containerized.go` `buildContainerRunArgs` passes only
`--device /dev/kvm` and lacks `--userns=keep-id`, so its `Containerized.Boot` would hit the
same rootless `ProbeKVM` denial. **Recommended upstream fix:** add `--userns=keep-id` (rootless
podman) in `buildContainerRunArgs`. The wrapper adds the flag itself for now and documents the
gap inline; the submodule was NOT modified (left for conductor/operator decision per §11.4.74).

## Verdict

Android §11.4.158 client recording: **PASS** — real, content-verified, via the §6.X-sanctioned
containerized path on a Linux KVM host. The macOS-podman path remains §11.4.112
structurally-impossible (`docs/research/android_emulator_podman_macos_20260623/feasibility.md`);
this is the research's recommended path [A] realised.
