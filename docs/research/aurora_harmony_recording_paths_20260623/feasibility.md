# Aurora-OS / Harmony-OS client recording — autonomous-path feasibility (HXC-108 residual)

**Date:** 2026-06-23
**Author:** research subagent (read-only + web research)
**Mandate:** §11.4.150 multi-angle deep research BEFORE classifying the Aurora/Harmony
recording gaps. Determine whether ANY autonomous path exists to record the HelixCode
Aurora-OS and Harmony-OS Fyne clients genuinely running, or whether they are genuine
§11.4.98(B) operator-bootstrap blockers (and not §11.4.112 structurally-impossible).

---

## Confirmed host / repo facts (FACT — captured this session)

- **macOS host:** `arm64`, **Apple M3 Pro**, Apple Silicon (`hw.optional.arm64=1`). Darwin 24.5.
- **macOS toolchains ABSENT:** `sfdk mb2 sb2 rpmbuild` (Aurora) and `hdc ohpm deveco hvigorw`
  (Harmony) all NOT FOUND on this host. (Mission-confirmed; re-verified.)
- **thinker.local (FACT, probed read-only this session):** reachable, `Linux thinker
  6.17.0-23-generic … Ubuntu 24.04 … x86_64` (KVM host). **NONE** of `sfdk mb2 sb2
  rpmbuild hdc ohpm deveco hvigorw` are installed there either (each `NOTFOUND`).
  → Neither SDK happens to be pre-installed on the Linux box. (Another agent owns thinker
  for Android; only a single read-only `command -v` probe was run.)
- **The apps are Fyne Go clients (FACT):** `applications/aurora_os` and
  `applications/harmony_os` both import `fyne.io` (in `main.go` + `theme.go`). Build
  targets exist: `make aurora-os → bin/aurora-os`, `make harmony-os → bin/harmony-os`
  (Makefile L599–609). They cross-compile but need the vendor SDK + a target
  device/emulator to RUN.

---

## Angle 1 — Aurora OS SDK on Linux + emulator headless feasibility

**FACT.** Aurora OS (ОС Аврора, Open Mobile Platform / OMP — a Sailfish-derived Russian
mobile OS) ships an officially **Linux-installable SDK** ("Аврора SDK", incl. the MB2
build engine). Evidence:
- Linux download page exists: `developer.auroraos.ru/downloads/sdk_mb2/5.1.3.85/linux`
  (also Windows + macOS variants).
- The SDK installs **two VirtualBox VMs** — a **Build Engine** VM (SSH-driven, host-OS-
  independent builds via `mb2 -t AuroraOS-armv7hl build` / `AuroraOS-i486`) and an
  **Emulator** VM (runs the app "similar to a real device").
- Host requirements: Linux/Win/macOS, ~5 GB disk, 4 GB RAM, hardware virtualization
  recommended, **Oracle VM VirtualBox required and pre-installed**.
- There is an emulator-management doc ("Управление эмуляторами") and an `sfdk`-class CLI;
  a community `aurora-cli` (keygenqt) wraps SDK/emulator ops, and `keygenqt/aurora-m1`
  documents running the Aurora emulator on Apple Silicon via **VirtualBox + VNC**.

**Headless / autonomy assessment (FACT + UNCONFIRMED):**
- The emulator is a **GUI VirtualBox VM**, not a headless CLI surface. Capturing the
  client running means screen-capturing the emulator window (a §11.4.117 pixel/OCR
  oracle), not reading a terminal. (FACT: it is a VM with a graphical framebuffer.)
- Running it requires **VirtualBox on the host**. On the **Apple-Silicon macOS host this
  is a hard blocker** — VirtualBox on Apple Silicon is experimental/unreliable; the
  community path (aurora-m1) is non-trivial bootstrap. (FACT: VirtualBox is x86-oriented;
  UNCONFIRMED whether a working Apple-Silicon VirtualBox + Aurora emulator combo is
  reproducible autonomously.)
- On **thinker (Linux x86_64)** VirtualBox could host the VMs, BUT it is a **KVM host** —
  running VirtualBox VMs alongside/under KVM hits nested-virtualization contention, and
  the box is **owned by the Android agent** (§11.4.119 single-resource-owner). (UNCONFIRMED
  that VirtualBox + Aurora emulator runs cleanly there without disturbing Android work.)
- **Access gate:** the SDK lives behind the Russian developer portal
  `developer.auroraos.ru`; download/registration may be **geo/account-gated** (§11.4.3
  `geo_restricted` risk — UNCONFIRMED whether anonymous download works from this locale).

Sources: developer.auroraos.ru SDK MB2 Linux download + setup_linux + manage_emulators
docs; omp.ru/sdk; keygenqt aurora-cli + aurora-m1 (URLs in footer).

## Angle 2 — Harmony OS / DevEco on Linux + emulator path

