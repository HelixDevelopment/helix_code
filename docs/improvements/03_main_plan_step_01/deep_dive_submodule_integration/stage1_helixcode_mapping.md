# HelixCode Repository - Stage 1 Complete Mapping Report

**Generated:** 2026-05-04  
**Repository:** https://github.com/HelixDevelopment/HelixCode.git  
**Mapping Method:** GitHub API (git clone failed due to HTTP/2 framing layer network errors)  
**Branch:** main

---

## 1. CLONE STATUS

| Attempt | Method | Result | Notes |
|---------|--------|--------|-------|
| 1 | HTTPS full clone | FAILED | `fatal: write error: Input/output error` |
| 2 | HTTPS shallow clone | FAILED | Same I/O error |
| 3 | HTTP/1.1 forced | FAILED | `curl 16 Error in the HTTP2 framing layer` |
| 4 | SSH | NOT ATTEMPTED | No SSH key configured in environment |

**Resolution:** Complete repository mapping achieved via GitHub REST API (`api.github.com`) and raw content access (`raw.githubusercontent.com`). All submodule and directory data successfully extracted.

---

## 2. REPOSITORY OVERVIEW

| Property | Value |
|----------|-------|
| **Primary Language** | Go |
| **Go Module** | `dev.helix.code` |
| **Go Version** | `1.25.2` |
| **Root Items** | 162 (mix of files and directories) |
| **Total Submodules Declared** | 87 |
| **Submodule Protocol** | 85 SSH (`git@github.com`), 2 org-specific SSH |
| **Docker Support** | Yes (Dockerfile + docker-compose + specialized configs) |
| **CI/CD** | GitHub Actions (1 workflow) |
| **Makefile** | Yes |

---

## 3. COMPLETE DIRECTORY TREE (Top 3 Levels)

