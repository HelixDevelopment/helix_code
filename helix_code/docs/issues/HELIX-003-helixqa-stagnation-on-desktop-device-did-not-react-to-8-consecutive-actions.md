---
id: HELIX-003
severity: critical
category: functional
platform: desktop
screen: 
status: open
found_date: 2026-06-03
---

# HelixQA stagnation on desktop — device did not react to 8 consecutive actions

Screen hash was identical across maxConsecutiveUnchanged curiosity steps. Either ADB commands are silently failing (e.g., `cmd input` returning 'No shell command implementation.' on Android 9) or the app is frozen. See FIX-QA-2026-04-21-017/018.