**FACT.** **DevEco Studio (the IDE) is Windows + macOS ONLY — no Linux build.** Linux gets
only **command-line tools** (`hvigor`, `hdc`) for build/sign/deploy — not an emulator.
- **hdc** (HarmonyOS Device Connector) runs on Win/Linux/macOS — but it only *connects to*
  a device/emulator; it does not *provide* one.
- **The HarmonyOS NEXT emulator does NOT run on Linux**, and one authoritative-community
  source states it "only runs on Windows and won't work through ANY VM" (needs Hyper-V +
  12 GB RAM). (FACT for Windows; see contradiction below for macOS.)
- **Conflicting-but-favourable data (FACT, latest source):** DevEco Studio **5.0.3 Beta2
  (API 15)** for **macOS** ships a **"100% functional" emulator with Apple-Silicon support**
  (Rosetta-assisted). The official site historically gave **no emulator for Intel Macs** —
  so the macOS emulator path is **Apple-Silicon-specific**. *This host is Apple M3 Pro*,
  so the macOS-Apple-Silicon emulator is technically applicable here.
- **Hard credential gate:** downloading the Command-Line Tools AND the SDK **requires a
  registered + verified Huawei Developer account login**; signing-key generation also
  requires that account. (FACT — credential bootstrap, §11.4.10 / §11.4.98(B) exception.
  Account verification is frequently China-region-gated — §11.4.3 `geo_restricted` risk,
  UNCONFIRMED for this locale.)

**Autonomy assessment:** even on the favourable macOS-Apple-Silicon path, RUN requires:
(1) installing the **GUI DevEco Studio** + SDK (absent here), (2) a **verified Huawei
account login** (operator credential, possibly geo-gated), (3) launching the emulator
**through the GUI IDE** (not a headless CLI), then (4) a §11.4.117 pixel/OCR window capture
of the Fyne client running on it. None of these are blocked by platform *design* — they
are toolchain-install + vendor-account + GUI-driving gates.

Sources: developer.huawei.com DevEco docs (ide-tools-overview, ide-software-install);
harmony-developers.com (5.0.3 Beta2 macOS emulator); device.harmonyos.com local-emulator
FAQ; Servo Book OpenHarmony page; XDA NEXT-emulator thread; Wikipedia DevEco Studio (URLs
in footer).

## Angle 3 — emulator-in-container precedents

**FACT/UNCONFIRMED.** Android has the well-known `android-emulator-container-scripts`
(headless emulator in a container, ADB/gRPC + VNC/WebRTC streaming) — the clean precedent
the Android agent leverages. **No equivalent first-class container-emulator exists for
either Aurora or Harmony:**
- Aurora's emulator is a **VirtualBox VM** — containerising it means nested virt (heavy,
  fragile); no published turnkey "Aurora emulator in a container" project found (the
  closest is the VirtualBox+VNC manual path).
- Harmony's NEXT emulator is reported to **refuse ANY VM** (Windows/Hyper-V case) and is
  otherwise the macOS-Apple-Silicon native emulator inside the GUI IDE — **no container
  path found**. (FACT: no precedent surfaced in search; UNCONFIRMED that none exists.)

## Angle 4 — Is the Fyne Go client on a generic desktop a legitimate §11.4.158 proxy?

**Honest boundary (FACT — NO).** The HelixCode Aurora/Harmony clients are **Fyne** apps;
`make desktop` builds the *same shared Fyne UI* for Linux/macOS. Recording that desktop
build would prove the **shared Fyne UI logic renders + the LLM-core wiring works**, but it
is a **different artifact running on a different OS** — it does **NOT** exercise the Aurora
or Harmony **native target** (different cross-compile target, vendor windowing/IME, platform
integration). Per **§11.4.143** (real user journey on the real target) and **§11.4.158**
(every feature recorded genuinely working on its platform), a Fyne-on-Linux/macOS recording
is **at most a supplementary UI smoke test, NOT a substitute** for "the Aurora/Harmony
client genuinely running." Claiming it as the platform recording would be a §11.4 PASS-bluff
at the platform-target layer. (This is the same honest boundary the mission flags.)

## Angle 5 — thinker.local SDK read-probe

**FACT (this session):** `ssh thinker.local 'command -v sfdk mb2 sb2 rpmbuild hdc ohpm
deveco hvigorw'` → every tool `NOTFOUND`. thinker is a clean Ubuntu 24.04 x86_64 KVM host
with neither SDK installed. No autonomous shortcut via a pre-provisioned Linux box.

---

## Per-platform VERDICT

### Aurora OS → **operator-bootstrap-required (§11.4.98(B))** — NOT structurally-impossible
A real autonomous path is *technically* describable (Linux x86_64 + VirtualBox + Aurora SDK
→ emulator VM → §11.4.117 pixel/OCR capture), so **§11.4.112 does NOT apply**. But every
prerequisite is an operator-bootstrap / credential / resource gate, none reachable
autonomously from the current state:
1. Install Aurora SDK (absent on both macOS host and thinker) + **VirtualBox**.
2. SDK is behind the OMP/`developer.auroraos.ru` portal — likely **account/geo-gated**
   (§11.4.3 risk, UNCONFIRMED).
