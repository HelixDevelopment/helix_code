# HelixQA Chaos Bank: HelixLLM Coder Chaos Resilience

**Run-ID:** `helixllm_coder_chaos_20260708T154503Z`
**Date:** 2026-07-08
**Bank:** `helix_qa/banks/helixllm_coder_chaos.yaml`
**Analyzer:** `helix_qa/cmd/helixqa-verify-coder-chaos/` (Go binary at `bin/helixqa-verify-coder-chaos`)
**Author:** vasic-digital (Milos Vasic)
**Covenant:** §11.4.85 (stress + chaos mandate) + §11.4.169 (comprehensive test-type coverage)

---

## Summary

This bank closes the **chaos-resilience** coverage gap for the HelixLLM coder (llama.cpp OpenAI-compatible sidecar at `:18434`). Three adversarial modes exercise distinct resilience axes without any destructive coder mutation (coder is read-only exercised per §11.4.122):

| Case | Mode | Assertion | Domain |
|------|------|-----------|--------|
| CODER-CHAOS-001 | Port-flood (50 TCP burst-10) | Coder recovers after connection storm | network-resilience |
| CODER-CHAOS-002 | Oversized-prompt (200 KiB) | Graceful degrade + post-oversized health | input-validation |
| CODER-CHAOS-003 | Concurrent-health (5+3 probes) | All POSTs + health OK under load | endpoint-isolation |
| SELF-VALIDATE-001-GOOD | Port-flood (10 burst-5) healthy | Analyzer PASS on healthy coder | anti-bluff |
| SELF-VALIDATE-001-BAD | Port-flood unreachable port | Analyzer FAIL inverted to PASS via --expect-fail | anti-bluff |

---

## Structure

### Bank file (`helix_qa/banks/helixllm_coder_chaos.yaml`)

- Follows the same `dispatches_to`/`required_evidence` convention as `helixllm_coder.yaml`, `helixllm_coder_concurrency.yaml`, and `helixllm_coder_memory.yaml`.
- Each case dispatches to `bin/helixqa-verify-coder-chaos` with a mode flag and mode-specific parameters.
- `required_evidence` asserts JSON verdict fields via the `| json:key==value |` pipe syntax.

### Analyzer (`helix_qa/cmd/helixqa-verify-coder-chaos/`)

- Thin, project-agnostic Go binary (CONST-051(B)/§11.4.28 decoupling).
- Three mode runners: `runPortFlood`, `runOversizedPrompt`, `runConcurrentHealth`.
- Each mode runner returns a structured `verdict` struct serialised as JSON to `--out`.
- Exit 0 = case PASS, exit 1 = case FAIL, exit 2 = infra error (mirrors sibling analyzers).
- `--expect-fail` inverts the case-level result for self-validation (no change to the raw assertion logic — only the case-level interpretation flips).

### Evidence path

- Raw verdict JSONs land in `helix_qa/qa-results/helixllm_coder_chaos/` (git-ignored, §11.4.128).
- Per-run curated evidence committed at release-prep under `docs/qa/<run-id>/`.
- Per-§11.4.116: the analyzer writes conduit events to `--conduit-dir` for real-time conductor integration.

---

## Self-validation (§11.4.107(10))

The SAME analyzer binary (`helixqa-verify-coder-chaos`) drives BOTH self-validation fixtures:

**GOOD:** `--mode port-flood --n 10 --burst-size 5 --endpoint http://localhost:18434`
- Healthy coder at default port. All TCP connects land on a real listener; post-flood recovery check gets HTTP 200.
- Expected: raw `pass==true`, `case_result==true`.

**BAD:** `--mode port-flood --n 10 --burst-size 5 --port 19999 --endpoint http://localhost:19999 --expect-fail`
- Unreachable port 19999 — every TCP connect fails. The analyzer's recovery logic correctly reports `recovered==false`, `pass==false`.
- `--expect-fail` inverts the case-level result: `case_result==true` (exit 0).
- A paired §1.1 mutation (forcing `v.Pass = true` unconditionally) would flip `case_result` to false (exit 1) — proving the recovery assertion is live and load-bearing.

---

## Anti-bluff design (§11.4.6/§11.4.69/§11.4.107)

Each mode carries a multi-dimensional runtime signature:

| Mode | Dimension 1 | Dimension 2 | Dimension 3 |
|------|-------------|-------------|-------------|
| Port-flood | `flood_sent` == N (actual TCP dials) | `recovered` == true (post-flood 200) | `post_flood_ok` == true (non-empty body) |
| Oversized | `oversized_handled` == true (non-zero status) | `oversized_status` recorded | `post_oversized_ok` == true (normal follow-up succeeds) |
| Concurrent-health | `all_ok` == true (N POSTs all 200) | `models_ok` == true (N health probes all 200) | `n_models_ok` == N (every probe accounted) |

All assertions require real HTTP round-trips or real TCP socket operations against the live coder — never metadata-only or probe-not-reachable.

---

## §11.4.185 — Manual QA Confirmation Pending

This bank is committed with the analyzer binary built and self-validated. Automated execution confirms the bank structure, analysis logic, and anti-bluff design are correct. **Final confirmation by the QA team** is the required terminal gate before the cases are marked "fully completed" per §11.4.185.

---

## Files modified/created

- `helix_qa/banks/helixllm_coder_chaos.yaml` — new bank (5 cases: 3 chaos + 2 self-validation)
- `helix_qa/cmd/helixqa-verify-coder-chaos/main.go` — new analyzer (~460 LoC)
- `helix_qa/bin/helixqa-verify-coder-chaos` — built binary (verified compile-OK)
- `docs/qa/helixllm_coder_chaos_20260708T154503Z/RESULTS.md` — this evidence document
