package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResumeFinder_FindMostRecentInProject(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, store.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID: "old", ProjectPath: "/p/one", LastActivity: now.Add(-1 * time.Hour),
	}))
	require.NoError(t, store.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID: "new", ProjectPath: "/p/one", LastActivity: now,
	}))
	require.NoError(t, store.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID: "other", ProjectPath: "/p/two", LastActivity: now.Add(time.Hour),
	}))

	rf := NewResumeFinder(store)
	meta, err := rf.FindResumeTarget(ctx, ResumeProject, "/p/one")
	require.NoError(t, err)
	assert.Equal(t, "new", meta.SessionID)
}

func TestResumeFinder_FindMostRecentGlobal(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, store.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID: "a", ProjectPath: "/p/one", LastActivity: now.Add(-1 * time.Hour),
	}))
	require.NoError(t, store.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID: "b", ProjectPath: "/p/two", LastActivity: now,
	}))

	rf := NewResumeFinder(store)
	meta, err := rf.FindResumeTarget(ctx, ResumeGlobal, "/p/one")
	require.NoError(t, err)
	assert.Equal(t, "b", meta.SessionID, "global mode should return most recent across all projects")
}

func TestResumeFinder_NoSessions(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	rf := NewResumeFinder(store)
	_, err := rf.FindResumeTarget(context.Background(), ResumeGlobal, "")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no sessions"), "expected 'no sessions' in error: %v", err)
}

func TestResumeFinder_Resume_LoadsTranscript(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	ctx := context.Background()
	sid := "load-me"

	require.NoError(t, store.Append(ctx, sid, Message{Role: "user", Content: "first", Timestamp: time.Now()}))
	require.NoError(t, store.Append(ctx, sid, Message{Role: "assistant", Content: "second", Timestamp: time.Now()}))
	require.NoError(t, store.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID: sid, ProjectPath: "/p", MessageCount: 2,
	}))

	rf := NewResumeFinder(store)
	msgs, meta, err := rf.Resume(ctx, sid)
	require.NoError(t, err)
	assert.Equal(t, sid, meta.SessionID)
	require.Len(t, msgs, 2)
	assert.Equal(t, "first", msgs[0].Content)
	assert.Equal(t, "second", msgs[1].Content)
}

func TestResumeFinder_Resume_UnknownErrors(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	rf := NewResumeFinder(store)
	_, _, err := rf.Resume(context.Background(), "ghost")
	require.Error(t, err)
	// Should be non-nil whether due to missing transcript or missing metadata
	_ = errors.Is(err, errors.New("dummy"))
}
