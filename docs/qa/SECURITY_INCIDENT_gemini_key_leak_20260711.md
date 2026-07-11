# SECURITY INCIDENT — Committed Google (Gemini) API key leak

**Anchor:** CONST-042 / Article XII §12.1 (No-Secret-Leak) · §11.4.10 · §11.4.138 (operator/reviewer-escape ⇒ bluff-audit + permanent guard)
**Severity:** CRITICAL — release blocker until **rotated AND post-mortemed** (this document).
**Status:** Redacted at HEAD (this commit) · **ROTATION PENDING (operator action required)** · permanent guard tracked (follow-up).
**Discovered:** 2026-07-11 by the R41 delta-independent-review stream (§11.4.142), NOT by any pre-existing gate — a §11.4.138 escape (a green suite + release process that shipped a committed secret).

## What leaked
- A real Google-API-key-shaped secret (`GEMINI_API_KEY=AIza…`, value **redacted, not reproduced here** per §11.4.10) was committed in plaintext.
- **Location:** `docs/qa/phase1_providers_20260708T141500Z/live_probe.md` (line ~46).
- **Introduced by:** commit `f994c0c2`; present continuously through HEAD `536ac9c6`.
- **Exposure:** HEAD is pushed to the configured remotes (github `HelixDevelopment/Helix-CLI`, gitlab `helixdevelopment1/HelixCode`) — the value is **public in pushed history**.

## Impact
- Anyone with read access to either remote could read the key from history. Even if the key is/was invalid, CONST-042 treats the leak as a blocker regardless of claimed validity.
- Scope confirmed limited (§11.4.6, values never printed): `git grep` found the Google-key prefix in **exactly one** tracked file; broader tracked-file sweep for `sk-…`/`AKIA…`/`ghp_…`/private-keys returned **0**. (`xoxb-` matched 10 files — pending verification as pattern/doc mentions, not real tokens — in the follow-up guard task.)

## Remediation
1. **Redacted at HEAD (this commit):** the key value replaced with `<REDACTED-GEMINI-KEY-CONST-042-…>` in the tracked evidence file (and in the untracked review-report scratch file). Verified `git grep` → 0 remaining.
2. **ROTATION REQUIRED — OPERATOR ACTION (only they can):** revoke/rotate the leaked key in the Google Cloud console. **This is the primary remediation**: the value remains in pushed git *history* and CANNOT be purged, because history-rewrite / force-push is **absolutely forbidden** (§11.4.113 — no exception, even for secret purge). Rotation neutralizes the exposure; the dead value in history is then harmless.
3. **Post-mortem:** this document (satisfies the CONST-042 "post-mortemed" clause).

## Root cause (§11.4.102)
- `.gitignore` correctly excludes `.env`/`*.pem`/`*.key`, but **committed `docs/qa/*.md` evidence files (§11.4.83) are version-controlled by design** and were **not secret-scanned** before commit. A live-probe evidence capture inlined a real key into a committed markdown file, and no pre-commit / pre-push / release-gate secret scan existed to block it.

## Permanent fix (§11.4.135 / §11.4.138 — tracked follow-up, dispatched this session)
- Add a **secret-scan pre-commit/pre-push guard** (a §11.4.53-class gate) that blocks any commit whose staged content matches a key-shaped pattern (Google `AIza…`, `sk-…`, `AKIA…`, `ghp_…`, private-key headers, real `xoxb-…`), with an allowlist for redaction markers/examples. A paired §1.1 mutation must prove the guard is load-bearing (plant a fake key → guard FAILs; redact → PASSes).
- Register a permanent regression guard so this exact class cannot recur silently.

## Tracking
- Workable item to be recorded in the project tracker (§11.4.148): Type=Bug, Severity=Critical, Status=Reopened→(fix in progress), created_by=AI, assigned_to=Operator (rotation).
