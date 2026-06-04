// Package core previously held two no-assertion stub tests (TestSimple,
// TestAuthSimple) that PASSed while asserting nothing — a canonical
// §11.4 PASS-bluff (a green test that certifies no behavior). They were
// removed rather than "filled in" because the root meta-repo module
// (dev.helix.code) contains no end-user-facing application behavior to
// exercise at the e2e layer: the real application (auth, LLM, CLI,
// server) lives in the separate inner module helix_code/ (module
// dev.helix.code, go 1.26), and any in-process assertion against the
// root module's helpers (internal/theme constants, internal/security
// scanners) would be a unit test mislabeled as e2e — itself a category
// bluff, not honest end-to-end coverage.
//
// Real, runtime-evidenced e2e coverage for HelixCode lives in shell
// challenges under tests/e2e/challenges/ (driven by
// tests/e2e/challenges/run_all_challenges.sh):
//
//   - phase1_llm_challenge.sh            real LLM generation
//   - phase2_tools_editor_challenge.sh   real tool/editor flow
//   - phase3_worker_challenge.sh         real worker pool
//   - phase4_workflow_challenge.sh       real workflow engine
//   - phase5_mcp_memory_challenge.sh     real MCP + memory
//   - phase6_testing_challenge.sh        real testing integration
//   - phase7_documentation_challenge.sh  docs verification
//   - phase8_helixqa_challenge.sh        HelixQA integration
//   - at_mention.sh / conversational_repl.sh / ui_terminal_interaction.sh
//   - ux_end_to_end_flow.sh
//   - ddos_health_flood.sh / stress_sustained_load.sh
//   - chaos_failure_injection.sh / scaling_horizontal.sh
//   - helix_qa_live_anti_bluff.sh / gptme_*.sh
//
// Auth-specific e2e (the intent the removed TestAuthSimple stub gestured
// at) is exercised against the real auth package in the inner module:
// helix_code/internal/auth (unit) and the integration suite at
// helix_code/tests/integration/ (run with -tags=integration against real
// PostgreSQL/Redis), plus the live HTTP flow in the phase challenges
// above.
//
// This file exists only to keep the package compiling so
// `go test ./tests/e2e/core/` resolves cleanly. Do NOT re-introduce an
// empty PASS test here; add a real challenge under tests/e2e/challenges/
// (with captured runtime evidence per §11.4.5/§11.4.69) instead.
package core
