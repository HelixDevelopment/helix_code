// Package profiles defines named work profiles for HelixCode subagents.
//
// A Profile encodes the review-time defaults that govern HOW a subagent
// behaves: the LLM system prompt it receives, the LLM model preferences
// (currently temperature only), and a tool filter that gates which tools
// the dispatcher may register for the subagent's session.
//
// This is the Go port of upstream gptme's profile mechanism (PR #2381
// "feat(profiles): add built-in verifier profile for subagent review/
// validation"). The minimal shippable slice landed here is the verifier
// profile, paired with the subagent.RoleVerify posture (PR #2382).
//
// Anti-bluff guarantees (CONST-035 / Article XI §11.9):
//   - The verifier profile is a real Profile struct with a non-empty
//     system prompt that names review/verify intent; it is consumed by
//     the spawner's LLMRequest construction so the LLM actually sees it.
//   - FilterTools is a real, deterministic filter over ToolDescriptor;
//     it rejects write-side tools by both name AND by the IsReadOnly
//     contract on the descriptor (defense in depth — a tool that lies
//     about being read-only is still caught by name).
//   - ForRole is the deterministic Role -> Profile resolver consumed by
//     the manager when dispatching role-tagged tasks. No hardcoded
//     English content is leaked to end users: the system prompt is the
//     LLM-facing instruction (not a user-visible label), and the profile
//     name is an internal identifier.
//
// Spec source-of-truth: cli_agents/gptme commit 84d3a1aa9 (verifier
// profile) and d13e2eebe (subagent role= parameter).
package profiles

import (
	"strings"

	"dev.helix.code/internal/agent/subagent"
)

// Profile name constants. These are internal identifiers (not user-facing
// strings) used by ForRole and Get lookups.
const (
	// NameVerifier is the built-in profile that pairs with RoleVerify.
	// Configures a read-only, low-temperature, review-focused LLM session.
	NameVerifier = "verifier"
	// NameExplorer is the built-in profile that pairs with RoleExplore.
	// Read-only investigation; broader exploratory prompt.
	NameExplorer = "explorer"
	// NameDeveloper is the built-in profile that pairs with RoleImplement.
	// Full capability; allows writes and shell.
	NameDeveloper = "developer"
)

// ToolDescriptor is the minimal contract the profile filter needs to
// classify a tool. Production tool implementations satisfy it by adding
// the two methods to their existing Tool interface; the profile package
// purposefully does NOT depend on the full `internal/tools` package
// (which would create an import cycle through approval/mcp/etc.).
//
// ToolName: stable lower-case identifier (e.g. "fs_read", "fs_write",
// "shell"). FilterTools uses this as the primary classifier.
//
// IsReadOnly: the tool's own self-classification. Used as a backstop
// when a name-based rule is not configured.
type ToolDescriptor interface {
	ToolName() string
	IsReadOnly() bool
}

// Profile encodes the review-time defaults applied to a subagent task.
// A Profile is consumed at dispatch time:
//   - SystemPrompt is prepended to the LLMRequest the spawner builds.
//   - Temperature is passed to the LLMRequest (deterministic 0.1 for
//     the verifier — review output should be reproducible).
//   - FilterTools is applied to the candidate tool registration list.
//
// A nil receiver is treated as the empty profile.
type Profile struct {
	// Name is the internal identifier (one of NameVerifier / NameExplorer
	// / NameDeveloper). NOT user-facing.
	Name string

	// SystemPrompt is the LLM-facing instruction that establishes the
	// subagent's posture. For the verifier this names review/verify
	// duties and explicitly forbids modifications.
	SystemPrompt string

	// Temperature is the preferred sampling temperature. For
	// deterministic-leaning profiles (verifier) this is 0.1; for
	// exploratory or generative profiles it may be left at 0
	// (use provider default).
	Temperature float64

	// AllowedToolNames is the explicit allowlist of tool names this
	// profile accepts. When non-empty, FilterTools KEEPS only tools whose
	// ToolName is in the set. When empty, FilterTools falls back to the
	// IsReadOnly self-classification of the tool.
	AllowedToolNames []string

	// DeniedToolNames is the explicit denylist. Applied AFTER the
	// allowlist; a tool in the denylist is dropped even if it would have
	// passed the read-only or allowlist check. Acts as a safety net for
	// tools that lie about IsReadOnly().
	DeniedToolNames []string

	// ReadOnlyOnly: when true, FilterTools also drops any tool that
	// reports IsReadOnly()==false, regardless of the allow/deny lists.
	// This is the bedrock guarantee for the verifier profile.
	ReadOnlyOnly bool
}

