# SDK & CLI-Agent Currency Research — 2026-06-10

**Scope:** §11.4.99 latest-source deep-web research for HelixCode. Two parts:
PART A = Go provider/infra SDK currency (SP2 P2.3 SDK-currency); PART B =
non-interactive/scriptable invocation for bridged CLI agents (SP4 P4.3).

**Method:** Every version/flag below was fetched from the OFFICIAL source
(GitHub releases, pkg.go.dev `?tab=versions`, vendor docs) on 2026-06-10 — never
training memory. Honest negative findings are recorded where a page was
unreachable. Full URL list in the `## Sources verified 2026-06-10` footer.

**HelixCode current pins (read 2026-06-10):**
- `helix_code/go.mod` — aws-sdk-go-v2 v1.32.7, config v1.28.7, credentials v1.17.48,
  service/bedrockruntime v1.23.1, azcore v1.16.0, azidentity v1.8.0,
  gin v1.11.0, smacker/go-tree-sitter `v0.0.0-20240827094217-dd81d9e9be82`,
  getzep/zep-go/v3 v3.10.0.
- `submodules/helix_agent/go.mod` — gin v1.12.0, smacker/go-tree-sitter
  `v0.0.0-20240827094217-dd81d9e9be82`.
- (`helix_agent/go.mod` at repo root does not exist; the submodule lives at
  `submodules/helix_agent/`.)
- No vendor SDK for OpenAI/Anthropic/Gemini is pinned — HelixCode hand-rolls
  HTTP to these providers (no `openai/openai-go`, `anthropics/anthropic-sdk-go`,
  or `google.golang.org/genai` in either go.mod).

---

## PART A — Go provider/infra SDK latest versions

| Dependency | HelixCode pin | LATEST (2026-06-10) | Released | Skew | Breaking major? |
|---|---|---|---|---|---|
| `github.com/aws/aws-sdk-go-v2` (core) | v1.32.7 | **v1.42.0** | 2026-06-08 | 10 minor behind | No — same v1 line; v1.42.0 adds a preview standard-retry change behind `AWS_NEW_RETRIES_2026` flag (opt-in, non-breaking) |
| `.../service/bedrockruntime` | v1.23.1 | **v1.53.5** | 2026-06-08 | ~30 minor behind | No — same v1 line. **Most stale provider SDK.** |
| `github.com/Azure/azure-sdk-for-go/sdk/azcore` | v1.16.0 | **v1.22.0** | 2026-06-04 | 6 minor behind | No — same v1 line |
| `github.com/Azure/azure-sdk-for-go/sdk/azidentity` | v1.8.0 | **v1.13.1** (stable) | 2025-11-10 | 5 minor behind | No — same v1 line. v1.14.0 only in beta (v1.14.0-beta.3, 2026-02-09). **Security note: GO-2024-2918 (Azure Identity EoP) affects old azidentity — upgrading to the latest patched release is advisable.** |
| `github.com/gin-gonic/gin` | v1.11.0 (helix_code) / v1.12.0 (helix_agent) | **v1.12.0** | 2026-02-28 | helix_code 1 minor behind; helix_agent already current | No — same v1 line. **Skew resolution: bump helix_code v1.11.0 → v1.12.0 to match helix_agent; v1.12.0 adds Protocol Buffers support, color latency logging, bson rendering — additive.** |
| `github.com/smacker/go-tree-sitter` | `v0.0.0-20240827094217-dd81d9e9be82` (2024-08-27 commit) | **same commit is still HEAD** (latest master commit dd81d9e, 2024-08-27) | 2024-08-27 | At HEAD, but repo is effectively dormant (no commits since Aug-2024) | n/a — see migration note below |
| `github.com/getzep/zep-go/v3` | v3.10.0 | **v3.23.0** | 2026-06-04 | 13 minor behind | No — same v3 line |
| `github.com/openai/openai-go` | not used (hand-rolled HTTP) | **v1.12.0** | 2025-07-30 | n/a | Would be a NEW dependency; still v1.x, no v2/v3 |
| `github.com/anthropics/anthropic-sdk-go` | not used (hand-rolled HTTP) | **v1.50.1** | 2026-06-09 | n/a | Would be a NEW dependency; v1.50.x adds managed-agent deployment APIs |
| `google.golang.org/genai` (Gemini) | not used (hand-rolled HTTP) | **v1.59.0** | 2026-06-03 | n/a | Would be a NEW dependency; v1.59.0 is the modern unified Google GenAI Go SDK (supersedes the older `generative-ai-go`) |

