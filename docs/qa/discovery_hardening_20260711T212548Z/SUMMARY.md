# Discovery / Hardening Sweep — Wave 1 Evidence

Autonomous discovery sweep (§11.4.118 + §11.4.169) of the "complete" (203-distinct-item,
all-terminal) HelixCode tree, followed by wave-1 hardening fixes. All fixes reproduce-first
(§11.4.115/§11.4.146), independently reviewed (§11.4.142), with captured RED→GREEN / coverage /
§1.1 mutation evidence. No bluff.

## Fixes closed this wave
- HXC-115 (Bug) CONST-046 gate portability — RED 19098-NEW/exit1 → GREEN 0-NEW/exit0; repo-relative baseline (§11.4.177); 2 new §11.4.135 guards; §1.1 enforcement preserved. Evidence: S3_const046_gate_evidence.md
- HXC-116 (Task) MULTITRACK_WORKTREE_RUNBOOK.md authored from real script reads. Evidence: S4_docs_evidence.md
- HXC-120 (Bug) CORS stale test reconciled to secure allowlist (§11.4.120), no wildcard reintroduced. Evidence: S1_cors_evidence.md
- HXC-121 (Task) provider tests huggingface 92.6% + together 96.4% (§1.1 mutation-proven). Evidence: S2_provider_tests_evidence.md
- HXC-123 (Task) cmd/security_scan tests 18 PASS (§1.1 proven). Evidence: S5_security_scan_evidence.md
- HXC-130 (Task) GUI_BUILD_ENV.md documents X11/GL dep + §11.4.77 regen-mechanism. Evidence: S4_docs_evidence.md

## Discovery reports (domain audits)
D1_test_execution.md · D3_wiring.md · D4_closed_audit.md · D5_challenges_qa.md

## Remaining backlog (Queued, wave-2+)
HXC-117 verifier caps · HXC-118 RAG integration · HXC-119 ACP · HXC-122 memory/automation runs ·
HXC-124 HelixQA JWT gap · HXC-125 integration-tag visibility · HXC-126 tracker move-drift ·
HXC-127 obsolete_details · HXC-128 short descriptions · HXC-129 severity backfill · (Phase F submodules pending D2)
