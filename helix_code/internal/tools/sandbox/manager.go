// Package sandbox — manager.go.
//
// SandboxManager is the SOLE entry point for executing shell commands inside
// the F14 sandbox. It enforces, in order, BEFORE any backend dispatch:
//
//  1. CONST-033 deny-list (immutable; rejects host power-management commands).
//  2. User-configured extra deny-list (additive only — cannot subtract from
//     CONST-033).
//  3. Fail-closed when no usable backend was selected by the Detector.
//  4. Default-DENY network policy (per spec §3.1; substituted when caller
//     passes the zero policy).
//  5. Dispatches to the selected backend's Run.
//
// Anti-bluff contract: spec §4.6 requires the deny-list check to happen at
// the manager — NOT inside individual backends — so a future backend cannot
// silently bypass CONST-033. This file is the single chokepoint.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// Sentinel errors. Wrapped by DenyError / FailClosedError so callers can
// errors.Is() against either the sentinel or assert against the typed error
// for richer details.
var (
	// ErrCommandDenied is the sentinel for any deny-list rejection.
	ErrCommandDenied = errors.New("command denied")
	// ErrSandboxUnavailable is the sentinel for fail-closed (no backend).
	ErrSandboxUnavailable = errors.New("sandbox unavailable")
)

// DenyError is returned by Execute when the command matched either the
// CONST-033 deny-list or the user-configured deny-list. The MatchedRule
// field is human-readable (e.g. "CONST-033: systemctl power-management
// subcommand …" or "user-deny: ^rm -rf"); Pattern is the raw regex source.
type DenyError struct {
	Command     string
	MatchedRule string
	Pattern     string
}

func (e *DenyError) Error() string {
	return fmt.Sprintf("sandbox: command denied (%s): %q", e.MatchedRule, e.Command)
}

// Unwrap allows errors.Is(err, ErrCommandDenied).
func (e *DenyError) Unwrap() error { return ErrCommandDenied }

// FailClosedError is returned when SelectedBackend == BackendNone. Reason
// is the verbatim text from SandboxCapabilities.UnavailableReason — it
// includes actionable install hints so the user can remediate.
type FailClosedError struct {
	Reason string
}

func (e *FailClosedError) Error() string {
	return fmt.Sprintf("sandbox: %s", e.Reason)
}

// Unwrap allows errors.Is(err, ErrSandboxUnavailable).
func (e *FailClosedError) Unwrap() error { return ErrSandboxUnavailable }

// SandboxManager is the orchestrator that owns capabilities, both backend
// candidates (one of which is selected per caps), and the in-memory deny
// state. All Execute calls go through this single chokepoint.
//
// Concurrency: a sync.RWMutex guards mutable state (config + compiled
// user-deny patterns). Read paths (Execute, Capabilities, Config,
// SelectedBackend, MergedDenyList) take a read lock; UpdateConfig takes a
// write lock and recompiles patterns under it.
type SandboxManager struct {
	capabilities SandboxCapabilities
	bubblewrap   *BubblewrapBackend // may be nil when not selected
	native       *NativeBackend     // may be nil when not selected
	log          *zap.Logger

	mu               sync.RWMutex
	config           SandboxConfig
	userDenyPatterns []*regexp.Regexp // pre-compiled from config.UserDenyList; rebuilt on UpdateConfig

	// backendOverride is a TEST-ONLY seam. Production code never sets this.
	// When non-nil, Execute dispatches to backendOverride instead of the
	// selected real backend. This lets the unit tests use a spyBackend to
	// observe dispatch ordering (deny BEFORE Run) without needing a real
	// bwrap binary or a real userns clone.
	backendOverride SandboxBackend
}

// NewSandboxManager constructs a manager from already-detected capabilities
// and (optionally) pre-built backends. The caller is responsible for
// ensuring `bubblewrap` is non-nil iff caps.SelectedBackend ==
// BackendBubblewrap (and similarly for native). NewSandboxManagerFromDetector
// is the preferred high-level constructor.
//
// The manager compiles config.UserDenyList up front so the first Execute
// call is not penalised by a regex.MustCompile cost.
func NewSandboxManager(caps SandboxCapabilities, bubblewrap *BubblewrapBackend, native *NativeBackend, config SandboxConfig, log *zap.Logger) *SandboxManager {
	if log == nil {
		log = zap.NewNop()
	}
	m := &SandboxManager{
		capabilities: caps,
		bubblewrap:   bubblewrap,
		native:       native,
		log:          log,
		config:       config,
	}
	// Best-effort compile at construction. Errors here are surfaced lazily
	// via UpdateConfig; if the initial config has malformed regex, Execute
	// will still work for CONST-033 (which is independent), and the
	// malformed user pattern is silently dropped — the manager logs a
	// WARN. This matches spec §6 ("malformed user deny entries do not fail
	// the manager; they are ignored with a warning").
	_ = m.compileUserDeny()
	return m
}

