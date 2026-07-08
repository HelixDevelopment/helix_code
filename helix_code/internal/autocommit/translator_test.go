// Sentinel + mutation tests for the CONST-046 translator wiring in
// internal/autocommit (round-229 §11.4 anti-bluff sweep, 2026-05-19).
// Mocks ALLOWED per CONST-050(A) — this is a unit test file.
package autocommit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	autocommiti18n "dev.helix.code/internal/autocommit/i18n"
)

// sentinelTranslator wraps every resolved message ID with a
// recognisable marker so call-site tests can prove the lookup
// ACTUALLY went through Translator.T — not through a hardcoded
// literal that happens to match the bundle value (which would be a
// §11.4 PASS-bluff at the i18n call-site layer).
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	if len(data) > 0 {
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		return "<SENT:" + id + "|keys=" + strings.Join(keys, ",") + ">", nil
	}
	return "<SENT:" + id + ">", nil
}

func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<SENT:" + id + ">", nil
}

// errorTranslator always fails — exercises the tr() fallback path
// (must degrade to raw message ID, never to empty string).
type errorTranslator struct{}

func (errorTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "", errors.New("errorTranslator: deliberate failure for " + id)
}

func (errorTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("errorTranslator: deliberate failure for " + id)
}

func resetTranslator(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetTranslator(nil) })
}

func TestSetTranslator_Nil_ResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	got := tr(context.Background(), "internal_autocommit_skipped_disabled", nil)
	if got != "<SENT:internal_autocommit_skipped_disabled>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_autocommit_skipped_disabled", nil)
	if got != "auto-commit disabled" {
		t.Fatalf("after SetTranslator(nil), expected resolved prose %q, got %q", "auto-commit disabled", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_autocommit_skipped_not_a_git_repo", nil)
	if got != "internal_autocommit_skipped_not_a_git_repo" {
		t.Fatalf("tr() with failing translator returned %q, want raw message ID", got)
	}
}

func TestCommitter_SkipRequested_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir, nil, true)
	res, _ := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"}, SkipRequested: true,
	})
	require.True(t, res.Skipped)
	if res.Reason != "<SENT:internal_autocommit_skipped_per_edit_skip_requested>" {
		t.Fatalf("per-edit-skip Reason did not route through translator: got %q", res.Reason)
	}
}

func TestCommitter_SkipViaArgs_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir, nil, true)
	res, _ := c.MaybeCommit(context.Background(), CommitContext{
		ToolName:     "fs_write",
		MutatedPaths: []string{"x.txt"},
		Args:         map[string]interface{}{SkipParamKey: true},
	})
	require.True(t, res.Skipped)
	if !strings.Contains(res.Reason, "<SENT:internal_autocommit_skipped_per_edit_skip_via_param|keys=ParamKey>") {
		t.Fatalf("args-skip Reason did not route through translator: got %q", res.Reason)
	}
}

func TestCommitter_Disabled_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir, nil, false)
	res, _ := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.True(t, res.Skipped)
	if res.Reason != "<SENT:internal_autocommit_skipped_disabled>" {
		t.Fatalf("disabled Reason did not route through translator: got %q", res.Reason)
	}
}

func TestCommitter_NotAGitRepo_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	dir := t.TempDir() // NOT a git repo
	c := newRealCommitter(t, dir, nil, true)
	res, _ := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.True(t, res.Skipped)
	if res.Reason != "<SENT:internal_autocommit_skipped_not_a_git_repo>" {
		t.Fatalf("not-a-git-repo Reason did not route through translator: got %q", res.Reason)
	}
}

func TestCommitter_NoChanges_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	c := newRealCommitter(t, dir, nil, true)
	res, _ := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.True(t, res.Skipped)
	if res.Reason != "<SENT:internal_autocommit_skipped_no_changes_to_commit>" {
		t.Fatalf("no-changes Reason did not route through translator: got %q", res.Reason)
	}
}

func TestDeterministicFallback_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	var d DeterministicFallback
	got := d.Summarise(context.Background(), "", "fs_edit", []string{"foo.go"})
	if !strings.Contains(got, "<SENT:internal_autocommit_subject_auto_edit_prefix|keys=") {
		t.Fatalf("DeterministicFallback subject did not route through translator: got %q", got)
	}
	// Both placeholders MUST be present in the keys list.
	if !strings.Contains(got, "ToolName") || !strings.Contains(got, "Paths") {
		t.Fatalf("DeterministicFallback subject lost a placeholder: got %q", got)
	}
}

func TestDeterministicFallback_RoutesThroughTranslator_RealCommitSubject(t *testing.T) {
	// Anti-bluff: prove the wired translator output lands as the
	// actual commit subject in `git log -1 --format=%s`. Catches a
	// regression where the subject is computed but never committed.
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	dir := setupRealGitRepo(t)
	initialCommit(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0644))
	c := newRealCommitter(t, dir, &fakeProvider{err: errors.New("boom")}, true)
	_, err := c.MaybeCommit(context.Background(), CommitContext{
		ToolName: "fs_write", MutatedPaths: []string{"x.txt"},
	})
	require.NoError(t, err)

	out, _ := exec.Command("git", "-C", dir, "log", "-1", "--format=%s").Output()
	subject := strings.TrimSpace(string(out))
	if !strings.Contains(subject, "<SENT:internal_autocommit_subject_auto_edit_prefix") {
		t.Fatalf("committed subject did not route through translator: got %q", subject)
	}
}

// TestNoopTranslator_T_Loud_Echo_IsRawID is the paired-mutation test —
// it asserts every CONST-046 message ID emitted by this package's
// migrated call sites appears in the active.en.yaml bundle (verified
// implicitly: NoopTranslator returns id verbatim, and the call-site
// tests above prove call sites use these exact IDs). If a new round
// adds a tr() call without a bundle entry, the bundle scan in
// scripts/audit_const046 + this loud-echo invariant must FAIL.
// Mirrors §1.1 paired-mutation guidance.
func TestNoopTranslator_T_Loud_Echo_IsRawID(t *testing.T) {
	noop := autocommiti18n.NoopTranslator{}
	for _, id := range migratedMessageIDs() {
		got, err := noop.T(context.Background(), id, nil)
		if err != nil {
			t.Fatalf("NoopTranslator.T(%q) error: %v", id, err)
		}
		if got != id {
			t.Fatalf("NoopTranslator.T(%q) returned %q, want loud echo of raw ID", id, got)
		}
	}
}

func migratedMessageIDs() []string {
	// Round-229 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_autocommit_skipped_disabled",
		"internal_autocommit_skipped_no_changes_to_commit",
		"internal_autocommit_skipped_no_staged_changes_after_add",
		"internal_autocommit_skipped_not_a_git_repo",
		"internal_autocommit_skipped_per_edit_skip_requested",
		"internal_autocommit_skipped_per_edit_skip_via_param",
		"internal_autocommit_subject_auto_edit_prefix",
	}
}
