package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestMessage builds a Message using the actual Message type defined in
// this package.
func makeTestMessage(role, content string) Message {
	return Message{Role: role, Content: content}
}

func TestTranscriptStore_AppendReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	ctx := context.Background()
	sid := "session-1"

	require.NoError(t, store.Append(ctx, sid, makeTestMessage("user", "hello")))
	require.NoError(t, store.Append(ctx, sid, makeTestMessage("assistant", "hi back")))
	require.NoError(t, store.Append(ctx, sid, makeTestMessage("user", "how are you")))

	msgs, err := store.ReadTranscript(ctx, sid)
	require.NoError(t, err)
	require.Len(t, msgs, 3)
	assert.Equal(t, "hello", msgs[0].Content)
	assert.Equal(t, "hi back", msgs[1].Content)
	assert.Equal(t, "how are you", msgs[2].Content)
}

func TestTranscriptStore_HandlesCorruptedLine(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	ctx := context.Background()
	sid := "session-bad"

	require.NoError(t, store.Append(ctx, sid, makeTestMessage("user", "ok line")))
	// Inject a corrupt line directly into the JSONL file
	path := filepath.Join(dir, sid, "transcript.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, _ = f.WriteString("this is not JSON\n")
	require.NoError(t, f.Close())
	require.NoError(t, store.Append(ctx, sid, makeTestMessage("user", "after corrupt")))

	msgs, err := store.ReadTranscript(ctx, sid)
	require.NoError(t, err)
	// Two valid lines + one skipped = 2 returned
	require.Len(t, msgs, 2)
	assert.Equal(t, "ok line", msgs[0].Content)
	assert.Equal(t, "after corrupt", msgs[1].Content)
}

func TestTranscriptStore_MetadataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	meta := SessionMetadata{
		SessionID:    "sm-1",
		ProjectPath:  "/tmp/proj",
		ProjectName:  "proj",
		StartedAt:    now,
		LastActivity: now.Add(time.Minute),
		MessageCount: 7,
		IsActive:     true,
	}
	require.NoError(t, store.UpdateSessionMetadata(ctx, meta))

	got, err := store.GetSessionMetadata(ctx, "sm-1")
	require.NoError(t, err)
	assert.Equal(t, meta.ProjectName, got.ProjectName)
	assert.Equal(t, meta.MessageCount, got.MessageCount)
}

func TestTranscriptStore_ListProjectScoped(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	ctx := context.Background()
	require.NoError(t, store.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID: "a", ProjectPath: "/p/one",
	}))
	require.NoError(t, store.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID: "b", ProjectPath: "/p/one",
	}))
	require.NoError(t, store.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID: "c", ProjectPath: "/p/two",
	}))

	one, err := store.ListSessionMetadata(ctx, "/p/one")
	require.NoError(t, err)
	assert.Len(t, one, 2)

	all, err := store.ListSessionMetadata(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestTranscriptStore_DeleteSession(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	ctx := context.Background()
	sid := "del-me"
	require.NoError(t, store.Append(ctx, sid, makeTestMessage("user", "x")))
	require.NoError(t, store.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID: sid, ProjectPath: "/p",
	}))

	require.NoError(t, store.DeleteSession(ctx, sid))
	_, err := os.Stat(filepath.Join(dir, sid))
	assert.True(t, os.IsNotExist(err))
}

func TestTranscriptStore_MetadataResynthesis(t *testing.T) {
	dir := t.TempDir()
	store := NewTranscriptStore(dir)
	ctx := context.Background()
	sid := "resynth"

	require.NoError(t, store.Append(ctx, sid, makeTestMessage("user", "first")))
	require.NoError(t, store.Append(ctx, sid, makeTestMessage("assistant", "second")))
	require.NoError(t, store.Append(ctx, sid, makeTestMessage("user", "third")))

	// Delete the metadata.json sidecar
	metaPath := filepath.Join(dir, sid, "metadata.json")
	require.NoError(t, os.Remove(metaPath))

	// GetSessionMetadata should resynthesise from the JSONL
	got, err := store.GetSessionMetadata(ctx, sid)
	require.NoError(t, err)
	assert.Equal(t, sid, got.SessionID)
	assert.Equal(t, 3, got.MessageCount)
	// also should write the resynthesised metadata back
	_, err = os.Stat(metaPath)
	assert.NoError(t, err)

	// Confirm via Marshal that metadata is also valid JSON
	data, err := os.ReadFile(metaPath)
	require.NoError(t, err)
	var parsed SessionMetadata
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, 3, parsed.MessageCount)
}