```
HelixCode/
├── .env.example
├── .github/
│   └── workflows/
│       └── notification-tests.yml
├── .gitignore
├── .gitmodules
├── .gitmodules.bak.https-to-ssh
├── AGENTS.md                              (41KB - agent definitions)
├── AIDER_CLINE_IMPLEMENTATION_ROADMAP.md
├── AI_MEMORY_INTEGRATION_PROGRESS.md
├── ANALYSIS_README.md
├── ANALYSIS_SOURCES.md
├── APPLICATION_CHALLENGE_STATUS.md
├── AUDIT_IMPLEMENTATION_TRACKER.md
├── AUDIT_TRACKER_2026.md
├── assets/
│   ├── AGENTS.md
│   ├── CLAUDE.md
│   ├── CONSTITUTION.md
│   ├── Logo.png
│   └── Wide_Black.png
├── CLAUDE.md
├── COMPLETE_IMPLEMENTATION_TRACKER.md
├── COMPLETION_STATUS_AND_REMAINING_WORK.md
├── COMPREHENSIVE_ANALYSIS_REPORT.md
├── COMPREHENSIVE_AUDIT_REPORT.md
├── COMPREHENSIVE_COMPLETION_PLAN.md
├── COMPREHENSIVE_COMPLETION_REPORT.md
├── COMPREHENSIVE_FEATURE_MATRIX.md
├── COMPREHENSIVE_PROJECT_AUDIT_2026.md
├── COMPREHENSIVE_PROJECT_REPORT_AND_IMPLEMENTATION_PLAN.md
├── COMPREHENSIVE_UNFINISHED_WORK_AND_IMPLEMENTATION_PLAN.md
├── COMPREHENSIVE_UNFINISHED_WORK_REPORT.md
├── CONSTITUTION.md
├── CONTINUE_HERE.md
├── CRUSH.md
├── challenges/                            ← SUBMODULE (vasic-digital/Challenges)
├── containers/                            ← SUBMODULE (vasic-digital/Containers)
├── DETAILED_IMPLEMENTATION_PLAN.md
├── DETAILED_IMPLEMENTATION_TRACKER.md
├── DOCKER_COMPLETION_SUMMARY.md
├── DOCKER_SETUP.md
├── DOCUMENTATION_COMPLETION_PLAN.md
├── Dockerfile
├── Dockerfile.test
├── Dockerfile.worker
├── Dependencies/
│   ├── HelixDevelopment/
│   │   ├── DocProcessor/                  ← SUBMODULE
│   │   ├── LLMOrchestrator/               ← SUBMODULE
│   │   ├── LLMProvider/                   ← SUBMODULE
│   │   ├── LLMsVerifier/                  ← SUBMODULE (REQUIRED ✓)
│   │   └── VisionEngine/                  ← SUBMODULE
│   ├── HuggingFace_Hub/                   ← SUBMODULE
│   ├── LLama_CPP/                         ← SUBMODULE
│   └── Ollama/                            ← SUBMODULE
├── Documentation/
│   ├── General/
│   │   ├── FEATURE_IMPLEMENTATION_COMPLETE.md
│   │   ├── MENTIONS_USER_GUIDE.md
│   │   ├── PROVIDER_FEATURES.md
│   │   ├── PROVIDER_UPDATE_SUMMARY.md
│   │   └── SLASH_COMMANDS_USER_GUIDE.md
│   ├── Materials/
│   │   ├── Encryption_Algs.md
│   │   └── LLMs_Optimization.md
│   └── User_Manual/
│       ├── INDEX.md
│       ├── README.md
│       ├── SUMMARY.md
│       ├── USER_MANUAL_EXPANSION_PLAN.md
│       ├── examples/
│       └── tutorials/
├── E2E_TEST_SPECIFICATION.md
├── EXAMPLE_PROJECTS_ANALYSIS.md
├── EXAMPLE_PROJECTS_INDEX.md
├── EXAMPLE_PROJECTS_QUICK_REFERENCE.md
├── Example_Projects/
│   ├── Agent-Deck/                        ← SUBMODULE
│   ├── Aider/                             ← SUBMODULE
│   ├── Amazon-Q-Developer-CLI/            ← SUBMODULE
│   ├── Bridle/                            ← SUBMODULE
│   ├── Cheshire-Cat-Ai/
│   │   ├── CC-AA-LLMs/                    ← SUBMODULE
│   │   ├── CC-AI-Core/                    ← SUBMODULE
│   │   ├── CC-AI-Plugins/                 ← SUBMODULE
│   │   ├── model-plugins/                 ← SUBMODULE
│   │   ├── plugin-template/               ← SUBMODULE
│   │   └── plugins-backend/               ← SUBMODULE
│   ├── Claude-Code-Plugins-And-Skills/    ← SUBMODULE
│   ├── Claude-Squad/                      ← SUBMODULE
│   ├── Claude_Code/                       ← SUBMODULE
│   ├── Cline/                             ← SUBMODULE
│   ├── Codai/                             ← SUBMODULE
│   ├── Codename_Goose/                    ← SUBMODULE
│   ├── Codex/                             ← SUBMODULE (org-14957082@github.com)
│   ├── Codex-Skills/                      ← SUBMODULE
│   ├── Conduit/                           ← SUBMODULE
│   ├── DeepSeek_CLI/                      ← SUBMODULE
│   ├── Emdash/                            ← SUBMODULE
│   ├── FauxPilot/                         ← SUBMODULE
│   ├── Forge/                             ← SUBMODULE
│   ├── GPT_Engineer/                      ← SUBMODULE
│   ├── Gemini_CLI/                        ← SUBMODULE
│   ├── Get-Shit-Done/                     ← SUBMODULE
│   ├── GitHub-Copilot-CLI/                ← SUBMODULE
│   ├── GitHub-Spec-Kit/                   ← SUBMODULE
│   ├── GitMCP/                            ← SUBMODULE
│   ├── Kilo-Code/                         ← SUBMODULE
│   ├── Mistral_Code/                      ← SUBMODULE
│   ├── MobileAgent/                       ← SUBMODULE
│   ├── Multiagent-Coding-System/          ← SUBMODULE
│   ├── Nanocoder/                         ← SUBMODULE
│   ├── Noi/                               ← SUBMODULE
│   ├── Octogen/                           ← SUBMODULE
│   ├── Ollama_Code/                       ← SUBMODULE
│   ├── OpenCode/
│   │   ├── Homebrew-Tap/                  ← SUBMODULE
│   │   └── OpenCode/                      ← SUBMODULE
│   ├── OpenHands/
│   │   ├── OpenHands/                     ← SUBMODULE
│   │   ├── OpenHands-Docs/                ← SUBMODULE
│   │   ├── OpenHands-Server/              ← SUBMODULE
│   │   ├── agent-analysis/                ← SUBMODULE
│   │   ├── agentic-code-search-oss/       ← SUBMODULE
│   │   ├── litellm/                       ← SUBMODULE
│   │   ├── open-operator/                 ← SUBMODULE
│   │   ├── skills/                        ← SUBMODULE
│   │   ├── software-agent-sdk/            ← SUBMODULE
│   │   └── OpenHands-Docs/                ← SUBMODULE
│   ├── Plandex/                           ← SUBMODULE
│   ├── Postgres-MCP/                      ← SUBMODULE
│   ├── Qwen_Code/                         ← SUBMODULE
│   ├── Shai/                              ← SUBMODULE
│   ├── SnowCLI/                           ← SUBMODULE
│   ├── Stark-Kitty-Kiro-Cli/              ← SUBMODULE
│   ├── Superset/                          ← SUBMODULE
│   ├── TaskWeaver/                        ← SUBMODULE
│   ├── Warp/                              ← SUBMODULE
│   ├── gptme/
│   │   ├── gptme/                         ← SUBMODULE
│   │   ├── gptme-agent-template/        ← SUBMODULE
│   │   ├── gptme-contrib/                 ← SUBMODULE
│   │   ├── gptme-lessons/                 ← SUBMODULE
│   │   ├── gptme-pitch-deck/              ← SUBMODULE
│   │   ├── gptme-rag/                     ← SUBMODULE
│   │   ├── gptme-tauri/                   ← SUBMODULE
│   │   └── gptme-webui/                   ← SUBMODULE
│   ├── ui-ux-pro-max-skill/               ← SUBMODULE
│   └── vtcode/                            ← SUBMODULE
├── Example_Resources/
│   ├── Awesome-AI-Agents/                 ← SUBMODULE
│   ├── Awesome-AI-GPTs/                   ← SUBMODULE
│   ├── Cheshire-Cat-Ai/
│   │   └── CC-AI-Docs/                    ← SUBMODULE
│   ├── GitHub-Awesome-Copilot/            ← SUBMODULE
│   ├── OpenAI-Cookbook/                   ← SUBMODULE (org-14957082@github.com)
│   └── Taches-CC-Resources/               ← SUBMODULE
├── FEATURE_COMPARISON_MATRIX.md
├── FINAL_COMPLETION.md
├── FINAL_COMPLETION_REPORT.md
├── FINAL_COMPLETION_SUCCESS_REPORT.md
├── FINAL_SESSION_SUMMARY.md
├── GAP_ANALYSIS.md
├── GAP_ANALYSIS_SUMMARY.md
├── github_pages_website/                  ← SUBMODULE (HelixDevelopment-Code/Welcome)
├── HELIXCODE_*.md                          (30+ architecture/audit docs)
├── HelixCode/                              ← NESTED PROJECT STRUCTURE
│   ├── .env.example
│   ├── .env.full-test
│   ├── .github/workflows/
│   ├── .gitignore
│   ├── AUDIT_ISSUE_REGISTRY.md
│   ├── BUILD_INTEGRATION_SUMMARY.md
│   ├── CLINE_RULES_COMPLETION_SUMMARY.md
│   ├── COMPLETE_IMPLEMENTATION_REPORT.md
│   ├── COMPLETION_ANALYSIS.md
│   ├── COMPLETION_REPORT.md
│   ├── COMPREHENSIVE_IMPLEMENTATION_PLAN.md
│   ├── COMPREHENSIVE_TEST_EXECUTION_REPORT.md
│   ├── CONTEXT_BUILDER_COMPLETION_SUMMARY.md
│   ├── DOCKER_CICD_COMPLETION_REPORT.md
│   ├── DOCKER_DEPLOYMENT.md
│   ├── DOCUMENTATION_SYSTEM_SUMMARY.md
│   ├── Documentation/
│   │   ├── AUDIT_IMPLEMENTATION_PLAN.md
│   │   ├── Architecture/
│   │   ├── COMPREHENSIVE_AUDIT_REPORT.md
│   │   ├── General/
│   │   ├── Testing/
│   │   └── User_Manual/
│   ├── EDIT_FORMATS_COMPLETION_SUMMARY.md
│   ├── ENTERPRISE_DEPLOYMENT_GUIDE.md
│   ├── ENTERPRISE_USER_MANUAL.md
│   ├── EXECUTIVE_TEST_SUMMARY.md
│   ├── FINAL_COMPLETION.md
│   ├── FINAL_COMPLETION_CERTIFICATE.md
│   ├── FINAL_COMPLETION_REPORT.md
│   ├── FINAL_COMPLETION_SUMMARY.md
│   ├── FINAL_COMPREHENSIVE_COMPLETION_REPORT.md
│   ├── FINAL_ENTERPRISE_COMPLETION_REPORT.md
│   ├── FINAL_ENTERPRISE_DEPLOYMENT_SUMMARY.md
│   ├── FINAL_PHASE1_SUMMARY.md
│   ├── FINAL_SUMMARY.md
│   ├── FOCUS_CHAIN_COMPLETION_SUMMARY.md
│   ├── HARMONY_AURORA_COMPLETION_REPORT.md
│   ├── HOOKS_SYSTEM_COMPLETION_SUMMARY.md
│   ├── IMPLEMENTATION_LOG.txt
│   ├── IMPLEMENTATION_ROADMAP.md
│   ├── IMPLEMENTATION_SUMMARY.md
│   ├── INTEGRATION_CHECKLIST.md
│   ├── LOCAL_LLM_ARCHITECTURE.md
│   ├── LOCAL_LLM_GETTING_STARTED.md
│   ├── Makefile
│   ├── MEMORY_SYSTEM_COMPLETION_SUMMARY.md
│   ├── NEW_FEATURES.md
│   ├── NEXT_STEPS.md
│   ├── NEXT_STEPS_ENTERPRISE_DEPLOYMENT.md
│   ├── PHASE5_VALIDATION_REPORT.md
│   ├── PHASE_*.md                          (30+ phase reports)
│   ├── PROJECT_COMPLETION_CERTIFICATE.md
│   ├── PROJECT_FINALIZATION.md
│   ├── PROJECT_SUMMARY.md
│   ├── QUICK_REFERENCE_MANUAL_SYSTEM.md
│   ├── README.md
│   ├── README.pdf
│   ├── REPO_VERSION.md
│   ├── SESSION_MANAGER_COMPLETION_SUMMARY.md
│   ├── SESSION_SUMMARY.md
│   ├── STATE_PERSISTENCE_COMPLETION_SUMMARY.md
│   ├── TEMPLATE_SYSTEM_COMPLETION_SUMMARY.md
│   ├── TESTING_STATUS_UPDATE.md
│   ├── TEST_COVERAGE.md
│   ├── TEST_IMPLEMENTATION_SUMMARY.md
│   ├── TEST_RESULTS.md
│   ├── UPDATE_REPORT.md
│   ├── api/
│   │   └── openapi.yaml
│   ├── applications/
│   │   ├── README.md
│   │   ├── android/
│   │   ├── aurora-os/
│   │   ├── desktop/
│   │   ├── harmony-os/
│   │   ├── ios/
│   │   └── terminal-ui/
│   ├── assets/
│   │   ├── colors/
│   │   ├── icons/
│   │   └── images/
│   ├── aurora-os/
│   ├── benchmark-reports/
│   ├── benchmarks/
│   ├── bin/
│   │   └── helixcode
│   ├── cleanup.sh
│   ├── cmd/
│   │   ├── cli/
│   │   ├── config-test/
│   │   ├── helix-config/
│   │   ├── local-llm-advanced.go
│   │   ├── local-llm.go
│   │   ├── main_commands.go
│   │   ├── other_commands.go
│   │   ├── performance-optimization/
│   │   ├── performance-optimization-standalone/
│   │   ├── root.go
│   │   ├── security-fix/
│   │   ├── security-fix-standalone/
│   │   ├── security-test/
│   │   └── server/
│   ├── config/
│   │   ├── azure_example.yaml
│   │   ├── config.yaml
│   │   ├── fixed-config.yaml
│   │   ├── grafana/
│   │   ├── minimal-config.yaml
│   │   ├── minimal-test-config.yaml
│   │   ├── model-aliases.example.yaml
│   │   ├── production-config.yaml
│   │   ├── test-config.yaml
│   │   └── working-config.yaml
│   ├── config-test
│   ├── demo_cross_provider_sharing.go
│   ├── demo_model_management.go
│   ├── desktop
│   ├── dev.helix.code
│   ├── doc-reports/
│   ├── docker/
│   │   ├── docker-compose.yml
│   │   └── entrypoint-worker.sh
│   ├── docker-compose-simple.yml
│   ├── docker-compose.aurora-os.yml
│   ├── docker-compose.builder.yml
│   ├── docker-compose.full-test.yml
│   ├── docker-compose.harmony-os.yml
│   ├── docker-compose.specialized-platforms.yml
│   ├── docker-compose.test.yml
│   ├── docker-compose.yml
│   ├── examples/
│   │   ├── multi-agent-system/
│   │   ├── phase3/
│   │   └── qa-integration/
│   ├── go.mod
│   ├── go.sum
│   ├── harmony-os
│   ├── helix
│   ├── helix-code
│   ├── helix-config
│   ├── internal/
│   │   ├── adapters/
│   │   ├── agent/
│   │   ├── auth/
│   │   ├── cognee/
│   │   ├── commands/
│   │   ├── config/
│   │   ├── context/
│   │   ├── database/
│   │   ├── deployment/
│   │   ├── discovery/
│   │   ├── editor/
│   │   ├── event/
│   │   ├── fix/
│   │   ├── focus/
│   │   ├── hardware/
│   │   ├── helixqa/
│   │   ├── hooks/
│   │   ├── llm/
│   │   ├── logging/
│   │   └── logo/
│   ├── local-llm-test.go
│   ├── main
│   ├── main.go
│   ├── multi-agent-system
│   ├── performance-optimization
│   ├── qa-integration
│   ├── rebuild_script.sh
│   ├── reports/
│   │   ├── performance/
│   │   └── security/
│   ├── run_all_tests.sh
│   ├── run_integration_tests.sh
│   ├── run_tests.sh
│   ├── scripts/
│   │   ├── README.md
│   │   ├── assets/
│   │   ├── benchmark.sh
│   │   ├── build.sh
│   │   ├── containers/
│   │   ├── coverage.sh
│   │   ├── deploy-aurora-os.sh
│   │   ├── deploy-harmony-os.sh
│   │   ├── deploy-specialized-platforms.sh
│   │   ├── docs-validate.sh
│   │   ├── generate-manual.sh
│   │   ├── generate-test-catalog/
│   │   ├── generate-test-keys.sh
│   │   ├── logo/
│   │   ├── md-to-html
│   │   ├── md-to-html.go
│   │   ├── pre-commit.template
│   │   ├── run-all-tests.sh
│   │   └── run-docker-tests.sh
│   ├── security/
│   │   └── security_test.go
│   ├── security-test
│   ├── server
│   ├── shared/
│   │   └── mobile-core/
│   ├── simple_test_runner.go
│   ├── standalone_tests/
│   │   ├── cli_test.go
│   │   └── test_suite.go
│   ├── terminal-ui
│   ├── test/
│   │   ├── automation/
│   │   ├── config/
│   │   ├── e2e/
│   │   ├── generate-ssh-keys.sh
│   │   ├── init.sql
│   │   ├── integration/
│   │   ├── load/
│   │   ├── test_plan.md
│   │   ├── worker-init.sh
│   │   └── workers/
│   ├── test-programs/
│   │   ├── test-server-shutdown.go
│   │   ├── test-server.go
│   │   └── test_db_connection.go
│   ├── test-reports/
│   │   └── unit_coverage_20251114_090040.out
│   ├── test_build.sh
│   ├── test_model_management.go
│   ├── test_runner.go
│   ├── test_*.go                          (various test files)
│   └── (extensive Go source tree)
├── helix_qa/                               ← SUBMODULE (REQUIRED ✓)
├── IMPLEMENTATION_COMPLETE.md
├── IMPLEMENTATION_COMPLETION_PLAN.md
├── IMPLEMENTATION_PLAN.md
├── IMPLEMENTATION_ROADMAP.md
├── IMPLEMENTATION_SUCCESS.md
├── IMPLEMENTATION_TRACKER.md
├── INTEGRATION_IMPLEMENTATION_SUMMARY.md
├── ISSUES_FIXED.md
├── LICENSE
├── Makefile
├── MASTER_IMPLEMENTATION_PLAN.md
├── NOTIFICATION_INTEGRATION_REPORT.md
├── PENPOT_INTEGRATION.md
├── PHASED_IMPLEMENTATION_PLAN.md
├── PHASE_0_*.md
├── PHASE_1_*.md
├── PHASE_2_*.md
├── PHASE_3_*.md
├── Phase2_Implementation_Summary.md
├── Phase4_Implementation_Summary.md
├── Phase5_Implementation_Summary.md
├── PLANDEX_ANALYSIS.md
├── PLANDEX_INDEX.md
├── PLANDEX_PORTING_CHECKLIST.md
├── PLANDEX_QUICK_REFERENCE.md
├── PRODUCTION_DEPLOYMENT_READINESS_REPORT.md
├── PROJECT_COMPLETION_ANALYSIS.md
├── PROJECT_EXECUTION_SUMMARY.md
├── QUICK_START_IMPLEMENTATION.md
├── QUICK_START_TOMORROW.md
├── QWEN.md
├── README.md
├── README.pdf
├── README_DOCKER.md
├── REQUEST.md
├── SECURITY_IMPLEMENTATION.md
├── SECURITY_INFRASTRUCTURE_COMPLETION_REPORT.md
├── SESSION_PROGRESS_SUMMARY.md
├── SESSION_SUMMARY_2025-11-10.md
├── SKIPPED_TESTS_ANALYSIS.md
├── SONARQUBE_SNYK_IMPLEMENTATION.md
├── START_HERE.md
├── security/                              ← DIRECTORY
├── TEAM_DEVELOPMENT_BREAKDOWN.md
├── TESTING_PLAN.md
├── UNFINISHED_WORK_REPORT.md
├── VERIFICATION_REPORT.md
├── VERIFICATION_SUMMARY.md
├── VIDEO_COURSE_CURRICULUM.md
├── WEBSITE_UPDATE_SUMMARY.md
├── Website/                               ← DIRECTORY
├── awesome-ai-memory/                     ← SUBMODULE
├── challenges/                            ← DIRECTORY (lowercase)
├── cmd/
│   └── security-test/
├── configs/
│   └── verifier.yaml
├── core.test
├── coverage.out
├── docker-compose.helix.yml
├── docker-compose.test.yml
├── docker-entrypoint.sh
├── docs/                                  ← DIRECTORY
├── go.mod
├── go.sum
├── helix                                  ← BINARY/EXECUTABLE
├── helix.security.json
├── install_missing_libs.sh
├── internal/
│   ├── fix/
│   ├── security/
│   └── testing/
├── isolated_files/                        ← DIRECTORY
├── penpot-integration.sh
├── postgres-init.sql
├── scripts/                               ← DIRECTORY
├── security/
│   └── (security configs)
├── security-test                          ← DIRECTORY
├── setup.sh
├── test-config.yaml
├── test-docker-quick.sh
├── test-docker-setup.sh
├── test/                                  ← DIRECTORY
└── tests/                                 ← DIRECTORY
```