// FilterTools applies the profile's tool-filter contract to the given
// list and returns the subset the dispatcher may register for the
// subagent.
//
// Resolution order:
//  1. DeniedToolNames: any name present is dropped immediately.
//  2. ReadOnlyOnly: when true, drop any tool with IsReadOnly()==false.
//  3. AllowedToolNames: when non-empty, keep ONLY tools whose name is
//     in the list. When empty, the previous step's verdict stands.
//
// A nil receiver returns the input slice unchanged (no opinion).
func (p *Profile) FilterTools(tools []ToolDescriptor) []ToolDescriptor {
	if p == nil {
		return tools
	}
	denied := toSet(p.DeniedToolNames)
	allowed := toSet(p.AllowedToolNames)
	useAllowlist := len(allowed) > 0

	out := make([]ToolDescriptor, 0, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		name := t.ToolName()
		if denied[name] {
			continue
		}
		if p.ReadOnlyOnly && !t.IsReadOnly() {
			continue
		}
		if useAllowlist && !allowed[name] {
			continue
		}
		out = append(out, t)
	}
	return out
}

func toSet(xs []string) map[string]bool {
	if len(xs) == 0 {
		return nil
	}
	out := make(map[string]bool, len(xs))
	for _, x := range xs {
		out[strings.ToLower(x)] = true
	}
	return out
}

// builtins is the registry of in-tree profile definitions. Indexed by
// canonical Name. Currently ships only the verifier profile (the
// minimal slice from upstream PR #2381); explorer/developer entries are
// declared for ForRole completeness with empty bodies until their
// respective slices land.
var builtins = map[string]*Profile{
	NameVerifier: {
		Name: NameVerifier,
		// SystemPrompt: names review/verify intent explicitly so the
		// LLM understands its posture. The string is LLM-facing (an
		// internal instruction, NOT user-visible text), so it is not
		// subject to CONST-046 (which governs user-facing content).
		SystemPrompt: "You are in VERIFIER mode. Your task is to review " +
			"and verify the work of other agents. You MUST:\n" +
			"- Review code, test output, and design for correctness and edge cases.\n" +
			"- Cite findings with file:line[:column] references.\n" +
			"- Identify regressions, security issues, and missing test coverage.\n" +
			"- Produce a structured verification report.\n" +
			"You MUST NOT:\n" +
			"- Modify any files, commit, push, or otherwise mutate state.\n" +
			"- Execute shell commands that have side effects.\n" +
			"- Fabricate findings: every issue you raise MUST be grounded in the\n" +
			"  artefact you were asked to review.\n",
		Temperature: 0.1,
		AllowedToolNames: []string{
			"fs_read",
			"glob",
			"grep",
			"codebase_map",
			"file_definitions",
			"lsp_definitions",
			"lsp_references",
			"lsp_hover",
			"web_fetch",
			"web_search",
			"notebook_read",
		},
		DeniedToolNames: []string{
			"fs_write",
			"fs_edit",
			"multiedit",
			"multiedit_apply",
			"shell",
			"shell_background",
			"shell_sandboxed",
			"smart_edit",
			"notebook_edit",
			"task",
		},
		ReadOnlyOnly: true,
	},
	NameExplorer: {
		Name:         NameExplorer,
		ReadOnlyOnly: true,
	},
	NameDeveloper: {
		Name:         NameDeveloper,
		ReadOnlyOnly: false,
	},
}

// Get returns the built-in profile registered under name and a found flag.
// Unknown names return a nil-valued Profile and false.
func Get(name string) (*Profile, bool) {
	p, ok := builtins[name]
	if !ok {
		return nil, false
	}
	// Return a shallow copy so callers can locally tweak fields without
	// mutating the package-level registry.
	cp := *p
	return &cp, true
}

// ForRole maps a subagent.Role to a built-in Profile.
//
//   - RoleVerify     -> NameVerifier (the verifier profile)
//   - RoleExplore    -> NameExplorer
//   - RoleImplement  -> NameDeveloper
//   - RoleGeneral / "" / unknown -> (nil, false) (no opinion)
//
// The mapping is deterministic; callers can rely on RoleVerify always
// resolving to the verifier profile across versions.
func ForRole(r subagent.Role) (*Profile, bool) {
	switch r {
	case subagent.RoleVerify:
		return Get(NameVerifier)
	case subagent.RoleExplore:
		return Get(NameExplorer)
	case subagent.RoleImplement:
		return Get(NameDeveloper)
	default:
		return nil, false
	}
}
