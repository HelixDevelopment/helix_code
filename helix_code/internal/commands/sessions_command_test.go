package commands

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/session"
)

func newSessionsCommand(t *testing.T) (*SessionsCommand, *session.TranscriptStore) {
	t.Helper()
	dir := t.TempDir()
	store := session.NewTranscriptStore(dir)
	return NewSessionsCommand(store, "/p/test"), store
}

func seedSession(t *testing.T, store *session.TranscriptStore, id, project string, lastActivity time.Time) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, store.UpdateSessionMetadata(ctx, session.SessionMetadata{
		SessionID:    id,
		ProjectPath:  project,
		ProjectName:  "p",
		StartedAt:    lastActivity.Add(-time.Hour),
		LastActivity: lastActivity,
		MessageCount: 1,
	}))
}

func TestSlashSessions_ListEmpty(t *testing.T) {
	// round-432: /sessions output is CONST-046-migrated; wire the
	// interpolatingTranslator so column headers render to English.
	resetTranslator(t)
	SetTranslator(interpolatingTranslator{})
	defer resetTranslator(t)

	c, _ := newSessionsCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "ID")
}

func TestSlashSessions_ListShowsSessions(t *testing.T) {
	c, store := newSessionsCommand(t)
	now := time.Now().UTC().Truncate(time.Second)
	seedSession(t, store, "s1", "/p/test", now)
	seedSession(t, store, "s2", "/p/other", now.Add(-time.Hour))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	// project-scoped by default (NewSessionsCommand was given /p/test)
	assert.Contains(t, res.Output, "s1")
	assert.NotContains(t, res.Output, "s2")
}

func TestSlashSessions_ListAll(t *testing.T) {
	c, store := newSessionsCommand(t)
	now := time.Now().UTC().Truncate(time.Second)
	seedSession(t, store, "s1", "/p/test", now)
	seedSession(t, store, "s2", "/p/other", now.Add(-time.Hour))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list", "--all"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "s1")
	assert.Contains(t, res.Output, "s2")
}

func TestSlashSessions_Show(t *testing.T) {
	// round-432: /sessions show report is CONST-046-migrated; wire the
	// interpolatingTranslator so the rendered output carries the real
	// session ID for the assertion below.
	resetTranslator(t)
	SetTranslator(interpolatingTranslator{})
	defer resetTranslator(t)

	c, store := newSessionsCommand(t)
	now := time.Now().UTC().Truncate(time.Second)
	seedSession(t, store, "s1", "/p/test", now)
	require.NoError(t, store.Append(context.Background(), "s1",
		session.Message{Role: "user", Content: "hello", Timestamp: now}))
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "s1"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "s1")
	assert.Contains(t, res.Output, "hello")
}

func TestSlashSessions_ShowUnknownErrors(t *testing.T) {
	c, _ := newSessionsCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"show", "ghost"}})
	require.Error(t, err)
}

func TestSlashSessions_Delete(t *testing.T) {
	c, store := newSessionsCommand(t)
	now := time.Now().UTC().Truncate(time.Second)
	seedSession(t, store, "s1", "/p/test", now)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"delete", "s1"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "deleted")
	_, err = store.GetSessionMetadata(context.Background(), "s1")
	require.Error(t, err)
}

func TestSlashSessions_DeleteUnknownErrors(t *testing.T) {
	c, _ := newSessionsCommand(t)
	// Delete is idempotent: removing a non-existent dir succeeds. We expect no error.
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"delete", "ghost"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "deleted")
}

func TestSlashSessions_DefaultIsList(t *testing.T) {
	// round-432: /sessions output is CONST-046-migrated; wire the
	// interpolatingTranslator so column headers render to English.
	resetTranslator(t)
	SetTranslator(interpolatingTranslator{})
	defer resetTranslator(t)

	c, _ := newSessionsCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "ID")
}

func TestSlashSessions_UnknownSubcommandErrors(t *testing.T) {
	c, _ := newSessionsCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
}