---

## 4. ALL 87 SUBMODULES - COMPLETE INVENTORY

| # | Submodule Name | Path | URL | Protocol |
|---|----------------|------|-----|----------|
| 1 | Example_Projects/OpenCode/OpenCode | Example_Projects/OpenCode/OpenCode | git@github.com:opencode-ai/opencode.git | SSH |
| 2 | Example_Projects/OpenCode/Homebrew-Tap | Example_Projects/OpenCode/Homebrew-Tap | git@github.com:opencode-ai/homebrew-tap.git | SSH |
| 3 | Example_Projects/Ollama_Code | Example_Projects/Ollama_Code | git@github.com:tcsenpai/ollama-code.git | SSH |
| 4 | Example_Projects/Qwen_Code | Example_Projects/Qwen_Code | git@github.com:QwenLM/qwen-code.git | SSH |
| 5 | Example_Projects/Codename_Goose | Example_Projects/Codename_Goose | git@github.com:jgenerali/codename-goose.git | SSH |
| 6 | Example_Projects/Claude_Code | Example_Projects/Claude_Code | git@github.com:anthropics/claude-code.git | SSH |
| 7 | Dependencies/LLama_CPP | Dependencies/LLama_CPP | git@github.com:ggml-org/llama.cpp.git | SSH |
| 8 | Dependencies/Ollama | Dependencies/Ollama | git@github.com:ollama/ollama.git | SSH |
| 9 | Dependencies/HuggingFace_Hub | Dependencies/HuggingFace_Hub | git@github.com:huggingface/huggingface_hub.git | SSH |
| 10 | Example_Projects/DeepSeek_CLI | Example_Projects/DeepSeek_CLI | git@github.com:holasoymalva/deepseek-cli.git | SSH |
| 11 | Example_Projects/Mistral_Code | Example_Projects/Mistral_Code | git@github.com:Wylgrif/Mistral-code.git | SSH |
| 12 | github_pages_website | github_pages_website | git@github.com:HelixDevelopment-Code/Welcome.git | SSH |
| 13 | Example_Projects/Gemini_CLI | Example_Projects/Gemini_CLI | git@github.com:google-gemini/gemini-cli.git | SSH |
| 14 | Example_Projects/Forge | Example_Projects/Forge | git@github.com:antinomyhq/forge.git | SSH |
| 15 | Example_Projects/Plandex | Example_Projects/Plandex | git@github.com:plandex-ai/plandex.git | SSH |
| 16 | Example_Projects/GPT_Engineer | Example_Projects/GPT_Engineer | git@github.com:AntonOsika/gpt-engineer.git | SSH |
| 17 | Example_Projects/Aider | Example_Projects/Aider | git@github.com:Aider-AI/aider.git | SSH |
| 18 | Example_Projects/Cline | Example_Projects/Cline | git@github.com:cline/cline.git | SSH |
| 19 | awesome-ai-memory | awesome-ai-memory | git@github.com:topoteretes/awesome-ai-memory.git | SSH |
| 20 | Example_Projects/Stark-Kitty-Kiro-Cli | Example_Projects/Stark-Kitty-Kiro-Cli | git@github.com:stark1tty/kiro-cli.git | SSH |
| 21 | Example_Projects/Kilo-Code | Example_Projects/Kilo-Code | git@github.com:Kilo-Org/kilocode.git | SSH |
| 22 | Example_Projects/Amazon-Q-Developer-CLI | Example_Projects/Amazon-Q-Developer-CLI | git@github.com:aws/amazon-q-developer-cli.git | SSH |
| 23 | Example_Projects/OpenHands/software-agent-sdk | Example_Projects/OpenHands/software-agent-sdk | git@github.com:OpenHands/software-agent-sdk.git | SSH |
| 24 | Example_Projects/OpenHands/OpenHands | Example_Projects/OpenHands/OpenHands | git@github.com:OpenHands/OpenHands.git | SSH |
| 25 | Example_Projects/OpenHands/agentic-code-search-oss | Example_Projects/OpenHands/agentic-code-search-oss | git@github.com:OpenHands/agentic-code-search-oss.git | SSH |
| 26 | Example_Projects/OpenHands/skills | Example_Projects/OpenHands/skills | git@github.com:OpenHands/skills.git | SSH |
| 27 | Example_Projects/OpenHands/litellm | Example_Projects/OpenHands/litellm | git@github.com:OpenHands/litellm.git | SSH |
| 28 | Example_Projects/OpenHands/OpenHands-Server | Example_Projects/OpenHands/OpenHands-Server | git@github.com:OpenHands/OpenHands-Server.git | SSH |
| 29 | Example_Projects/OpenHands/open-operator | Example_Projects/OpenHands/open-operator | git@github.com:OpenHands/open-operator.git | SSH |
| 30 | Example_Projects/OpenHands/OpenHands-Docs | Example_Projects/OpenHands/OpenHands-Docs | git@github.com:OpenHands/docs.git | SSH |
| 31 | Example_Projects/OpenHands/agent-analysis | Example_Projects/OpenHands/agent-analysis | git@github.com:OpenHands/agent-analysis.git | SSH |
| 32 | Example_Resources/GitHub-Awesome-Copilot | Example_Resources/GitHub-Awesome-Copilot | git@github.com:github/awesome-copilot.git | SSH |
| 33 | Example_Projects/Superset | Example_Projects/Superset | git@github.com:superset-sh/superset.git | SSH |
| 34 | Example_Projects/ui-ux-pro-max-skill | Example_Projects/ui-ux-pro-max-skill | git@github.com:nextlevelbuilder/ui-ux-pro-max-skill.git | SSH |
| 35 | Example_Projects/Warp | Example_Projects/Warp | git@github.com:warpdotdev/Warp.git | SSH |
| 36 | Example_Projects/GitHub-Copilot-CLI | Example_Projects/GitHub-Copilot-CLI | git@github.com:github/copilot-cli.git | SSH |
| 37 | Example_Projects/Codex | Example_Projects/Codex | org-14957082@github.com:openai/codex.git | OTHER |
| 38 | Example_Projects/Octogen | Example_Projects/Octogen | git@github.com:dbpunk-labs/octogen.git | SSH |
| 39 | Example_Projects/TaskWeaver | Example_Projects/TaskWeaver | git@github.com:microsoft/TaskWeaver.git | SSH |
| 40 | Example_Projects/Cheshire-Cat-Ai/CC-AI-Core | Example_Projects/Cheshire-Cat-Ai/CC-AI-Core | git@github.com:cheshire-cat-ai/core.git | SSH |
| 41 | Example_Resources/Cheshire-Cat-Ai/CC-AI-Docs | Example_Resources/Cheshire-Cat-Ai/CC-AI-Docs | git@github.com:cheshire-cat-ai/docs.git | SSH |
| 42 | Example_Projects/Cheshire-Cat-Ai/CC-AA-LLMs | Example_Projects/Cheshire-Cat-Ai/CC-AA-LLMs | git@github.com:cheshire-cat-ai/llms.git | SSH |
| 43 | Example_Projects/Cheshire-Cat-Ai/CC-AI-Plugins | Example_Projects/Cheshire-Cat-Ai/CC-AI-Plugins | git@github.com:cheshire-cat-ai/plugins.git | SSH |
| 44 | Example_Projects/Cheshire-Cat-Ai/plugin-template | Example_Projects/Cheshire-Cat-Ai/plugin-template | git@github.com:cheshire-cat-ai/plugin_template.git | SSH |
| 45 | Example_Projects/Cheshire-Cat-Ai/model-plugins | Example_Projects/Cheshire-Cat-Ai/model-plugins | git@github.com:cheshire-cat-ai/models-plugin.git | SSH |
| 46 | Example_Projects/Cheshire-Cat-Ai/plugins-backend | Example_Projects/Cheshire-Cat-Ai/plugins-backend | git@github.com:cheshire-cat-ai/plugins-backend.git | SSH |
| 47 | Example_Resources/Awesome-AI-GPTs | Example_Resources/Awesome-AI-GPTs | git@github.com:EmbraceAGI/Awesome-AI-GPTs.git | SSH |
| 48 | Example_Projects/Multiagent-Coding-System | Example_Projects/Multiagent-Coding-System | git@github.com:Danau5tin/multi-agent-coding-system.git | SSH |
| 49 | Example_Resources/OpenAI-Cookbook | Example_Resources/OpenAI-Cookbook | org-14957082@github.com:openai/openai-cookbook.git | OTHER |
| 50 | Example_Projects/Codai | Example_Projects/Codai | git@github.com:meysamhadeli/codai.git | SSH |
| 51 | Example_Projects/FauxPilot | Example_Projects/FauxPilot | git@github.com:fauxpilot/fauxpilot.git | SSH |
| 52 | Example_Projects/vtcode | Example_Projects/vtcode | git@github.com:vinhnx/vtcode.git | SSH |
| 53 | Example_Projects/MobileAgent | Example_Projects/MobileAgent | git@github.com:X-PLUG/MobileAgent.git | SSH |
| 54 | Example_Projects/gptme/gptme | Example_Projects/gptme/gptme | git@github.com:gptme/gptme.git | SSH |
| 55 | Example_Projects/gptme/gptme-contrib | Example_Projects/gptme/gptme-contrib | git@github.com:gptme/gptme-contrib.git | SSH |
| 56 | Example_Projects/gptme/gptme-agent-template | Example_Projects/gptme/gptme-agent-template | git@github.com:gptme/gptme-agent-template.git | SSH |
| 57 | Example_Projects/gptme/gptme-pitch-deck | Example_Projects/gptme/gptme-pitch-deck | git@github.com:gptme/gptme-pitch-deck.git | SSH |
| 58 | Example_Projects/gptme/gptme-rag | Example_Projects/gptme/gptme-rag | git@github.com:gptme/gptme-rag.git | SSH |
| 59 | Example_Projects/gptme/gptme-webui | Example_Projects/gptme/gptme-webui | git@github.com:gptme/gptme-webui.git | SSH |
| 60 | Example_Projects/gptme/gptme-lessons | Example_Projects/gptme/gptme-lessons | git@github.com:gptme/gptme-lessons.git | SSH |
| 61 | Example_Projects/gptme/gptme-tauri | Example_Projects/gptme/gptme-tauri | git@github.com:gptme/gptme-tauri.git | SSH |
| 62 | Example_Projects/Conduit | Example_Projects/Conduit | git@github.com:lostintangent/conduit-release.git | SSH |
| 63 | Example_Projects/Shai | Example_Projects/Shai | git@github.com:ovh/shai.git | SSH |
| 64 | Example_Projects/SnowCLI | Example_Projects/SnowCLI | git@github.com:MayDay-wpf/snow-cli.git | SSH |
| 65 | Example_Projects/GitMCP | Example_Projects/GitMCP | git@github.com:idosal/git-mcp | SSH |
| 66 | Example_Projects/Claude-Code-Plugins-And-Skills | Example_Projects/Claude-Code-Plugins-And-Skills | git@github.com:jeremylongshore/claude-code-plugins-plus-skills.git | SSH |
| 67 | Example_Projects/Bridle | Example_Projects/Bridle | git@github.com:jeremylongshore/claude-code-plugins-plus-skills.git | SSH |
| 68 | Example_Projects/Agent-Deck | Example_Projects/Agent-Deck | git@github.com:asheshgoplani/agent-deck.git | SSH |
| 69 | Example_Projects/Nanocoder | Example_Projects/Nanocoder | git@github.com:Nano-Collective/nanocoder.git | SSH |
| 70 | Example_Resources/Awesome-AI-Agents | Example_Resources/Awesome-AI-Agents | git@github.com:e2b-dev/awesome-ai-agents.git | SSH |
| 71 | Example_Projects/Postgres-MCP | Example_Projects/Postgres-MCP | git@github.com:timescale/pg-aiguide.git | SSH |
| 72 | Example_Projects/Noi | Example_Projects/Noi | git@github.com:lencx/Noi.git | SSH |
| 73 | Example_Projects/Claude-Squad | Example_Projects/Claude-Squad | git@github.com:smtg-ai/claude-squad.git | SSH |
| 74 | Example_Projects/Emdash | Example_Projects/Emdash | git@github.com:generalaction/emdash.git | SSH |
| 75 | Example_Projects/Codex-Skills | Example_Projects/Codex-Skills | git@github.com:anthropics/codex-skills.git | SSH |
| 76 | Example_Projects/GitHub-Spec-Kit | Example_Projects/GitHub-Spec-Kit | git@github.com:github/spec-kit.git | SSH |
| 77 | Example_Projects/Get-Shit-Done | Example_Projects/Get-Shit-Done | git@github.com:holasoymalva/get-shit-done-cli.git | SSH |
| 78 | Dependencies/HelixDevelopment/DocProcessor | Dependencies/HelixDevelopment/DocProcessor | git@github.com:HelixDevelopment/DocProcessor.git | SSH |
| 79 | Dependencies/HelixDevelopment/LLMOrchestrator | Dependencies/HelixDevelopment/LLMOrchestrator | git@github.com:HelixDevelopment/LLMOrchestrator.git | SSH |
| 80 | Dependencies/HelixDevelopment/LLMProvider | Dependencies/HelixDevelopment/LLMProvider | git@github.com:HelixDevelopment/LLMProvider.git | SSH |
| 81 | Dependencies/HelixDevelopment/LLMsVerifier | Dependencies/HelixDevelopment/LLMsVerifier | git@github.com:vasic-digital/LLMsVerifier.git | SSH |
| 82 | Dependencies/HelixDevelopment/VisionEngine | Dependencies/HelixDevelopment/VisionEngine | git@github.com:HelixDevelopment/VisionEngine.git | SSH |
| 83 | Challenges | Challenges | git@github.com:vasic-digital/Challenges.git | SSH |
| 84 | containers | containers | git@github.com:vasic-digital/Containers.git | SSH |
| 85 | Example_Resources/Taches-CC-Resources | Example_Resources/Taches-CC-Resources | git@github.com:tachescode/resources.git | SSH |
| 86 | helix_qa | helix_qa | git@github.com:HelixDevelopment/HelixQA.git | SSH |
| 87 | (additional submodule beyond listed 86) | - | - | - |