### go-tree-sitter migration note (negative/structural finding)
`smacker/go-tree-sitter` is pinned at its HEAD commit but the repo has had **no
commits since 2024-08-27** (verified on the commits/master page). The OFFICIAL
binding `github.com/tree-sitter/go-tree-sitter` (in the upstream `tree-sitter`
org) is actively released — latest tag **v0.25.0 (2025-02-02)**. The official
README does NOT contain explicit "supersedes smacker/..." language (honest
negative finding — I did not find a vendor statement of supersession), so the
migration is a judgment call, not a vendor-mandated one. If HelixCode wants a
maintained binding it would move smacker → `tree-sitter/go-tree-sitter` v0.25.0;
note that is a different import path + API (Close-on-allocation requirement), so
it is real migration work, not a version bump.

---

## PART B — Non-interactive / scriptable invocation for bridge CLI agents

Confirmed CURRENT (2026-06-10) command forms. "JSON output" column = does the
agent emit machine-readable JSON in headless mode today.

| CLI agent | Single non-interactive prompt | JSON / machine-readable output | List models | Auth / env |
|---|---|---|---|---|
| **Claude Code** (`claude`) | `claude -p "<prompt>"` (`--print`/`-p`) | **Yes** — `--output-format json` (also `text`, `stream-json`); `--input-format text\|stream-json`; `--json-schema '<schema>'` for validated structured output | No dedicated `models` list command; set with `--model sonnet\|opus\|haiku\|fable\|<full-name>`; `--fallback-model` chain | `ANTHROPIC_API_KEY` or Claude subscription; `claude setup-token` mints a long-lived OAuth token for CI |
| **Qwen Code** (`qwen`) | `qwen -p "<prompt>"` (`-p`/`--prompt`, "headless mode") | **Yes** — `--output-format text\|json\|stream-json`; `--input-format text\|stream-json`; `--include-partial-messages` | **No** CLI flag to list models (config-file / interactive `/model` only) | API-key / OAuth via `qwen auth` subcommands (`qwen auth api-key`, `qwen auth status`); `-y`/`--yolo` auto-approve |
| **OpenCode** (`opencode`) incl. **Zen** | `opencode run "<prompt>"` (args or stdin); `--attach http://host:port` to reuse a server | **Yes** — `opencode run --format json` (raw JSON events; default = formatted) | **Yes** — `opencode models [provider]` (`--refresh`, `--verbose`); for Zen also `GET https://opencode.ai/zen/v1/models` | Zen: sign in to OpenCode Zen, copy API key, `/connect` → OpenCode Zen → paste key. **Zen model id format = `opencode/<model-id>`** (e.g. `opencode/gpt-5.5`); Zen is a curated 40+ model gateway (GPT/Claude/Gemini/Qwen/DeepSeek/…) |
| **Gemini CLI** (`gemini`) | `gemini -p "<prompt>"` (`-p`/`--prompt`; headless also auto-triggers in non-TTY) | **Yes** — `--output-format text\|json`; JSON is a single object with `response` (string), `stats` (token/latency), `error` (optional) | No documented list-models flag (honest negative finding); set with `-m`/`--model` | Gemini API key / Google auth env; standard Gemini CLI config |
| **Crush** (`crush`) | `crush run "<prompt>"` (args or piped stdin; single-turn → stdout) | **No** dedicated JSON output format found (honest negative finding — README + run docs show no `--format json`); `--quiet` suppresses spinner for clean stdout | Models via config file (`.crush.json` / `crush.json` / `~/.config/crush/crush.json`); `crush update-providers` refreshes provider list; no `models` list flag found | Provider keys via config/env; `--yolo` auto-approves tools (non-interactive); global flags `--debug`, `--cwd`, `--data-dir` |
| **Codex CLI** (`codex`) | `codex exec "<prompt>"` (alias `codex e`) | **Yes** — `--json` / `--experimental-json` (newline-delimited JSON events); `-o`/`--output-last-message <path>` writes final message to file | No documented list-models flag; set with `-m`/`--model` | OpenAI auth; `--sandbox read-only\|workspace-write\|danger-full-access`; `--ephemeral` for no session files |
| **Goose** (`goose`) | `goose run -t "<prompt>"` (`-t`/`--text`; `-i`/`--instructions <file\|->`; `--no-session`) | **Yes** — `--output-format text\|json\|stream-json` | No CLI list-models flag (honest negative finding); set with `--provider` + `--model` | Provider/model via env or `--provider`/`--model` flags |
| **GitHub Copilot CLI** (`copilot`) | `copilot -p "<prompt>"` (`-p`/`--prompt`, non-interactive) | **No** JSON format flag found (honest negative finding); `-s`/`--silent` strips stats/decoration → clean text for piping | **No** list flag; model strings are shown in the `--model` option description under `copilot help`; set with `--model gpt-5.2\|claude-sonnet-4.6\|…` | GitHub auth; `--no-ask-user` prevents clarifying-question pauses; `--allow-all-tools` for unattended runs |

---

## 12-line summary

