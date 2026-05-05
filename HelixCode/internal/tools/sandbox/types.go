// Package sandbox defines the foundational types, default policies, and
// CONST-033 host-power-management deny-list for HelixCode's sandboxed shell
// execution feature (P1-F14).
//
// This file is type-only: every backend (bubblewrap / native), the manager,
// the agent tool, the slash command, and the YAML loader all consume the
// types declared here. Behaviour (capability detection, command execution,
// backend dispatch) lives in sibling files added by later T03–T10 tasks.
//
// Constitutional anchor: CONST-033 (Host Power Management Hard Ban).
// `ConstitutionalDenyList` enumerates the canonical patterns; matching is
// performed by `MatchConstitutionalDenyList` and is non-overridable. User
// configuration may only ADD to the deny-list, never subtract.
package sandbox

import (
	"context"
	"regexp"
	"time"
)

// BackendKind identifies which sandbox implementation is in use.
//
// Selection priority (per spec §3.3 and §5.2): bubblewrap > native > none.
// `BackendNone` is the fail-closed sentinel — it never executes commands,
// it only signals to callers that no usable backend was selected and that
// `SandboxCapabilities.UnavailableReason` carries a human-readable cause.
type BackendKind string

const (
	// BackendBubblewrap selects the `bwrap(1)` external sandbox binary.
	BackendBubblewrap BackendKind = "bubblewrap"
	// BackendNative selects the in-process Linux user-namespace fallback.
	BackendNative BackendKind = "native"
	// BackendNone is the fail-closed sentinel (no usable backend on host).
	BackendNone BackendKind = "none"
)

// String returns the lowercase backend name (matches the constant value).
func (b BackendKind) String() string {
	return string(b)
}

// SandboxCapabilities is the result of host detection: which sandbox
// primitives the kernel + filesystem expose, and which backend was selected.
//
// `SelectedBackend == BackendNone` MUST be paired with a non-empty
// `UnavailableReason`; downstream code uses that text verbatim in
// `ErrSandboxUnavailable` messages so the user can act on it.
type SandboxCapabilities struct {
	GOOS               string      `json:"goos"`
	BubblewrapPath     string      `json:"bubblewrap_path,omitempty"`     // exec.LookPath result; empty when absent
	UnprivilegedUserNS bool        `json:"unprivileged_userns"`           // /proc/sys/kernel/unprivileged_userns_clone == 1
	CGroupsV2          bool        `json:"cgroups_v2"`                    // /sys/fs/cgroup/cgroup.controllers exists
	SelectedBackend    BackendKind `json:"selected_backend"`              // bubblewrap | native | none
	UnavailableReason  string      `json:"unavailable_reason,omitempty"`  // populated when SelectedBackend == None
}

// BindMount is an additional path the sandbox should bind into the container.
// `Source` is a host path; `Target` is the mount point inside the sandbox.
type BindMount struct {
	Source   string `yaml:"source"    json:"source"`
	Target   string `yaml:"target"    json:"target"`
	ReadOnly bool   `yaml:"read_only" json:"read_only"`
}

// SandboxPolicy describes the runtime policy for a single command execution.
//
// Zero-value semantics are deliberately the safe defaults: NetworkAllowed=false
// (DENY), no resource limits, no bind mounts, no extra denies. Use
// `DefaultSandboxPolicy()` to also materialise the default 30s timeout and
// read-only root flag.
type SandboxPolicy struct {
	NetworkAllowed bool          `yaml:"network_allowed" json:"network_allowed"`
	Timeout        time.Duration `yaml:"timeout"          json:"timeout"`
	MemoryLimitMB  int           `yaml:"memory_limit_mb"  json:"memory_limit_mb"` // 0 = no limit
	CPULimitPct    int           `yaml:"cpu_limit_pct"    json:"cpu_limit_pct"`   // 0 = no limit
	ReadOnlyRoot   bool          `yaml:"read_only_root"   json:"read_only_root"`  // bind / as ro
	BindMounts     []BindMount   `yaml:"bind_mounts"      json:"bind_mounts"`     // additional rw bind mounts
	ExtraDeny      []string      `yaml:"extra_deny"       json:"extra_deny"`      // additive only; cannot subtract from CONST-033
}

