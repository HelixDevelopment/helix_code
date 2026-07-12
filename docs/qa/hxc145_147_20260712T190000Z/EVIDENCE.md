# HXC-145 + HXC-147 — QA evidence (§11.4.83)

**Items:** HXC-145 (Bug/Low) + HXC-147 (Bug/Med)
**Fix commit:** helix_code `0e3bb747` (6 files; pushed github+gitlab)
**Date (UTC):** 2026-07-12T19:00:00Z
**Closure vocab:** Fixed (§11.4.33, Bug)

## HXC-145 — Xiaomi mimo-v2-flash deprecated
xiaomi_provider.go seed list: mimo-v2-flash → mimo-v2.5-pro. XIAOMI_PROVIDER.md doc updated.
Test fixtures already cleaned by HXC-142/143 compile fix.

## HXC-147 — OpenRouter nil-ptr panic
README.md + USER_GUIDE.md: deepseek-r1-free → openai/gpt-oss-20b:free (production already
fixed, docs were stale). assert.NoError/NotNil → require.NoError/NotNil in
free_providers_automation_test.go + openrouter_automation_test.go (nil-deref crash hardening).

## Verification
go build -tags=nogui exit 0. Anti-bluff smoke clean. require import confirmed in both files.
