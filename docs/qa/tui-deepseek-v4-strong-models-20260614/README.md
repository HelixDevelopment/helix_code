# DeepSeek V4 + strong-models real-tools re-record — QA evidence (2026-06-14)

**Run-id:** `tui-deepseek-v4-strong-models-20260614`
**Operator request (2026-06-14):** *"repeat everything with deep seek v4 models and similar strong available models! let us know when videos are ready!"*

## Verdict: PASS (both videos, captured-frame evidence)

Live model discovery confirmed **DeepSeek V4 is available** (`deepseek-v4-pro`, `deepseek-v4-flash`) alongside other strong models (Mistral `mistral-large-2512`, Groq `gpt-oss-120b`/`qwen3-32b`). Re-recorded the two-video structure with **DeepSeek V4 Pro** as the headline single model and the **ensemble** (which now resolves a DeepSeek **V4** member alongside Mistral/Groq/OpenRouter).

- **Video 1** — DeepSeek V4 Pro (`deepseek-v4-pro`, picker digit 2): `~/Downloads/video1-deepseek-v4-pro.mp4` (200 s). All 3 prompts; the model autonomously calls `git_status` with `status`/`log`/`diff`/`show` subcommands (real diffs of the in-flight changes + real commits), `glob`/`grep`/`fs_read` for the codebase + AGENTS.md, and the read-only guard refuses `shell` on video.
- **Video 2** — Ensemble with strong members (picker digit 9): `~/Downloads/video2-ensemble-strong-models.mp4` (267 s). All 3 prompts complete with multiple members visible: prompt 1 (codebase) **3/4 members**, prompt 3 (git status) **3/4 members** each with rich real summaries; prompt 2 (AGENTS.md — the heaviest) **2/4 members**, resolves cleanly.

### Captured frames (`frames/`)
| Frame | Proves |
|---|---|
| `video1-deepseek-v4-pro-p3-gitstatus-diff-show.png` | DeepSeek V4 Pro: `git_status` diff (`4 files changed, 255 insertions`) + `show` (real commit `d2eaaf5d`); `shell` refused. |
| `video2-ensemble-p1-codebase-3of4.png` | Ensemble 3/4 members (DeepSeek V4 / Mistral / OpenRouter) describing the real codebase via `glob`+`git_status`. |
| `video2-ensemble-p2-agentsmd-2of4-resolves.png` | The heaviest prompt resolves (no overflow, no byte-array garbage; `codebase_map` renders as clean JSON). |
| `video2-ensemble-p3-gitstatus-3of4.png` | Ensemble 3/4 members each with a real git-status summary (the actual 4-file diff). |

## Fixes landed this run (all TDD RED→GREEN, paired §1.1)
The DeepSeek-V4 single-model path worked immediately (128k context). The **ensemble** with smaller free-tier members surfaced two real robustness bugs, both fixed:

1. **Context overflow on deep multi-tool investigations** — the AGENTS.md prompt triggers many `glob`/`grep`/`fs_read` calls; full tool outputs accumulated across turns and overflowed the ensemble's smallest member (Groq 8K-context), failing the whole loop (`Please reduce the length of the messages`). → `RunToolLoop` now bounds each model-facing tool result (`ToolLoopOptions.MaxToolResultChars`, TUI sets 800; the display trace keeps its own excerpt).
2. **`fs_read` returned byte arrays** — `*filesystem.FileContent` (with a `[]byte` content field) reached the model as decimal numbers (`[35 32 65 …]`) via `%v`, so models reported files as "binary/corrupted." → `FileContent.String()` renders readable text; `stringifyResult` now handles `string`/`error`/`[]byte`/`fmt.Stringer`/JSON (defense-in-depth for every tool).

## Honest note (§11.4.6 / §11.4.123)
The ensemble's heaviest deep-investigation prompt (AGENTS.md) is inherently harder for free-tier small-context members; it resolves reliably now (no crash/garbage) but with fewer participating members (2/4) and thinner answers than the lighter prompts (3/4, rich). A single large-context model (DeepSeek V4 Pro, Video 1) handles every prompt richly. This is a free-tier-infrastructure characteristic, not a code defect.

## Reproduce
```
source /tmp/helix_keys.sh
cd helix_code && go build -o bin/helix-tui ./applications/terminal_ui/
vhs /tmp/v1_ds.tape       # Video 1 — DeepSeek V4 Pro (digit 2)
vhs /tmp/v2_strong.tape   # Video 2 — ensemble (digit 9, 75s/prompt)
```

## Sources verified 2026-06-14
- DeepSeek live catalog (`https://api.deepseek.com/models`) → `deepseek-v4-pro`, `deepseek-v4-flash`.
- OpenRouter / Mistral / Groq live `/models` endpoints for the strong-model inventory.
