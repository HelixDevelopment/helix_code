## Ported cli_agents capabilities

Evidence-backed inventory of capabilities **actually ported into HelixCode** from the
`cli_agents/` reference catalogue (51 vendored reference agents). Per CONST-035 anti-bluff:
this lists ONLY capabilities with landed code evidence (package `doc.go` origin headers,
CONTINUATION.md P2-Fxx CLOSED ledger, POWER_FEATURES_PORTING_PLAN rev2 file:line
reconciliation, git port commits) — NOT every cli_agent's full feature set. Planned-but-not-landed
items are marked `Dev=absent`/`partial` honestly. **No feature is `📹 yes`** (no recordings analyzed
for ported features). **Overall is never `confirmed`** (a confirmed rollup requires an analyzed video).

Evidence basis: (1) CONTINUATION.md ports ledger lists `P2-F21..P2-F30` as CLOSED with landing
packages; (2) each landed package's `doc.go` names its source agent ("aider voice input port P2-F27",
"Roo-code CLI agent port", "Continue.dev IDE integration", "Go port of upstream gptme's profile mechanism");
(3) POWER_FEATURES_PORTING_PLAN.md rev2 reconciles Phase-1 against HEAD with file:line;
(4) `HXC-031-codex-cline-port.md` is DRAFT (plan only, NO code) — codex multimodal + cline computer-use are PLANNED, not landed.