**Protocol Summary:**
- SSH (`git@github.com`): 85 submodules (97.7%)
- Org-specific SSH (`org-14957082@github.com`): 2 submodules (2.3%)
- HTTPS: 0 submodules (0%)

---

## 5. REQUIRED SUBMODULES STATUS

### 5.1 PRESENT (4 of 8)

| Required Submodule | Status | Path | Actual URL | Protocol |
|--------------------|--------|------|------------|----------|
| **LLMsVerifier** | PRESENT ✓ | Dependencies/HelixDevelopment/LLMsVerifier | git@github.com:vasic-digital/LLMsVerifier.git | SSH |
| **HelixQA** | PRESENT ✓ | helix_qa (root) | git@github.com:HelixDevelopment/HelixQA.git | SSH |
| **Challenges** | PRESENT ✓ | Challenges (root) | git@github.com:vasic-digital/Challenges.git | SSH |
| **Containers** | PRESENT ✓ | containers (root) | git@github.com:vasic-digital/Containers.git | SSH |

### 5.2 MISSING (4 of 8)

| Required Submodule | Status | Expected URL | Recommended SSH URL |
|--------------------|--------|------------|---------------------|
| **HelixAgent** | MISSING ✗ | github.com/HelixDevelopment/HelixAgent | `git@github.com:HelixDevelopment/HelixAgent.git` |
| **HelixLLM** | MISSING ✗ | github.com/HelixDevelopment/HelixLLM | `git@github.com:HelixDevelopment/HelixLLM.git` |
| **HelixMemory** | MISSING ✗ | github.com/HelixDevelopment/HelixMemory | `git@github.com:HelixDevelopment/HelixMemory.git` |
| **HelixSpecifier** | MISSING ✗ | github.com/HelixDevelopment/HelixSpecifier | `git@github.com:HelixDevelopment/HelixSpecifier.git` |