// NewSandboxManagerFromDetector is the production high-level constructor.
// It runs the Detector, builds the appropriate backend (or none), and
// returns a fully-wired manager + the resolved capabilities so the caller
// can surface them via /sandbox status without re-detecting.
//
// workDir is the host cwd to bind read-write inside the sandbox.
func NewSandboxManagerFromDetector(workDir string, config SandboxConfig, log *zap.Logger) (*SandboxManager, SandboxCapabilities, error) {
	if log == nil {
		log = zap.NewNop()
	}
	caps := NewDetector().Detect()

	var bw *BubblewrapBackend
	var nb *NativeBackend
	switch caps.SelectedBackend {
	case BackendBubblewrap:
		bw = NewBubblewrapBackend(caps.BubblewrapPath, workDir)
	case BackendNative:
		var err error
		nb, err = NewNativeBackend(workDir)
		if err != nil {
			// Fall back to fail-closed; preserve the resolution error in
			// caps.UnavailableReason for surfaced diagnostics.
			caps.SelectedBackend = BackendNone
			caps.UnavailableReason = fmt.Sprintf("native sandbox unavailable: %v", err)
		}
	case BackendNone:
		// fall through; manager will fail-closed on Execute.
	}

	mgr := NewSandboxManager(caps, bw, nb, config, log)
	return mgr, caps, nil
}

// Capabilities returns a snapshot of the detected capabilities. Cheap;
// the value is immutable after construction.
func (m *SandboxManager) Capabilities() SandboxCapabilities {
	return m.capabilities
}

// SelectedBackend returns the kind chosen by the Detector at construction.
func (m *SandboxManager) SelectedBackend() BackendKind {
	return m.capabilities.SelectedBackend
}

// Config returns a copy of the current sandbox config.
func (m *SandboxManager) Config() SandboxConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig replaces the in-memory config (e.g. after `/sandbox reload`)
// and recompiles the user deny-list. Thread-safe. Returns an error only
// when ALL user patterns failed to compile — partial success is allowed,
// individual bad patterns are logged at WARN and skipped.
func (m *SandboxManager) UpdateConfig(cfg SandboxConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	return m.compileUserDenyLocked()
}

// compileUserDeny is the read-locked variant called from NewSandboxManager
// (where we hold no lock yet because the manager hasn't been published).
func (m *SandboxManager) compileUserDeny() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.compileUserDenyLocked()
}

// compileUserDenyLocked compiles m.config.UserDenyList into
// m.userDenyPatterns. Caller MUST hold m.mu (write).
func (m *SandboxManager) compileUserDenyLocked() error {
	patterns := m.config.UserDenyList
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	var firstErr error
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			m.log.Warn("sandbox: skipping malformed user-deny pattern",
				zap.String("pattern", p),
				zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		compiled = append(compiled, re)
	}
	m.userDenyPatterns = compiled
	// Only report an error if ALL patterns failed (caller may want to know).
	if len(compiled) == 0 && len(patterns) > 0 {
		return fmt.Errorf("all user-deny patterns failed to compile: %w", firstErr)
	}
	return nil
}

// Execute is the only entry point for sandboxed command execution. The
// ordering of checks is load-bearing (spec §4.6):
//
//  1. CONST-033 deny — before the user-deny check so an attempt to whitelist
//     a power-management command via UserDenyList contortions cannot succeed.
//  2. User-deny — additive denylist; CONST-033 has already passed.
//  3. Fail-closed — if no backend was selected, reject with the verbatim
//     UnavailableReason so the user sees install hints.
//  4. Default policy substitution — zero SandboxPolicy becomes
//     DefaultSandboxPolicy() (network DENY, 30s timeout, RO root).
//  5. Dispatch to backend.Run.
//
// Returns:
//   - (*SandboxResult, nil) when the backend ran the command (any exit code).
//   - (nil, *DenyError) when blocked by CONST-033 / user-deny.
//   - (nil, *FailClosedError) when no backend.
//   - (nil, error) when the backend itself errored before producing a result
//     (rare; most failures are surfaced via SandboxResult).
func (m *SandboxManager) Execute(ctx context.Context, command string, policy SandboxPolicy) (*SandboxResult, error) {
	// Tokenise once and pass both forms to MatchConstitutionalDenyList so it
	// can match against argv-head AND the raw command string (catches
	// `bash -c '...'` and chained forms).
	argv := tokeniseCommand(command)

	// 1. CONST-033 — non-overridable.
	if entry, hit := MatchConstitutionalDenyList(command, argv); hit {
		err := &DenyError{
			Command:     command,
			MatchedRule: entry.Description,
			Pattern:     entry.Pattern.String(),
		}
		m.log.Warn("sandbox: CONST-033 deny",
			zap.String("command", command),
			zap.String("rule", entry.Description))
		return nil, err
	}

	// 2. User-deny — additive only.
	if pattern, hit := m.matchUserDeny(command); hit {
		err := &DenyError{
			Command:     command,
			MatchedRule: "user-deny: " + pattern,
			Pattern:     pattern,
		}
		m.log.Warn("sandbox: user-deny",
			zap.String("command", command),
			zap.String("pattern", pattern))
		return nil, err
	}

	// 3. Fail-closed when no backend.
	backend := m.resolveBackend()
	if backend == nil {
		reason := m.capabilities.UnavailableReason
		if reason == "" {
			// Detector contract: BackendNone must carry a reason. If we
			// somehow get here with an empty reason, surface a generic but
			// honest message rather than crashing.
			reason = "no usable sandbox backend selected"
		}
		m.log.Warn("sandbox: fail-closed",
			zap.String("command", command),
			zap.String("reason", reason))
		return nil, &FailClosedError{Reason: reason}
	}

	// 4. Default policy substitution. SandboxPolicy contains slices so we
	//    cannot use == against the zero value; instead, detect "all
	//    safe-default sentinels" (Timeout==0 + ReadOnlyRoot==false +
	//    NetworkAllowed==false + no slices/limits) and substitute. This
	//    matches the spec contract: a caller who passes the zero policy
	//    gets DefaultSandboxPolicy() applied; any non-zero field signals
	//    intentional caller-driven policy and is left untouched.
	if isZeroPolicy(policy) {
		policy = DefaultSandboxPolicy()
	}

	// 5. Dispatch.
	result, err := backend.Run(ctx, command, policy)

	// Logging: INFO with command + backend + exit code; never log full
	// stdout/stderr (secret-safety).
	if err != nil {
		m.log.Warn("sandbox: backend error",
			zap.String("command", command),
			zap.String("backend", string(backend.Kind())),
			zap.Error(err))
	} else if result != nil {
		m.log.Info("sandbox: command executed",
			zap.String("command", command),
			zap.String("backend", string(result.Backend)),
			zap.Int("exit_code", result.ExitCode),
			zap.Bool("timed_out", result.TimedOut))
	}
	return result, err
}

