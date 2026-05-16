# HelixCode CLI Agent Integration — Quick Reference Card

## SSH Submodule Commands (Run These First)

```bash
# 1. Verify SSH access
ssh -T git@github.com

# 2. Add 4 MISSING submodules via SSH (NOT HTTPS)
git submodule add --force git@github.com:HelixDevelopment/HelixAgent.git dependencies/HelixDevelopment/HelixAgent
git submodule add --force git@github.com:HelixDevelopment/HelixLLM.git dependencies/HelixDevelopment/HelixLLM
git submodule add --force git@github.com:HelixDevelopment/HelixMemory.git dependencies/HelixDevelopment/HelixMemory
git submodule add --force git@github.com:HelixDevelopment/HelixSpecifier.git dependencies/HelixDevelopment/HelixSpecifier

# 3. Initialize ALL submodules recursively
git submodule update --init --recursive

# 4. Verify no HTTPS URLs exist
grep -n "https://github.com" .gitmodules  # Should return NOTHING

# 5. Create Go workspace and build
cat > go.work << 'EOF'
go 1.26
use (
    .
    ./HelixCode
    ./dependencies/HelixDevelopment/LLMsVerifier
    ./dependencies/HelixDevelopment/HelixAgent
    ./dependencies/HelixDevelopment/HelixLLM
    ./dependencies/HelixDevelopment/HelixMemory
    ./dependencies/HelixDevelopment/HelixSpecifier
    ./HelixQA
    ./Challenges
    ./Containers
)
EOF
go work sync
go build ./...

# 6. Commit
git add .gitmodules go.work
git commit -m "feat: Add missing submodules via SSH"
```

---

## Integration Phases at a Glance

| Phase | Duration | Key Deliverable | Risk |
|-------|----------|-----------------|------|
| **0** Foundation | 5-7 days | All 11 submodules initialized | HIGH |
| **1** Core Infra | 14-18 days | LLM gateway + memory + spec | HIGH |
| **2** CLI Foundation | 18-22 days | Tool framework + context + edit | MED-HIGH |
| **3** Power Features | 21-28 days | Plan mode + sandbox + MCP | HIGH |
| **4** UI/UX | 14-18 days | Streaming TUI + themes | MED |
| **5** Testing | 14-18 days | 100% coverage + QA | MED |

**Total: ~63 working days**

---

## Top 10 Features to Port First

1. **OS-Native Sandboxed Execution** (Codex) — CRITICAL security
2. **Auto-Compaction System** (Claude Code) — Infinite conversations
3. **Permission Rule System** (Claude Code) — 5 modes + wildcards
4. **LSP Integration** (OpenCode) — Semantic code understanding
5. **4-Layer Fuzzy Matching** (Aider) — Robust search/replace
6. **Cumulative Diff Review** (Plandex) — Review ALL before applying ANY
7. **Subagent Delegation** (Kilo Code) — Recursive agent hierarchy
8. **MCP Full Lifecycle** (Claude Code) — 4 transports + OAuth
9. **Plan/Act Dual-Mode** (Cline) — Separate planning from execution
10. **Shadow Git Checkpoints** (Cline) — Granular rollback

---

## Key Files Generated

| File | Size | Purpose |
|------|------|---------|
| `stage1_helixcode_mapping.md` | 43KB | Complete repo tree + 87 submodules |
| `stage1_standalone_repos_mapping.md` | 7KB | 8 standalone repos analyzed |
| `stage2_helixcode_architecture.md` | 38KB | Internal architecture deep dive |
| `stage2_claude_code_deep_analysis.md` | 43KB | PRIMARY agent — 20 top features |
| `stage2_cli_agents_catalog.md` | 36KB | 60+ CLI agents cataloged |
| `stage2_multi_cli_comparison.md` | 60KB | 10 agent comparison matrix |
| `stage3_gap_analysis.md` | 64KB | 137 features, 54 gaps identified |
| `stage4_integration_plan.md` | 84KB | 60 tasks, 6 phases, critical path |
| `stage4_technical_documentation.md` | 519KB | 29 features with Go code examples |
| `stage4_testing_strategy.md` | 179KB | 5,245+ tests across 8 categories |
| `HELIX_MASTER_DOCUMENTATION.md` | 39KB | Executive summary + compiled master |
| `Helix_Master_Documentation.docx` | 33KB | Word version of master document |
| `integrate_submodules.sh` | 8KB | Automated SSH submodule integration script |

---

## Critical Stats

- **Repositories**: 9 analyzed (HelixCode + 8 submodules)
- **Submodules in HelixCode**: 87 declared, 4 missing, 4 empty
- **CLI Agents Cataloged**: 60+
- **Features Evaluated**: 137
- **Gaps to Close**: 54 (18 P0 critical)
- **Tests Required**: 5,245+
- **Estimated Effort**: 63 working days (~3 months, 3-4 engineers)