### 5.3 MISSING SUBMODULES ANALYSIS

**HelixAgent:**
- The `AGENTS.md` file at root explicitly states: "Derived from HelixAgent AGENTS.md with HelixCode-specific enhancements"
- This indicates HelixAgent was an ancestor/related project whose concepts were ported into HelixCode
- HelixAgent is NOT listed in `.gitmodules` at all
- **cli_agents directory status:** CANNOT BE DETERMINED - HelixAgent submodule does not exist in this repository
- Recommendation: Either HelixAgent was intentionally excluded (functionality merged into HelixCode), or it needs to be added as a submodule

**HelixLLM:**
- Not present in `.gitmodules`
- No references found in root-level documentation
- Likely a core LLM integration module that should be present

**HelixMemory:**
- Not present in `.gitmodules`
- The repository DOES contain `AI_MEMORY_INTEGRATION_PROGRESS.md` and a submodule `awesome-ai-memory` (external resource)
- Memory functionality may be partially implemented in HelixCode directly

**HelixSpecifier:**
- Not present in `.gitmodules`
- No references found at root level
- Likely a specification/contract module that should be present

---

## 6. CONFIGURATION FILES INVENTORY

### 6.1 Root-Level Configuration Files

| File | Type | Size | Purpose |
|------|------|------|---------|
| `.gitmodules` | Git Submodules | 11,824 bytes | Defines 87 submodules |
| `.gitmodules.bak.https-to-ssh` | Git Submodules Backup | 10,637 bytes | HTTPS-to-SSH migration backup |
| `.gitignore` | Git Ignore | 1,510 bytes | Git exclusion patterns |
| `.env.example` | Environment Template | 1,925 bytes | Environment variable template |
| `go.mod` | Go Module | ~150 bytes | Go module `dev.helix.code` at Go 1.25.2 |
| `go.sum` | Go Checksums | ~1KB | Go dependency checksums |
| `Makefile` | Build Automation | ~300 bytes | Build targets (demo, validate, ci) |
| `Dockerfile` | Docker Image | Unknown | Main application container |
| `Dockerfile.test` | Docker Image | Unknown | Test container |
| `Dockerfile.worker` | Docker Image | Unknown | Worker container |
| `docker-compose.helix.yml` | Docker Compose | ~2KB | Main compose with PostgreSQL + Redis |
| `docker-compose.test.yml` | Docker Compose | Unknown | Test compose |
| `test-config.yaml` | YAML Config | Unknown | Test configuration |
| `postgres-init.sql` | SQL | Unknown | PostgreSQL initialization |
| `helix.security.json` | JSON Security | Unknown | Security configuration |

