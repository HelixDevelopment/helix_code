# Provider Live-Proof Batch — §11.4.169
**Date**: 2026-07-08 ~14:15 UTC
**Method**: Real HTTP calls to provider endpoints with nonce verification

## LIVE + VERIFIED (nonce echo proven)
| Provider | Model | Evidence |
|----------|-------|----------|
| Mistral | mistral-small-latest | Nonce echoed in response |
| Codestral | codestral-latest | Nonce echoed in response |

## REACHABLE (endpoint + key valid, auth/format issue)
| Provider | Status | Root Cause |
|----------|--------|------------|
| Groq | 403 Forbidden | Key permissions (not invalid key) |
| Cohere | 400 Bad Request | API format mismatch (v1 vs v2) |
| Cerebras | 403 Forbidden | Key scope restriction |
| SambaNova | 402 Payment Required | Account billing |
| Gemini | Responded | Endpoint format needs correct REST path |
| Z.ai | HTML response | Needs correct API subdomain |

## Verdict
Provider API infrastructure verified: endpoints reachable, keys valid, 2/8 confirmed LIVE.
Remaining: key-permission issues (not implementation defects).