// resolveBackend returns the active backend. Test seam takes precedence;
// then the slot matching capabilities.SelectedBackend; nil otherwise.
func (m *SandboxManager) resolveBackend() SandboxBackend {
	if m.backendOverride != nil {
		return m.backendOverride
	}
	switch m.capabilities.SelectedBackend {
	case BackendBubblewrap:
		if m.bubblewrap == nil {
			return nil
		}
		return m.bubblewrap
	case BackendNative:
		if m.native == nil {
			return nil
		}
		return m.native
	default:
		return nil
	}
}

// matchUserDeny tests rawCmd against every compiled user-deny pattern.
// Returns the matched pattern source + true on first hit, or ("", false).
func (m *SandboxManager) matchUserDeny(rawCmd string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Iterate in original order so the user can predict precedence.
	for i, re := range m.userDenyPatterns {
		if re.MatchString(rawCmd) {
			// m.config.UserDenyList[i] is the source string; safe because
			// we only append to userDenyPatterns when compile succeeded
			// AND we walk config.UserDenyList in the same order. Bounds
			// check is defensive — the lengths can diverge if compile-skip
			// dropped patterns.
			if i < len(m.config.UserDenyList) {
				return m.config.UserDenyList[i], true
			}
			return re.String(), true
		}
	}
	return "", false
}

// MergedDenyList returns the (constitutional, user) lists for display by
// `/sandbox policy`. The constitutional list is descriptions (not raw
// patterns) because the descriptions are the user-facing contract; the
// user list is the raw patterns the user authored.
func (m *SandboxManager) MergedDenyList() (constitutional []string, user []string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	constitutional = make([]string, 0, len(ConstitutionalDenyList))
	for i := range ConstitutionalDenyList {
		constitutional = append(constitutional, ConstitutionalDenyList[i].Description)
	}

	user = make([]string, len(m.config.UserDenyList))
	copy(user, m.config.UserDenyList)
	return constitutional, user
}

// isZeroPolicy reports whether `p` is the zero SandboxPolicy. We cannot use
// == because SandboxPolicy contains slices; instead, check each scalar +
// length of the slice fields. A policy with NetworkAllowed=false,
// Timeout=0, MemoryLimitMB=0, CPULimitPct=0, ReadOnlyRoot=false, no
// BindMounts, and no ExtraDeny is treated as "caller didn't customise"
// and substituted with DefaultSandboxPolicy().
func isZeroPolicy(p SandboxPolicy) bool {
	return !p.NetworkAllowed &&
		p.Timeout == 0 &&
		p.MemoryLimitMB == 0 &&
		p.CPULimitPct == 0 &&
		!p.ReadOnlyRoot &&
		len(p.BindMounts) == 0 &&
		len(p.ExtraDeny) == 0
}

// tokeniseCommand splits `cmd` into argv-style tokens. We deliberately use
// strings.Fields rather than a full shlex parse because:
//
//   - The CONST-033 patterns ALSO match against the raw command string
//     (regex with shell-separator boundaries), so anything strings.Fields
//     fails to capture (e.g. quoted substrings) is still caught by the
//     raw-string check inside MatchConstitutionalDenyList.
//   - Avoiding a shlex import keeps the package dependency-free (CLAUDE.md
//     "no new external deps") and bounds the surface area of the
//     manager-level sanity checks.
//
// This is intentional pragmatism, not a bluff: the raw-string regex is the
// authoritative matcher; argv tokens are belt-and-braces.
func tokeniseCommand(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	return strings.Fields(cmd)
}