3. VirtualBox on **Apple Silicon** = unreliable; on **thinker** = nested-virt under KVM +
   the box is **owned by the Android agent** (§11.4.119) → not an available resource now.

**Recommended classification:** `Operator-blocked` (§11.4.21) with enumerated unblock
choices (§11.4.148-D3): **[A]** operator provisions a dedicated **x86_64 Linux box with
VirtualBox + Aurora SDK** (account supplied) → agent then drives emulator + §11.4.117
window-scoped recording; **[B]** operator provides Aurora SDK creds + authorises shared use
of an x86_64 host → same; **[C]** defer as documented §11.4.98(B) operator-bootstrap gap
with this research as the cited evidence the path was exhausted.

### Harmony OS → **operator-bootstrap-required (§11.4.98(B))** — NOT structurally-impossible
The macOS-Apple-Silicon DevEco emulator (5.0.3 Beta2+) means a RUN path **does exist on a
host of this exact class** (Apple M3 Pro), so **§11.4.112 does NOT apply**. Blockers are
bootstrap/credential/GUI gates:
1. Install **GUI DevEco Studio + SDK** (absent here; no Linux IDE exists).
2. **Verified Huawei Developer account login** required for SDK + signing + emulator
   (operator credential, §11.4.10; verification often China-region-gated — §11.4.3 risk,
   UNCONFIRMED for this locale).
3. Emulator launches **through the GUI IDE** → drive + §11.4.117 pixel/OCR window capture.

**Recommended classification:** `Operator-blocked` (§11.4.21) with unblock choices:
**[A]** operator installs DevEco Studio on this Apple-Silicon host + completes Huawei
verified-account login → agent drives the IDE emulator + window-scoped recording (§11.4.159);
**[B]** operator supplies a physical HarmonyOS device + Huawei account → `hdc`-driven on
real hardware; **[C]** defer as documented §11.4.98(B) gap with this research cited.

---

## Honest recommendation

Neither platform is **structurally-impossible** (§11.4.112 is **NOT** warranted — a credible
autonomous run path exists for each once the toolchain + vendor account are present), and
neither has an autonomous path **available from the current host state** (SDKs absent
everywhere probed; vendor-account + GUI-IDE/VirtualBox + possible geo gates). They are
genuine **§11.4.98(B) operator-bootstrap blockers**. The §11.4.150/§11.4.8 deep-research
path is hereby **exhausted and documented**; the correct closure is **`Operator-blocked`
(§11.4.21)** per platform with the enumerated unblock choices above — **not** a faked
recording, **not** a Fyne-on-desktop proxy passed off as the native target (§11.4.143/
§11.4.158 forbid it), and **not** a `structurally-impossible` won't-fix. Residual
UNCONFIRMED items an operator can resolve cheaply: (i) anonymous vs account/geo-gated Aurora
SDK download from this locale; (ii) Huawei verified-account availability/region from this
locale; (iii) reproducibility of VirtualBox+Aurora-emulator on Apple Silicon vs an x86_64
Linux box.

## Sources verified 2026-06-23
- Aurora SDK MB2 Linux download — https://developer.auroraos.ru/downloads/sdk_mb2/5.1.3.85/linux
- Aurora SDK Linux setup — https://developer.auroraos.ru/doc/5.2.0/sdk/app_development/setup/setup_linux
- Aurora emulator management — https://developer.auroraos.ru/doc/sdk/app_development/setup/manage_emulators
- Aurora Scratchbox2 / mb2 — https://developer.auroraos.ru/doc/sdk/tools/scratchbox2
- OMP Aurora SDK — https://www.omp.ru/sdk
- Aurora CLI (keygenqt) — https://keygenqt.github.io/aurora-cli/
- Aurora on Apple Silicon (VirtualBox+VNC) — https://github.com/keygenqt/aurora-m1
- HUAWEI DevEco tools overview — https://developer.huawei.com/consumer/en/doc/harmonyos-guides/ide-tools-overview
- HUAWEI DevEco install — https://developer.huawei.com/consumer/en/doc/harmonyos-guides/ide-software-install
- DevEco macOS emulator (5.0.3 Beta2, Apple Silicon) — https://www.harmony-developers.com/p/global-developers-can-now-download
- HarmonyOS local-emulator FAQ — https://device.harmonyos.com/en/docs/apiref/doc-guides/faq-local-emulator-0000001116085454
- OpenHarmony build / CLI tools (Servo Book) — https://book.servo.org/building/openharmony.html
- DevEco Studio — https://en.wikipedia.org/wiki/DevEco_Studio
- Android emulator-container precedent — https://github.com/google/android-emulator-container-scripts
