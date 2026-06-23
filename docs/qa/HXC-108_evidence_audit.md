# HXC-108 — Recordable-Client Evidence Audit (§11.4.118 discovery / bluff-hunt)

| | |
|---|---|
| **Run-id** | HXC-108_evidence_audit |
| **Date (UTC)** | 2026-06-23 |
| **Auditor** | subagent (respawn per §11.4.147), read-only |
| **Mandate** | Apply the HXC-107 skeptical lens (which caught rotated-away ledger recordings) to the 7 HXC-108 per-client evidence docs — confirm the recordable-client §11.4.158 coverage is genuinely backed by extant artifacts, not bluffed. |
| **Method** | Per doc: (1) extract every cited artifact path; (2) check existence in the committed `docs/qa/` tree AND the raw `/Volumes/T7/Downloads/Recordings/` corpus; (3) where a cited filename is absent, md5-compare to detect content-present-under-different-name vs genuinely-missing; (4) confirm the self-validated OCR analyzer script + harness scripts exist. |

## Overall verdict

**Recordable-client coverage is GENUINELY BACKED by real artifacts — NO content-loss bluff of the HXC-107 class was found.** Every load-bearing md5 cited across CLI / desktop GUI / web / iOS / android matched a file on disk byte-for-byte. The self-validated analyzer (`scripts/video_qa/record_feature.sh`, 26 KB, golden-good/golden-bad wired) and all three cited harness scripts exist.

**BUT there is a real, lower-severity ROTATION RISK and one filename-citation-drift finding:** 6 of 7 docs cite the **rotatable git-ignored raw corpus** (`/Volumes/T7/Downloads/Recordings/`) as their sole artifact home — exactly the §11.4.154-rotation surface the HXC-107 audit flagged. Only the **android** doc moved its evidence to the durable committed `docs/qa/HXC-108_android/` path. The raw corpus is one `record_*.sh` rerun (or §11.4.154 rotation) away from invalidating the other 6 docs' citations. The desktop-GUI doc additionally cites **stale filenames** (`T110359Z`) that no longer exist — the byte-identical content survives under newer timestamps (`T155000Z`), so it is drift, not loss, but the doc as written points at absent files.

## §11.4.118 enumerated coverage — the 7 docs checked + each outcome

| # | Doc | Verdict | Cited-artifact existence |
|---|-----|---------|--------------------------|
| 1 | `HXC-108_cli_recordings_evidence.md` | **SOUND** (rotation-risk) | 6/6 MP4s present; version md5 `6e2317…` matches exactly |
| 2 | `HXC-108_tui_server_recordings_evidence.md` | **SOUND** (rotation-risk) | dashboard + server-api MP4s present; both timestamps match |
| 3 | `HXC-108_desktopgui_features_evidence.md` | **OVER-CLAIM (filename drift)** | cited `*-T110359Z.mp4` MISSING; byte-identical content present as `*-T155000Z.mp4` (dashboard `c302e2b8…`, tasks `327319062…` md5-confirmed) |
| 4 | `HXC-108_ios_evidence.md` | **SOUND** (rotation-risk) | MP4 md5 `e565c6bd…` matches; 144 frames ffprobe-confirmed; PNG present |
| 5 | `HXC-108_web_evidence.md` | **SOUND** (rotation-risk) | MP4 md5 `ddf3eaf6…` + evidence_frame PNG both match exactly |
| 6 | `HXC-108_tui_views_evidence.md` | **SOUND** (rotation-risk) | all 6 view MP4s + evidence_frame + pane.txt present (tasks/workers/sessions/projects/qa timestamps all match) |
| 7 | `HXC-108_android_evidence.md` | **SOUND (best practice)** | durable committed mp4+png present; committed mp4 byte-identical to raw (`b120f822…`); 1080×1920/33f + 1080×1920 PNG ffprobe-confirmed |

## Findings (path evidence per finding; FACT unless marked)

### F1 — DESKTOP-GUI doc cites stale filenames that no longer exist (OVER-CLAIM / filename drift) — FACT
`docs/qa/HXC-108_desktopgui_features_evidence.md` cites all six tab MP4s as `helixcode-desktopgui-<tab>-20260623T110359Z.mp4`. **No `T110359Z` file exists** in `/Volumes/T7/Downloads/Recordings/`. The corpus instead holds `T155000Z` + `T155035Z` variants (a later regeneration). Severity-reducing FACT: the cited **md5s match the `T155000Z` files byte-for-byte** — dashboard cited `c302e2b8a032f62384a468d1c54e0c95` = `helixcode-desktopgui-dashboard-20260623T155000Z.mp4`; tasks cited `327319062af6a51de55f15d7ec16fca3` = BOTH `T155000Z` and `T155035Z`. So the **content is present and provably the same**; only the doc's filename pointers are stale. This is filename-citation drift (the doc points at absent files), NOT the HXC-107 content-loss bluff. Fix: update the six filenames to `T155000Z` (or re-pin to whatever the regeneration left), or move the MP4s to a durable `docs/qa/HXC-108_desktopgui/` path.

