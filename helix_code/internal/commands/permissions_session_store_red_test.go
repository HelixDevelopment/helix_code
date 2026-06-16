package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// RED_MODE polarity switch per §11.4.115 / §11.4.135.
//
//	HELIXCODE_PERMISSIONS_RED=1: reproduce the documented bluff on a PRE-FIX
//	            artifact — /permissions add stores nothing, so /permissions list
//	            never shows the just-added rule. This run captures positive
//	            evidence that the defect is present (passes ONLY on broken code).
//	default / =0: the standing GREEN regression guard — the same source asserts
//	            the round-trip works (add → list shows → remove → list omits).
//
// Default is the GREEN guard so the suite stays green in CI; flip the env var to
// reproduce the historical defect on the broken artifact. The bug-catcher IS the
// regression guard (one source, two roles).
func redMode() bool {
	return os.Getenv("HELIXCODE_PERMISSIONS_RED") == "1"
}

// emptyUserPermissionsHome wires HOME to a tempdir holding an empty (rules-less)
// user permissions.yaml so list output is deterministic and contains no rule
// patterns from disk. Returns nothing; the command resolves the file via HOME.
func emptyUserPermissionsHome(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Run from the tempdir too so the ProjectPath (cwd/.helixcode/...) is also
	// empty and cannot leak rules into the list output.
	cwd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	require.NoError(t, os.Chdir(tmp))

	userDir := filepath.Join(tmp, ".helixcode")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "permissions.yaml"),
		[]byte("apiVersion: helixcode.permissions/v1\nmode: default\nrules: []\n"), 0o600))
}

// TestPermissions_AddThenList_RoundTrip is the polarity-switched test.
//
// On the broken artifact (the addSession/removeSession no-op stubs that discard
// the parsed action), RED_MODE=1 asserts the defect is present: after `add`, the
// pattern does NOT appear in `list`. Once the real session-rule store lands, the
// same assertion under RED_MODE=0 asserts the rule IS shown after add and is
// gone after remove.
func TestPermissions_AddThenList_RoundTrip(t *testing.T) {
	emptyUserPermissionsHome(t)
	ctx := context.Background()
	const pattern = "Bash(deploy --prod:*)"

	cmd := NewPermissionsCommand()

	// add the rule
	_, err := cmd.Execute(ctx, &CommandContext{
		Args:     []string{"add", pattern, "deny", "50"},
		RawInput: "/permissions add " + pattern + " deny 50",
	})
	require.NoError(t, err)

	// list and inspect
	listRes, err := cmd.Execute(ctx, &CommandContext{Args: []string{}, RawInput: "/permissions"})
	require.NoError(t, err)
	require.NotNil(t, listRes)

	if redMode() {
		// Reproduce the bluff on the current artifact: the stub discarded the
		// rule, so list cannot show it.
		require.NotContains(t, listRes.Output, pattern,
			"RED expectation: stub discards the rule so list must NOT show it; "+
				"if this fails the bluff is already fixed — flip HELIXCODE_PERMISSIONS_RED=0")
		return
	}

	// GREEN guard: the real store persisted the rule; list shows it.
	require.Contains(t, listRes.Output, pattern,
		"after add, /permissions list must show the stored rule")

	// remove the rule
	_, err = cmd.Execute(ctx, &CommandContext{
		Args:     []string{"remove", pattern},
		RawInput: "/permissions remove " + pattern,
	})
	require.NoError(t, err)

	// list again: the rule must be gone
	listRes2, err := cmd.Execute(ctx, &CommandContext{Args: []string{}, RawInput: "/permissions"})
	require.NoError(t, err)
	require.NotContains(t, listRes2.Output, pattern,
		"after remove, /permissions list must NOT show the rule")
}