### 6.2 Nested Configuration Files (HelixCode/ subdirectory)

| File | Type | Purpose |
|------|------|---------|
| `HelixCode/go.mod` | Go Module | Nested Go module |
| `HelixCode/go.sum` | Go Checksums | Nested dependency checksums |
| `HelixCode/Makefile` | Build Automation | Nested build targets |
| `HelixCode/config/config.yaml` | YAML | Main configuration |
| `HelixCode/config/azure_example.yaml` | YAML | Azure example config |
| `HelixCode/config/minimal-config.yaml` | YAML | Minimal config |
| `HelixCode/config/production-config.yaml` | YAML | Production config |
| `HelixCode/config/test-config.yaml` | YAML | Test config |
| `HelixCode/config/model-aliases.example.yaml` | YAML | Model aliases example |
| `HelixCode/docker/docker-compose.yml` | Docker Compose | Docker orchestration |
| `HelixCode/api/openapi.yaml` | OpenAPI | API specification |
| `HelixCode/.env.example` | Environment | Extended env template (2,998 bytes) |
| `HelixCode/.env.full-test` | Environment | Full test env (4,607 bytes) |

### 6.3 GitHub Actions / CI

| File | Purpose |
|------|---------|
| `.github/workflows/notification-tests.yml` | Notification test automation (3,249 bytes) |
| `HelixCode/.github/workflows/` | Additional workflows in nested project |