// DefaultSandboxPolicy returns the safe-default policy:
//   - Network DENY (Q3=A — explicit opt-in only)
//   - Timeout 30s
//   - No memory or CPU limits
//   - Read-only root
//   - No bind mounts, no extra denies
func DefaultSandboxPolicy() SandboxPolicy {
	return SandboxPolicy{
		NetworkAllowed: false,
		Timeout:        30 * time.Second,
		MemoryLimitMB:  0,
		CPULimitPct:    0,
		ReadOnlyRoot:   true,
		BindMounts:     nil,
		ExtraDeny:      nil,
	}
}

// SandboxConfig is the persisted on-disk `sandbox.yaml` shape.
//
// `UserDenyList` is additive: T06 (manager) merges it with
// `ConstitutionalDenyList` before dispatch. Users cannot remove CONST-033
// entries; attempting to do so via config has no effect.
type SandboxConfig struct {
	DefaultPolicy SandboxPolicy `yaml:"default_policy"`
	UserDenyList  []string      `yaml:"user_deny_list"` // additional commands to reject
}

// DefaultSandboxConfig returns the zero-config that ships with HelixCode:
// `DefaultSandboxPolicy()` and an empty user deny-list.
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		DefaultPolicy: DefaultSandboxPolicy(),
		UserDenyList:  nil,
	}
}

// SandboxResult is what a `SandboxBackend.Run` call returns. `TimedOut` is
// true when the sandbox killed the command because `Policy.Timeout` elapsed;
// callers should treat that as a non-zero exit even if the kernel reports 0.
type SandboxResult struct {
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	ExitCode int           `json:"exit_code"`
	TimedOut bool          `json:"timed_out"`
	Backend  BackendKind   `json:"backend"`
	Duration time.Duration `json:"duration"`
}

// SandboxBackend is the implementation contract: bubblewrap or native.
//
// `Run` MUST honour `policy` exactly — backends do not relax limits or
// override the deny-network default. Backends MAY assume the manager has
// already applied the CONST-033 + user deny-list checks before dispatch.
type SandboxBackend interface {
	Kind() BackendKind
	Run(ctx context.Context, command string, policy SandboxPolicy) (*SandboxResult, error)
}

// ConstitutionalDenyEntry pairs a compiled regex with a human-readable
// description used in error messages and `/sandbox policy` output.
type ConstitutionalDenyEntry struct {
	Pattern     *regexp.Regexp
	Description string
}

// ConstitutionalDenyList is the immutable set of patterns matching commands
// forbidden by CONST-033 (host power management). Populated at package init
// from `constitutionalDenySpecs`. Use `MatchConstitutionalDenyList` to test.
//
// This list is checked BEFORE any backend is invoked (see spec §4.6). User
// configuration adds to it via `SandboxConfig.UserDenyList`; it cannot
// subtract.
var ConstitutionalDenyList []ConstitutionalDenyEntry

