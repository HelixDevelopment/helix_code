package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newManagerWithStore builds a SessionManager wired to a tempdir TranscriptStore.
func newManagerWithStore(t *testing.T) (*SessionManager, *TranscriptStore, string) {
	t.Helper()
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	mgr := NewSessionManagerForTestF11()
	mgr.SetStore(store)
	sid := "f11-test-session"
	mgr.setCurrentID(sid)
	return mgr, store, sid
}

func TestSessionManager_AppendPersists(t *testing.T) {
	mgr, store, sid := newManagerWithStore(t)
	ctx := context.Background()

	msg := Message{Role: "user", Content: "hello", Timestamp: time.Now()}
	require.NoError(t, mgr.Append(ctx, msg))

	got, err := store.ReadTranscript(ctx, sid)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "hello", got[0].Content)
}

func TestSessionManager_AppendUpdatesMetadata(t *testing.T) {
	mgr, store, sid := newManagerWithStore(t)
	ctx := context.Background()

	require.NoError(t, mgr.Append(ctx, Message{Role: "user", Content: "a", Timestamp: time.Now()}))
	require.NoError(t, mgr.Append(ctx, Message{Role: "assistant", Content: "b", Timestamp: time.Now()}))

	meta, err := store.GetSessionMetadata(ctx, sid)
	require.NoError(t, err)
	assert.Equal(t, 2, meta.MessageCount)
}

func TestSessionManager_Resume_LoadsTranscript(t *testing.T) {
	mgr, store, sid := newManagerWithStore(t)
	ctx := context.Background()

	// Pre-seed transcript via the store
	require.NoError(t, store.Append(ctx, sid, Message{Role: "user", Content: "first", Timestamp: time.Now()}))
	require.NoError(t, store.Append(ctx, sid, Message{Role: "assistant", Content: "second", Timestamp: time.Now()}))

	// Resume should load the transcript into manager state
	require.NoError(t, mgr.Resume(ctx, sid))
	assert.Equal(t, sid, mgr.CurrentID())
	assert.GreaterOrEqual(t, mgr.LoadedMessageCountForTestF11(), 2)
}

func TestSessionManager_CurrentID(t *testing.T) {
	mgr, _, sid := newManagerWithStore(t)
	assert.Equal(t, sid, mgr.CurrentID())
}

func TestSessionManager_AppendNoStore_NoError(t *testing.T) {
	mgr := NewSessionManagerForTestF11()
	// no store wired → Append should succeed silently (no-op)
	err := mgr.Append(context.Background(), Message{Role: "user", Content: "x", Timestamp: time.Now()})
	assert.NoError(t, err)
}

// TestSessionManager_Append_PreservesProjectMetadata is the regression test for
// the F11-T08 bug: SessionManager.Append used to overwrite the on-disk metadata
// sidecar with a freshly-constructed SessionMetadata that did not carry
// ProjectPath / ProjectName / BranchName / original StartedAt. That made
// project-scoped lookup (ListSessionMetadata(ctx, projectPath)) silently miss
// any session that had been appended to.
//
// After the fix, Append must read the existing metadata, mutate only
// LastActivity + MessageCount (keeping IsActive true while the session is
// active), and preserve every other persisted field byte-exact.
func TestSessionManager_Append_PreservesProjectMetadata(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewTranscriptStore(dir)

	sid := "f11-preserve-meta"
	startedAt := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

	// Seed metadata with the full identity payload that production code wires.
	seed := SessionMetadata{
		SessionID:    sid,
		ProjectPath:  "/foo/bar",
		ProjectName:  "bar",
		StartedAt:    startedAt,
		LastActivity: startedAt,
		MessageCount: 0,
		IsActive:     true,
		BranchName:   "main",
	}
	require.NoError(t, store.UpdateSessionMetadata(ctx, seed))

	// Construct manager and bind to the seeded session via Resume so the
	// in-memory startedAt/currentID are aligned with the on-disk record.
	mgr := NewSessionManagerForTestF11()
	mgr.SetStore(store)
	require.NoError(t, mgr.Resume(ctx, sid))

	require.NoError(t, mgr.Append(ctx, Message{Role: "user", Content: "one", Timestamp: time.Now()}))
	require.NoError(t, mgr.Append(ctx, Message{Role: "assistant", Content: "two", Timestamp: time.Now()}))

	got, err := store.GetSessionMetadata(ctx, sid)
	require.NoError(t, err)

	assert.Equal(t, "/foo/bar", got.ProjectPath, "Append must preserve ProjectPath")
	assert.Equal(t, "bar", got.ProjectName, "Append must preserve ProjectName")
	assert.Equal(t, "main", got.BranchName, "Append must preserve BranchName")
	assert.True(t, startedAt.Equal(got.StartedAt), "Append must preserve StartedAt: got %v want %v", got.StartedAt, startedAt)
	assert.True(t, got.IsActive, "Append must keep IsActive=true while the session is active")
	assert.Equal(t, 2, got.MessageCount, "Append must update MessageCount")
	assert.True(t, got.LastActivity.After(startedAt), "Append must advance LastActivity past StartedAt")

	// Project-scoped listing must still find the session — this is the
	// downstream consequence the harness was tripping over.
	matches, err := store.ListSessionMetadata(ctx, "/foo/bar")
	require.NoError(t, err)
	require.Len(t, matches, 1, "ListSessionMetadata(/foo/bar) must return the session after Append")
	assert.Equal(t, sid, matches[0].SessionID)
}