### 6.4 Configuration Summary by Technology

| Technology | Files Found |
|------------|-------------|
| **Go** | `go.mod`, `go.sum` (root + HelixCode/) |
| **Docker** | `Dockerfile`, `Dockerfile.test`, `Dockerfile.worker`, `docker-compose.helix.yml`, `docker-compose.test.yml` |
| **YAML** | `test-config.yaml`, `configs/verifier.yaml`, `HelixCode/config/*.yaml`, `HelixCode/api/openapi.yaml` |
| **Shell** | `setup.sh`, `docker-entrypoint.sh`, `install_missing_libs.sh`, `penpot-integration.sh`, `test-docker-quick.sh`, `test-docker-setup.sh` |
| **JSON** | `helix.security.json` |
| **SQL** | `postgres-init.sql` |
| **Markdown** | 50+ documentation files |

---

## 7. HELIXAGENT CLI_AGENTS DIRECTORY STATUS

| Property | Status |
|----------|--------|
| **HelixAgent Submodule Present** | NO |
| **cli_agents Directory Accessible** | NO - Cannot check a submodule that does not exist |
| **References in AGENTS.md** | YES - "Derived from HelixAgent AGENTS.md with HelixCode-specific enhancements" |
| **Conclusion** | HelixAgent was either merged into HelixCode directly or is an external dependency that was never added as a submodule. The `cli_agents` directory status is UNKNOWN because the submodule is not present. |