// constitutionalDenySpecs is the source-of-truth pattern table compiled at
// init. Each entry produces one `ConstitutionalDenyEntry`.
//
// The patterns combine two matching strategies (per spec §6 / §3.3):
//
//  1. "Token-style" entries — anchored with `^\s*` so they match against the
//     argv head (e.g. `^\s*systemctl\s+(suspend|...)\b`). These catch direct
//     invocations: `systemctl suspend`, `systemctl   poweroff` (extra spaces).
//
//  2. "Substring" entries — wrapped in `(^|[\s;&|])` ... `\b` to match anywhere
//     inside the raw command string with word boundaries. These catch chained
//     and nested forms: `ls; systemctl suspend`, `bash -c 'systemctl suspend'`,
//     `echo mem > /sys/power/state`.
//
// `MatchConstitutionalDenyList` runs every regex against BOTH the raw
// command string AND the joined argv; either match returns true.
var constitutionalDenySpecs = []struct {
	pattern     string
	description string
}{
	// systemctl power-management subcommands. Allow common shell-quoting
	// chars (single/double-quote, backtick) as a preceding boundary so
	// nested forms like `bash -c 'systemctl suspend'` still match.
	{
		pattern:     "(?:^|[\\s;&|(`'\"])systemctl\\s+(?:suspend|hibernate|hybrid-sleep|suspend-then-hibernate|poweroff|halt|reboot|kexec)\\b",
		description: "CONST-033: systemctl power-management subcommand (suspend/hibernate/poweroff/halt/reboot/kexec)",
	},
	// loginctl power-management + session-killing subcommands
	{
		pattern:     "(?:^|[\\s;&|(`'\"])loginctl\\s+(?:suspend|hibernate|hybrid-sleep|suspend-then-hibernate|poweroff|reboot|terminate-user|terminate-session|kill-user|kill-session)\\b",
		description: "CONST-033: loginctl power-management or session-termination subcommand",
	},
	// pm-utils suspend/hibernate binaries (word-boundary, with optional path prefix `/`)
	{
		pattern:     "(?:^|[\\s;&|(`'\"/])pm-(?:suspend|hibernate|suspend-hybrid)\\b",
		description: "CONST-033: pm-utils suspend/hibernate binary (pm-suspend/pm-hibernate/pm-suspend-hybrid)",
	},
	// Bare power-state binaries as a command head: shutdown, halt, poweroff, reboot, kexec.
	// Anchored to start-of-string OR after a shell command separator so we do not
	// fire on "echo halts and catches fire" or paths like "./reboot-helper.sh".
	{
		pattern:     `(?:^|[;&|(]\s*|\s(?:sudo|doas|pkexec|env|nohup|exec|eval)\s+)(?:shutdown|halt|poweroff|reboot|kexec)(?:\s|$)`,
		description: "CONST-033: bare power-state binary as command head (shutdown/halt/poweroff/reboot/kexec)",
	},
	// pkill / killall against the entire user session
	{
		pattern:     `(?:^|[\s;&|(])(?:pkill|killall)\s+(?:-[A-Za-z0-9]+\s+)*-u\s+\S+`,
		description: "CONST-033: pkill/killall -u against an entire user (session-termination vector)",
	},
	// gnome-session-quit (logout / shutdown the desktop session)
	{
		pattern:     `(?:^|[\s;&|(])gnome-session-quit\b`,
		description: "CONST-033: gnome-session-quit (desktop session logout/shutdown)",
	},
	// dbus-send to org.freedesktop.login1.Manager power methods
	{
		pattern:     `(?i)dbus-send\b[^\n]*org\.freedesktop\.login1\.Manager\.(?:Suspend|Hibernate|HybridSleep|SuspendThenHibernate|PowerOff|Reboot)\b`,
		description: "CONST-033: dbus-send invoking org.freedesktop.login1.Manager power method",
	},
	// Direct write to /sys/power/state (echo mem|disk|freeze > /sys/power/state)
	{
		pattern:     `>\s*/sys/power/state\b`,
		description: "CONST-033: redirect into /sys/power/state (kernel power-state write)",
	},
}

func init() {
	ConstitutionalDenyList = make([]ConstitutionalDenyEntry, 0, len(constitutionalDenySpecs))
	for _, s := range constitutionalDenySpecs {
		// MustCompile is appropriate here: the patterns are compile-time
		// constants and a malformed regex is a programmer error that should
		// crash the binary at startup, not at first use.
		ConstitutionalDenyList = append(ConstitutionalDenyList, ConstitutionalDenyEntry{
			Pattern:     regexp.MustCompile(s.pattern),
			Description: s.description,
		})
	}
}

// MatchConstitutionalDenyList returns the first matching deny-list entry
// for either `rawCmd` or the joined `argv`, or `(nil, false)` if no match.
//
// Each pattern is tested against:
//
//  1. The raw command string (`rawCmd`) — catches chained (`a; b`) and nested
//     (`bash -c '...'`) forms.
//  2. The space-joined argv (`strings.Join(argv, " ")`) — keeps behaviour
//     symmetrical when callers have already tokenised the command.
//
// Either match is a hit. Returning the first match is intentional: callers
// only need to know that a violation exists and what description to surface.
func MatchConstitutionalDenyList(rawCmd string, argv []string) (*ConstitutionalDenyEntry, bool) {
	joined := joinArgs(argv)
	for i := range ConstitutionalDenyList {
		entry := &ConstitutionalDenyList[i]
		if entry.Pattern.MatchString(rawCmd) {
			return entry, true
		}
		if joined != "" && entry.Pattern.MatchString(joined) {
			return entry, true
		}
	}
	return nil, false
}

// joinArgs is a tiny dependency-free space-joiner. We avoid `strings.Join`
// only to keep the import surface of this type-only file at parity with
// `lsp_types.go` (encoding/json + time + regexp + context).
func joinArgs(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	n := 0
	for _, a := range argv {
		n += len(a) + 1
	}
	buf := make([]byte, 0, n)
	for i, a := range argv {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = append(buf, a...)
	}
	return string(buf)
}