### F2 — 6 of 7 docs anchor evidence ONLY in the rotatable raw corpus (ROTATION RISK) — FACT
CLI, tui_server, desktopgui, ios, web, tui_views all cite `/Volumes/T7/Downloads/Recordings/` (git-ignored §11.4.128, §11.4.154-rotatable) as the sole artifact location. None of their MP4s/PNGs are committed under `docs/qa/`. The android doc explicitly contrasts this: *"Durable evidence committed under `docs/qa/HXC-108_android/` … NOT the rotatable git-ignored raw corpus — the exact §11.4.154-rotation bluff the HXC-107 audit caught."* The other 6 docs did not adopt that fix. **They are SOUND today** (every artifact verified present), but a single harness rerun or rotation invalidates every citation — the same fragility HXC-107 flagged. Recommended: copy the load-bearing MP4 + evidence_frame for each of the 6 into a committed `docs/qa/HXC-108_<surface>/` per the android pattern.

### F3 — analyzer + harness scripts genuinely exist (NOT a bluff) — FACT (positive)
`scripts/video_qa/record_feature.sh` exists (26202 bytes, 26 hits for `selftest`/`golden-good`/`golden-bad`/`ocr-analyze`), so the §11.4.107(10) self-validation claimed across every doc is backed by a real script. `record_tui_views.sh` + `record_gui_features_inprocess.sh` (cited by docs 7 and 3) both exist. No analyzer-self-validation over-claim found.

### F4 — claimed "real output" results are concrete + falsifiable (NOT vague) — FACT (positive)
Spot-checked the load-bearing assertions; all are specific and unfalsifiable-if-fake: CLI generate `tokens: in=17 out=18 … finish: stop` + answer `4`; CLI command-exec `expr 6 * 7 = 42` (real `os/exec`, cannot be print-and-sleep); iOS `Go core OK - themes: 3, tasks: 2`; web `HELIXCODE_WEB_OK4` + `tokens=146 finish=stop`; android `ResumedActivity: dev.helix.code/.MainActivity` + APK md5 match. These are concrete enough that a faked PASS would be detectable — no vague "renders correctly"-class claims.

### F5 — docs are HONEST about their gaps (NOT over-claiming completeness) — FACT (positive)
Each doc carries explicit §11.4.3 SKIP/limit sections: CLI SKIPs streaming + `-qa-run` (server-gated); tui_server documents the TUI interaction limit + the asciinema TCC env-gap; desktopgui notes dialogs-not-driven; web surfaces a REAL upstream DeepSeek-502 finding rather than hiding it; tui_views honestly states a transient throttle killed the agent before the analyzer ran and the conductor finished validation directly; ios documents the CoreSimulator noowners blocker. No doc claims more than it shows.

### F6 — md5/frame integrity of the durable android artifacts (positive control) — FACT
The one doc that did it right verifies cleanly: committed `docs/qa/HXC-108_android/helixcode-android-client.mp4` is byte-identical (`b120f8222a286b3534cde4a15e469a86`) to the raw-corpus original, ffprobe confirms H.264 1080×1920 / 33 frames, and the key PNG is 1080×1920. The android doc's claims are fully backed by committed, non-rotatable evidence.

## Durable-vs-rotatable summary (the §11.4.154 axis)

- **Durable (committed, rotation-proof):** `HXC-108_android_evidence.md` only.
- **Rotation-risk (raw corpus only):** the other 6 — cli, tui_server, desktopgui, ios, web, tui_views.

## Conclusion

The HXC-108 recordable-client evidence is **real and presently backed** — no HXC-107-class content-loss bluff. The bluff-hunt surfaced (a) one filename-drift over-claim (desktop GUI cites absent `T110359Z` files; byte-identical content lives under `T155000Z`), and (b) a systemic rotation RISK: 6 of 7 docs rely on the git-ignored raw corpus the android doc deliberately avoided. Both are honest-state findings to remediate before relying on these docs at a release gate, not evidence of fabrication.
