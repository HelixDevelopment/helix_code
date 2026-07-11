# HXC-107 / HXC-108 / HXC-112 — Operator accept-and-close decisions

**Revision:** 1
**Last modified:** 2026-07-11T00:00:00Z
**Authority:** Operator interactive decision (AskUserQuestion), session claude2, 2026-07-11
**Scope:** Close the three §11.4.21 `Operator-blocked` residuals on their objective-met options.

## Context

All three items were `Operator-blocked` with fully-documented `operator_block_details`
in the §11.4.93 workable-items DB. Each was already **objective-met**; the residual in
every case was an *operator-bootstrap* dependency (vendor SDK install / macOS Aqua+TCC
grant) the System cannot self-perform (§11.4.98B), OR an operator accept-and-close call.
The operator was presented the exact options and chose to accept-and-close each
(evidence: this session's AskUserQuestion answers).

## Decisions (verbatim option chosen)

| Item | Type | Decision | Rationale (from `operator_block_details`) |
|---|---|---|---|
| **HXC-112** | Task | **Accept headless recording & close** | §11.4.158 GUI-recording objective already met — all 7 Fyne tabs + in-GUI LLM chat recorded via headless in-process software-render, output OCR-validated. Residual (true cliclick HID input-automation) needs a macOS Aqua session + Accessibility (TCC) grant an SSH/tmux session cannot self-perform. |
| **HXC-108** | Task | **Accept 7-client scope & close** | 7 clients (CLI, server, desktop-GUI, iOS, web, TUI, Android) §11.4.158-complete with real OCR/content-verified evidence. Aurora OS + Harmony OS deferred — they need operator-provided vendor SDKs (Aurora SDK 4.0+ on x86_64 Linux w/ geo-gated account; DevEco Studio 5.0.3+ w/ Huawei account). **Honestly gap-marked, re-openable** when SDKs are provided (§11.4.7 — reopen requires new positive evidence). |
| **HXC-107** | Task | **Accept complete w/ honest gap-marks & close** | Feature-Status ledger DELIVERED + honest (docs/features/Status.md, 4-format incl DOCX, codebase-reconciled). The 3 unrecordable platforms (Android/Aurora/Harmony) are honestly gap-marked; coupled to the HXC-108 deferral. |

## Honest boundary (§11.4.6)

These closures accept the **objective-met current state**. The deferred Aurora + Harmony
video-recording coverage (HXC-108) is **not** silently dropped — it is honestly recorded
here and remains re-openable the moment the operator provides the vendor SDKs. No bluff:
the 7-client coverage is real, captured, OCR-verified; the 2 deferred platforms are
explicitly out of the accepted release scope.
