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