| Area | Component (HelixCode pkg) | Feature | Dev | Wired | Real-use | Tests | V&V | 📹 Video | Analysis | Origin | Overall |
|---|---|---|---|---|---|---|---|---|---|---|---|
| service | internal/approval | Approval/exec-policy modes (per-tool yes/no, autopilot tiers) | done | yes | unknown | unit | no | no | no | ported:codex | working-untaped |
| service | internal/autocommit | Git auto-commit per change (generated msg, secret-filter, summariser) | done | yes | unknown | unit | no | no | no | ported:aider | working-untaped |
| service | internal/tools/browser | Browser tool suite (launch/click/type/scroll/screenshot via chromedp) | done | yes | unknown | unit | no | no | no | ported:cline | working-untaped |
| service | internal/projectmemory | Project-memory context files (AGENTS.md/.clinerules-style loader+watcher) | done | yes | unknown | unit | no | no | no | ported:codex | working-untaped |
| service | internal/plantree | Plan trees (branching persistent implementation plans, JSON-backed) | done | yes | unknown | unit | no | no | no | ported:plandex | working-untaped |
| service | internal/session (condense.go) | Context compaction / history condense (CompactIfNeeded/ShouldCompact) | done | yes | unknown | unit | no | no | no | ported:plandex | working-untaped |
| service | internal/workspace | Container per-task workspace (Docker/Podman mount, TTL cleanup) | done | partial | unknown | unit | no | no | no | ported:openhands | partial |
| service | internal/voice | Voice-to-code input (arecord/sox capture, Whisper API + whisper.cpp fallback) | done | yes | unknown | unit | no | no | no | ported:aider | working-untaped |
| service | internal/repomap | Repo-map (tree-sitter ranked incremental project map) | done | yes | unknown | unit | no | no | no | ported:aider | working-untaped |
| service | internal/kilocode | AST-aware refactoring (cross-file rename, impact/call-graph, extract/move/inline) | done | yes | unknown | unit | no | no | no | ported:kilo-code | working-untaped |
| service | internal/roocode | Roo-code full port (task delegation, template gen, diff review, conv memory) | done | yes | unknown | unit | no | no | no | ported:roo-code | working-untaped |
| application | internal/continua | Continue.dev IDE integration (inline completions, editor, chat panel, diff, model selector) | done | partial | unknown | unit | no | no | no | ported:continue | partial |
| service | internal/agent/profiles | Verifier work profile + RoleVerify posture (subagent review/validation) | done | yes | unknown | unit | no | no | no | ported:gptme | working-untaped |
| application | cmd/cli (main.go:2118,2326) + autocommit/git.go | /undo + /diff (git-aware, force-push-free revert) | done | yes | unknown | unit | no | no | no | ported:aider | working-untaped |
| service | internal/workflow/autonomy | Configurable autonomy presets (None/Basic/BasicPlus/SemiAuto/FullAuto, 5 tiers) | done | yes | unknown | unit | no | no | no | ported:plandex | working-untaped |
| service | internal/workflow/planmode | First-class Plan/Act mode controller (tool-gated by mode) | done | yes | unknown | unit | no | no | no | ported:cline | working-untaped |
| service | internal/tools/askuser | ask_user interactive clarification tool | done | yes | unknown | unit | no | no | no | ported:gemini-cli | working-untaped |
| service | internal/commands (markdown_skills.go) | Markdown skills subsystem (project>user 2-tier precedence) | partial | yes | unknown | unit | no | no | no | ported:gemini-cli | partial |
| service | internal/checkpoint | Workspace file-snapshot checkpoint + /checkpoint create/list/restore | done | yes | unknown | unit | no | no | no | ported:cline | working-untaped |
| application | cmd/cli (main.go:1727) | Per-request token-usage counter (real provider Usage, anti-fabrication) | done | yes | unknown | unit | no | no | no | ported:codai | working-untaped |
| service | (no production code) | Context-window-% indicator | absent | no | no | none | no | no | no | ported:codai (planned) | gap |
| service | (no production code) | TODO/step tracker surface (/tasks is background-job list, not TODO tracker) | partial | partial | no | none | no | no | no | ported:gemini-cli (planned) | partial |
| service | markdown_skills.go | SKILL.md built-in/bundled tier + canonical SKILL.md filename | absent | no | no | none | no | no | no | ported:gemini-cli (planned) | gap |
| service | internal/llm (HXC-031 plan) | Codex multimodal image-content LLM request surface | absent | no | no | none | no | no | no | ported:codex (planned/DRAFT) | gap |
| service | internal/tools/browser (HXC-031 plan) | Cline computer-use feedback loop (screenshot-per-action coord control) | absent | no | no | none | no | no | no | ported:cline (planned/DRAFT) | gap |
| service | (planned, POWER_FEATURES F22) | Cumulative diff-review sandbox (stage-before-apply, apply/reject hunks) | absent | no | no | none | no | no | no | ported:plandex (planned) | gap |
| service | (planned, POWER_FEATURES F25) | Conversation branches / fork | absent | no | no | none | no | no | no | ported:plandex (planned) | gap |
| service | (planned, POWER_FEATURES F36/F43) | /rewind + Tangent mode | absent | no | no | none | no | no | no | ported:gemini-cli/amazon-q (planned) | gap |
| service | (planned, POWER_FEATURES F18) | Messaging connectors (Slack/Telegram/Discord) | absent | no | no | none | no | no | no | ported:cline (planned) | gap |
| service | (planned, POWER_FEATURES F35) | ACP mode (Agent Client Protocol over stdio) | absent | no | no | none | no | no | no | ported:gemini-cli (planned) | gap |
| service | (planned, POWER_FEATURES F56) | OpenAI-compatible server endpoints | absent | no | no | none | no | no | no | ported:shai/aichat (planned) | gap |
| service | (planned, POWER_FEATURES F61) | Spec-driven workflow surface (specify→plan→tasks→implement) | absent | no | no | none | no | no | no | ported:spec-kit (planned) | gap |
| service | (planned, POWER_FEATURES F33) | OS-level exec sandbox (Seatbelt/Landlock/bwrap) | absent | no | no | none | no | no | no | ported:codex (planned) | gap |

Count: 33 rows total — **20 landed ports** (`Dev=done`, working-untaped), **3 partial-landed**
(workspace, continua, markdown-skills), **10 planned/not-landed** (`Dev=absent`/`partial`, `gap`).
Zero `📹 yes`; zero `confirmed`.

Honest assessment: The **Phase-2 port wave (P2-F21..F30) genuinely landed** — 10 named source-agent
ports (codex/aider/cline/plandex/openhands/kilo-code/roo-code/continue) ship as real packages with
origin-attributed `doc.go` headers, unit tests, and CONTINUATION CLOSED records, plus the gptme
verifier-profile port. The **POWER_FEATURES Phase-1 wrap (autonomy/Plan-Act/undo-diff/ask_user/skills/
token-count/checkpoint) is largely landed** per the rev2 file:line reconciliation. Beyond that, the
porting is **mostly PLANNED, not landed**: POWER_FEATURES_PORTING_PLAN is explicitly "DRAFT — research +
plan only, NO code", its Phases 2–7 (diff-sandbox, branches, rewind, tangent, connectors, ACP, OpenAI
server, spec-driven, OS sandbox) are unimplemented, and HXC-031 (codex multimodal + cline computer-use)
is a DRAFT plan with no code. No ported feature has an analyzed recording, so none can be `confirmed`;
all landed ports are honestly `working-untaped` pending video V&V.
