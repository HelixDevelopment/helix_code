# HXC-037 — §11.4.103-141 + CONST-048..060 anchor cascade backfill into 7 owned submodules

## Evidence run: 2026-06-09

### BEFORE (this session start): 30 cascade failures across 7 submodules
(captured earlier: debate_orchestrator/doc_processor/event_bus/helix_agent/llm_ops/llm_orchestrator/llm_provider × {CONSTITUTION,CLAUDE,AGENTS}.md missing §11.9 + CONST-048..060 + §11.4.103-121 heading anchors)

### AFTER: verify-governance-cascade.sh (live re-run)
```
PASS
```

### Fix: scripts/backfill_anchor_cascade.sh — deterministic, additive-only (§11.4.122), idempotent, verbatim golden (helix_qa) cascade
### Submodule commits (pushed to origin, fast-forward, no force §11.4.113):
- helix_agent: 480fa854
- debate_orchestrator: c9b51f9
- doc_processor: ea11bbf
- event_bus: ac1e7aa
- llm_ops: 3f7bac5
- llm_orchestrator: 0bb1e37
- llm_provider: 8034101
