package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/session"
)

func setupTempStore(t *testing.T) (string, *session.TranscriptStore) {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "sessions"), session.NewTranscriptStore(filepath.Join(dir, "sessions"))
}

func TestSessionsCmd_List(t *testing.T) {
	_, store := setupTempStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, store.UpdateSessionMetadata(context.Background(), session.SessionMetadata{
		SessionID: "s1", ProjectPath: "/p/here", ProjectName: "here",
		StartedAt: now, LastActivity: now, MessageCount: 3,
	}))
	cmd := newSessionsCmd(sessionsCmdDeps{Store: store, CurrentProject: "/p/here"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "s1")
	assert.Contains(t, buf.String(), "here")
}

func TestSessionsCmd_Show(t *testing.T) {
	_, store := setupTempStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, store.UpdateSessionMetadata(context.Background(), session.SessionMetadata{
		SessionID: "s1", ProjectPath: "/p/here", ProjectName: "here",
		StartedAt: now, LastActivity: now, MessageCount: 1,
	}))
	require.NoError(t, store.Append(context.Background(), "s1", session.Message{
		Role: "user", Content: "show-me", Timestamp: now,
	}))
	cmd := newSessionsCmd(sessionsCmdDeps{Store: store, CurrentProject: "/p/here"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"show", "s1"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "show-me")
}

func TestSessionsCmd_Delete(t *testing.T) {
	_, store := setupTempStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, store.UpdateSessionMetadata(context.Background(), session.SessionMetadata{
		SessionID: "s1", ProjectPath: "/p/here", ProjectName: "here",
		StartedAt: now, LastActivity: now,
	}))
	cmd := newSessionsCmd(sessionsCmdDeps{Store: store, CurrentProject: "/p/here"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"delete", "s1"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "deleted")
}