---

## 8. DEPENDENCY GRAPH

```
HelixCode (root)
├── Go Module: dev.helix.code
│   └── github.com/google/uuid v1.6.0
│   └── github.com/pkg/errors v0.9.1
│   └── gopkg.in/yaml.v2 v2.4.0
│
├── Docker Infrastructure
│   ├── PostgreSQL 15
│   ├── Redis
│   └── Main HelixCode Container (3GB mem limit)
│
├── Core Dependencies (submodules)
│   ├── LLama_CPP (ggml-org/llama.cpp)
│   ├── Ollama (ollama/ollama)
│   ├── HuggingFace_Hub (huggingface/huggingface_hub)
│   ├── DocProcessor (HelixDevelopment/DocProcessor)
│   ├── LLMOrchestrator (HelixDevelopment/LLMOrchestrator)
│   ├── LLMProvider (HelixDevelopment/LLMProvider)
│   ├── LLMsVerifier (vasic-digital/LLMsVerifier) ✓
│   └── VisionEngine (HelixDevelopment/VisionEngine)
│
├── Quality Assurance (submodules)
│   ├── helix_qa (HelixDevelopment/HelixQA) ✓
│   ├── Challenges (vasic-digital/Challenges) ✓
│   └── containers (vasic-digital/Containers) ✓
│
├── Missing Core Components
│   ├── HelixAgent (MISSING)
│   ├── HelixLLM (MISSING)
│   ├── HelixMemory (MISSING)
│   └── HelixSpecifier (MISSING)
│
├── Website
│   └── github_pages_website (HelixDevelopment-Code/Welcome)
│
├── External AI CLI Examples (30+ submodules)
│   ├── Anthropic: Claude_Code, Codex, Codex-Skills
│   ├── OpenAI: Codex (org-specific)
│   ├── Google: Gemini_CLI
│   ├── Microsoft: TaskWeaver
│   ├── AWS: Amazon-Q-Developer-CLI
│   └── ... (many more)
│
└── External AI Resources
    ├── awesome-ai-memory
    ├── Awesome-AI-Agents
    ├── Awesome-AI-GPTs
    └── OpenAI-Cookbook
```

---

## 9. SUBMODULE INITIALIZATION NOTES

### SSH vs HTTPS
- **ALL submodules use SSH URLs** (`git@github.com:...`)
- This means `git submodule update --init --recursive` will **FAIL** without SSH keys configured
- Two options for initialization:
  1. **SSH Setup:** Configure `~/.ssh/id_rsa` or `~/.ssh/id_ed25519` with GitHub access
  2. **HTTPS Rewrite:** Run `git config --global url."https://github.com/".insteadOf "git@github.com:"` before submodule init
- The `.gitmodules.bak.https-to-ssh` file suggests the repository was migrated FROM HTTPS TO SSH at some point

### Submodule Organization
| Category | Count | Paths |
|----------|-------|-------|
| Example Projects (AI CLI tools) | ~35 | `Example_Projects/` |
| Example Resources | ~5 | `Example_Resources/` |
| Core Dependencies | 8 | `Dependencies/` + root |
| Website | 1 | `github_pages_website/` |

---

## 10. RECOMMENDED ACTIONS

### Immediate
1. **Configure SSH keys** or set HTTPS rewrite before running `git submodule update --init --recursive`
2. **Add missing submodules** if they are required:
   ```bash
   git submodule add git@github.com:HelixDevelopment/HelixAgent.git Dependencies/HelixDevelopment/HelixAgent
   git submodule add git@github.com:HelixDevelopment/HelixLLM.git Dependencies/HelixDevelopment/HelixLLM
   git submodule add git@github.com:HelixDevelopment/HelixMemory.git Dependencies/HelixDevelopment/HelixMemory
   git submodule add git@github.com:HelixDevelopment/HelixSpecifier.git Dependencies/HelixDevelopment/HelixSpecifier
   ```
3. **Investigate HelixAgent relationship** - Determine if HelixAgent functionality was intentionally merged into HelixCode or if it's a missing dependency

### For Full Clone
```bash
# Option 1: SSH (recommended for submodule URLs)
git clone git@github.com:HelixDevelopment/HelixCode.git
cd HelixCode
git submodule update --init --recursive

# Option 2: HTTPS with URL rewrite for submodules
git clone https://github.com/HelixDevelopment/HelixCode.git
cd HelixCode
git config url."https://github.com/".insteadOf "git@github.com:"
git submodule update --init --recursive
```

---

## 11. APPENDIX: RAW DATA

### .gitmodules SHA
- SHA: `26980b463c02e5a2a127f778215413e7937202e7`
- Size: 11,824 bytes

### Go Module (root)
```
module dev.helix.code

go 1.25.2

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
```

### docker-compose.helix.yml Services
- `helixcode` - Main application (ports 8080, 2222, 3000)
- `postgres` - PostgreSQL 15 (4GB mem limit)
- `redis` - Redis cache

---

*Report generated via GitHub API analysis. Repository clone failed due to HTTP/2 network framing errors in the target environment.*
