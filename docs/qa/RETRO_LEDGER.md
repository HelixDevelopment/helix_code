# docs/qa Retro-Capture Ledger — §11.4.83 G7 gap close

**Created 2026-06-16 vs HEAD `9e6e0458` (`9e6e0458f93b7e50ee6d47d8324b34f02e77b31c`).**

This ledger closes the §11.4.83 `docs/qa/<run-id>/` evidence gap (G7) **honestly**,
by RE-CAPTURING FRESH real end-to-end evidence from the running current System on
2026-06-16. No historical transcripts were fabricated (§11.4.6 / §11.4.123). Every
retro-captured run is a CURRENT run, explicitly labelled as such in its run-id
README — never claimed as the original feature-shipping evidence.

Scope of this stream: `docs/qa/` additions ONLY. No source, no constitution
submodule, no `docs/Fixed.md` / `docs/Issues.md` / `docs/workable_items.db`, no
tests/integration files were touched. No push.

## A. Enumeration (in-scope vs deferred)

The existing `docs/qa/` tree is largely bug-fix-tracker-keyed (HXC-NNN) plus a
handful of feature-keyed run-ids (`g2-live-boot`, `web-llm-e2e`, `tui-ensemble-*`,
`all-providers-keys-integration`, `real-dial`, `W6B`, …). The significant shipped
user-visible features lacking a docs/qa entry, and their disposition this round:

### Retro-captured FRESH this round (runnable now, CLI path)

| Feature | run-id | Status |
|---|---|---|
| Real LLM generate + models-list (CONST-036) + health | `retro-cli-llm-generate-20260616` | ✅ captured, real DeepSeek output |
| `/specify` (real multi-agent SpecKit) + `/debate` | `retro-cli-specify-debate-20260616` | ✅ captured, real 3-round LLM debates |
| Streaming generate (`-stream`) | `retro-cli-stream-generate-20260616` | ✅ captured, real streamed output |
| Edit-safety `/diff` + `/checkpoint` + `/undo` | `retro-cli-edit-safety-20260616` | ✅ captured — **surfaced a real defect** (see §C) |

### Deferred — NOT reproducible headlessly now (honest, per §11.4.3 / §11.4.6)

| Feature | Original ship (commit) | Why not reproducible in this stream | Classification |
|---|---|---|---|
| Desktop Fyne GUI (TUI parity) | `47ee1ed7` | Needs a GUI window + macOS TCC screen-capture grant; not driveable from a headless background stream. | `NEEDS-GUI` / operator-attended |
| HelixCode logo + dark theme across 6 clients | `f9c6ab06` | Purely visual across GUI surfaces; needs window capture. | `NEEDS-GUI` |
| Web `/specify` UI (frontend) | `7a3fec7d` | Needs a browser/chromedp drive; pairs with existing `web-llm-e2e` harness but out of this stream's lightweight scope. | `NEEDS-GUI` |
| iOS / Android buildable apps (HXC-109) | `38caa48d`, `57b4dbcd` | Needs Xcode simulator / Android emulator hardware. | `NEEDS-HW` |
| Mobile gomobile `Generate()` binding | `28465071` | Needs gomobile bind + on-device call. | `NEEDS-HW` |
| harmony_os / aurora_os real chat | `38612f24`, `f8e14e29`, … | Platform-specific; nogui path partially CLI but OS targets need their runtime. | `NEEDS-HW` |
| Server `POST /api/v1/specify` (HXC-105) | `d7dffa14` | Runnable-now-API, but booting a server collides with concurrent streams (8111 helixagent already up; 8080 deliberately not booted to avoid cross-stream interference). Deferred to an operator/dedicated stream; the CLI `/specify` (same SpecKit engine) IS captured this round. | `RUNNABLE-API / deferred-to-avoid-collision` |
| Infra: full-test distributed to nezha (containers remote-compose) | `13a32796` | Needs remote creds + would restart shared nezha services another stream uses (forbidden by this stream's scope). | `NEEDS-CREDS` / shared-resource |

Deferred items are honest coverage gaps, not failures — recorded here per
§11.4.3 / §11.4.6 rather than faked.

## B. Provider used

DeepSeek (`HELIX_LLM_PROVIDER=deepseek`, keys from `~/api_keys.sh`). Real
provider responses with real token accounting captured throughout.

## C. Real defect surfaced by retro-run (anti-bluff finding)

`/checkpoint restore` prints an unconditional `✅ working tree restored to
checkpoint <id>` even when the git-backend checkpoint did NOT restore the
content — specifically for git-**untracked** working-tree files, which the
git-backend never snapshots. Tracked-file restore works correctly; untracked
files stay corrupted while the user sees a green "restored" message. Fully
characterized (tracked-vs-untracked controlled comparison + `git ls-tree`
proof) in `docs/qa/retro-cli-edit-safety-20260616/cli_checkpoint_restore_isolated.txt`.
Reported to the conductor for tracker filing; source fix is out of this
stream's scope.

## D. Quiescence (§11.4.84)

After all captures: HEAD unchanged at `9e6e0458`; the only working-tree changes
are this stream's `docs/qa/retro-*` additions + this ledger. The pre-existing
`m constitution` and another stream's `?? .../project_idor_realpg_test.go` are
NOT this stream's. Transient `.helix/` checkpoint working dir and the isolated
temp git repo created during capture were removed. Zero stray files left behind.