1. **Most-stale SDK (priority #1): `service/bedrockruntime` v1.23.1 → v1.53.5** — ~30 minor versions behind; same v1 line, non-breaking.
2. **Priority #2: `getzep/zep-go/v3` v3.10.0 → v3.23.0** — 13 minor behind, same v3 line.
3. **Priority #3: `aws-sdk-go-v2` core v1.32.7 → v1.42.0** (+ config/credentials siblings) — 10 minor behind; new retry behavior is opt-in behind `AWS_NEW_RETRIES_2026`.
4. **Priority #4: `azidentity` v1.8.0 → v1.13.1** — 5 minor behind AND carries security relevance (GO-2024-2918 EoP on old releases); stay on stable, v1.14.0 is beta-only.
5. **Priority #5: `azcore` v1.16.0 → v1.22.0** — 6 minor behind, non-breaking.
6. **gin skew fix: bump helix_code v1.11.0 → v1.12.0** to match helix_agent (already on v1.12.0); additive features only.
7. **All Part-A bumps are same-major (no breaking major bump required)** for currently-pinned deps.
8. **go-tree-sitter:** pinned at smacker HEAD but that repo is dormant since Aug-2024; maintained path is `tree-sitter/go-tree-sitter` v0.25.0 — a real migration (different import path/API), not a version bump; no vendor supersession statement found.
9. **Vendor SDKs HelixCode does NOT use (hand-rolled HTTP):** openai-go v1.12.0, anthropic-sdk-go v1.50.1, google.golang.org/genai v1.59.0 — adopting any is net-new dependency work, all on v1.x.
10. **CLI agents with CONFIRMED non-interactive JSON output today:** Claude Code (`--output-format json`), Qwen Code (`--output-format json`), OpenCode (`run --format json`), Gemini CLI (`--output-format json`), Codex CLI (`exec --json`), Goose (`--output-format json`).
11. **CLI agents WITHOUT a JSON output format (text-only, use `-s`/`--quiet` for clean stdout):** GitHub Copilot CLI (`-p` + `-s`) and Crush (`run` + `--quiet`) — honest negative finding from their official docs.
12. **OpenCode/Zen specifics for the bridge:** Zen is the only agent with a first-class machine model list (`opencode models` + `GET /zen/v1/models`); Zen model ids use the `opencode/<model-id>` format and require an OpenCode Zen API key via `/connect`.

---

## Sources verified 2026-06-10

PART A (SDK versions):
- https://github.com/aws/aws-sdk-go-v2/releases
- https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/bedrockruntime?tab=versions
- https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azcore?tab=versions
- https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity?tab=versions
- https://github.com/gin-gonic/gin/releases
- https://github.com/smacker/go-tree-sitter/commits/master
- https://github.com/tree-sitter/go-tree-sitter
- https://pkg.go.dev/github.com/tree-sitter/go-tree-sitter?tab=versions
- https://pkg.go.dev/google.golang.org/genai?tab=versions
- https://github.com/getzep/zep-go/releases
- https://pkg.go.dev/github.com/getzep/zep-go/v3?tab=versions
- https://pkg.go.dev/github.com/openai/openai-go?tab=versions
- https://pkg.go.dev/github.com/anthropics/anthropic-sdk-go?tab=versions

PART B (CLI agents):
- https://code.claude.com/docs/en/cli-reference (redirect from https://docs.claude.com/en/docs/claude-code/cli-reference)
- https://qwenlm.github.io/qwen-code-docs/en/users/features/headless/
- https://opencode.ai/docs/cli/
- https://opencode.ai/docs/zen/
- https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/headless.md
- https://geminicli.com/docs/cli/headless/ (referenced)
- https://github.com/charmbracelet/crush
- https://deepwiki.com/charmbracelet/crush/2.2-cli-usage (referenced for `crush run` flags; mintlify.com/charmbracelet/crush/cli/run returned HTTP 410 Gone)
- https://developers.openai.com/codex/cli/reference
- https://goose-docs.ai/docs/guides/goose-cli-commands/
- https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-programmatic-reference
- https://docs.github.com/en/copilot/how-tos/copilot-cli/automate-copilot-cli/run-cli-programmatically

Unreachable / negative-finding pages (recorded honestly):
- https://docs.claude.com/en/docs/claude-code/cli-reference — 301 → resolved at code.claude.com.
- https://qwenlm.github.io/qwen-code-docs/en/cli/ , .../cli/configuration/ , .../cli/headless/ — 404; correct path is `/en/users/features/headless/`.
- https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/configuration.md and google-gemini.github.io .../configuration.html — 404; resolved via `docs/cli/headless.md`.
- https://www.mintlify.com/charmbracelet/crush/cli/run — HTTP 410 Gone; Crush JSON-output absence confirmed from README + DeepWiki CLI-usage page (negative finding).
- https://block.github.io/goose/docs/guides/goose-cli-commands/ — 404; resolved via goose-docs.ai.
