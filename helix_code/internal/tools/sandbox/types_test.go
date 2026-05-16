package sandbox

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackendKind_String(t *testing.T) {
	tests := []struct {
		kind BackendKind
		want string
	}{
		{BackendBubblewrap, "bubblewrap"},
		{BackendNative, "native"},
		{BackendNone, "none"},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			require.Equal(t, tt.want, tt.kind.String())
		})
	}
}

func TestDefaultSandboxPolicy(t *testing.T) {
	p := DefaultSandboxPolicy()

	require.False(t, p.NetworkAllowed, "default policy MUST deny network (Q3=A)")
	require.Equal(t, 30*time.Second, p.Timeout, "default timeout must be 30s")
	require.Equal(t, 0, p.MemoryLimitMB, "default memory limit is 0 (no limit)")
	require.Equal(t, 0, p.CPULimitPct, "default cpu limit is 0 (no limit)")
	require.True(t, p.ReadOnlyRoot, "default policy must mount root read-only")
	require.Empty(t, p.BindMounts, "default policy has no bind mounts")
	require.Empty(t, p.ExtraDeny, "default policy has no extra deny entries")
}

func TestDefaultSandboxConfig(t *testing.T) {
	cfg := DefaultSandboxConfig()
	require.Equal(t, DefaultSandboxPolicy(), cfg.DefaultPolicy)
	require.Empty(t, cfg.UserDenyList, "DefaultSandboxConfig must have empty UserDenyList")
}

func TestSandboxResult_JSONRoundTrip(t *testing.T) {
	orig := SandboxResult{
		Stdout:   "hello\n",
		Stderr:   "warn\n",
		ExitCode: 0,
		TimedOut: false,
		Backend:  BackendBubblewrap,
		Duration: 250 * time.Millisecond,
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got SandboxResult
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, orig, got)
}

func TestSandboxCapabilities_FailClosedHasReason(t *testing.T) {
	caps := SandboxCapabilities{
		GOOS:              "linux",
		SelectedBackend:   BackendNone,
		UnavailableReason: "no usable backend: bubblewrap missing AND unprivileged userns disabled",
	}
	require.Equal(t, BackendNone, caps.SelectedBackend)
	require.NotEmpty(t, caps.UnavailableReason,
		"when SelectedBackend == BackendNone, UnavailableReason MUST be populated")
}

func TestConstitutionalDenyList_NotEmpty(t *testing.T) {
	require.NotEmpty(t, ConstitutionalDenyList,
		"ConstitutionalDenyList must be populated at package init")
	require.GreaterOrEqual(t, len(ConstitutionalDenyList), 7,
		"expected at least the canonical CONST-033 categories")

	for i, e := range ConstitutionalDenyList {
		require.NotNil(t, e.Pattern, "entry %d Pattern must be a compiled regex", i)
		require.NotEmpty(t, e.Description, "entry %d Description must be non-empty", i)
	}
}

func tokenize(s string) []string {
	// minimal whitespace tokeniser for tests; matches MatchConstitutionalDenyList's
	// expectation that callers pre-split argv however they like.
	out := []string{}
	cur := []rune{}
	flush := func() {
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = cur[:0]
		}
	}
	for _, r := range s {
		if r == ' ' || r == '\t' {
			flush()
			continue
		}
		cur = append(cur, r)
	}
	flush()
	return out
}

func TestMatchConstitutionalDenyList_Systemctl(t *testing.T) {
	cases := []string{
		"systemctl suspend",
		"systemctl   poweroff",
		"systemctl reboot now",
		"systemctl hibernate",
		"systemctl hybrid-sleep",
		"systemctl suspend-then-hibernate",
		"systemctl halt",
		"systemctl kexec",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			entry, ok := MatchConstitutionalDenyList(c, tokenize(c))
			require.True(t, ok, "expected match for %q", c)
			require.NotNil(t, entry)
			require.NotEmpty(t, entry.Description)
		})
	}
}

func TestMatchConstitutionalDenyList_NestedBashC(t *testing.T) {
	raw := `bash -c 'systemctl suspend'`
	// argv[0]=bash so tokenised argv-head match would not catch this.
	// Raw word-boundary regex MUST match.
	entry, ok := MatchConstitutionalDenyList(raw, tokenize(raw))
	require.True(t, ok, "raw regex must catch nested bash -c '...power-mgmt...'")
	require.NotNil(t, entry)
}

func TestMatchConstitutionalDenyList_ChainedSemicolon(t *testing.T) {
	raw := "ls; systemctl suspend"
	entry, ok := MatchConstitutionalDenyList(raw, tokenize(raw))
	require.True(t, ok, "raw regex must catch chained ;-separated commands")
	require.NotNil(t, entry)
}

func TestMatchConstitutionalDenyList_Loginctl(t *testing.T) {
	raw := "loginctl terminate-user $USER"
	entry, ok := MatchConstitutionalDenyList(raw, tokenize(raw))
	require.True(t, ok, "loginctl terminate-user must be denied")
	require.NotNil(t, entry)
}

func TestMatchConstitutionalDenyList_DbusSend(t *testing.T) {
	raw := "dbus-send --system --print-reply --dest=org.freedesktop.login1 /org/freedesktop/login1 org.freedesktop.login1.Manager.Suspend boolean:true"
	entry, ok := MatchConstitutionalDenyList(raw, tokenize(raw))
	require.True(t, ok, "dbus-send to login1 power method must be denied")
	require.NotNil(t, entry)
}

func TestMatchConstitutionalDenyList_EchoToPowerState(t *testing.T) {
	raw := "echo mem > /sys/power/state"
	entry, ok := MatchConstitutionalDenyList(raw, tokenize(raw))
	require.True(t, ok, "writing to /sys/power/state must be denied")
	require.NotNil(t, entry)
}

func TestMatchConstitutionalDenyList_BareShutdownReboot(t *testing.T) {
	cases := []string{
		"shutdown -h now",
		"poweroff",
		"halt",
		"reboot",
		"kexec -e",
		"pm-suspend",
		"pm-hibernate",
		"pm-suspend-hybrid",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			entry, ok := MatchConstitutionalDenyList(c, tokenize(c))
			require.True(t, ok, "expected match for %q", c)
			require.NotNil(t, entry)
		})
	}
}

func TestMatchConstitutionalDenyList_BenignDoesNotMatch(t *testing.T) {
	cases := []string{
		"ls -la",
		"git status",
		"echo hello",
		"cat /etc/hostname",
		"systemctl status nginx",   // status is fine
		"systemctl list-units",     // list-units is fine
		"loginctl list-sessions",   // list-sessions is fine
		"dbus-send --session --dest=org.example.Service /a b.c",
		"echo hello > /tmp/out",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			entry, ok := MatchConstitutionalDenyList(c, tokenize(c))
			require.False(t, ok, "%q must NOT match deny-list", c)
			require.Nil(t, entry)
		})
	}
}

func TestMatchConstitutionalDenyList_PartialWordsDoNotFalsePositive(t *testing.T) {
	cases := []string{
		"mysystemctl suspend", // prefixed binary name
		"./reboot-helper.sh",  // reboot is part of a larger word/path
		"poweroff_test_runner",
		"echo halts and catches fire",
		"echo The shutdown procedure",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			entry, ok := MatchConstitutionalDenyList(c, tokenize(c))
			require.False(t, ok, "%q should NOT match (word boundary)", c)
			require.Nil(t, entry)
		})
	}
}
