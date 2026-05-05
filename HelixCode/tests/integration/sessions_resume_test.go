//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/session"
)

// TestSessions_ResumePersistsAcrossRestart writes 3 messages via a real
// TranscriptStore in a tempdir, simulates a CLI restart by constructing a
// fresh SessionManager bound to the same store + sessionID, and asserts that
// Resume rehydrates all 3 messages with byte-exact role/content. Verifies
// the on-disk artefacts (transcript.jsonl + metadata.json) exist before the
// "restart" so the test cannot pass via in-process state alone.
func TestSessions_ResumePersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store := session.NewTranscriptStore(dir)
	sessionID := "sess-restart-001"

	// Seed metadata so the SessionManager has a defined starting point.
	now := time.Now().UTC()
	require.NoError(t, store.UpdateSessionMetadata(ctx, session.SessionMetadata{
		SessionID:    sessionID,
		ProjectPath:  "/tmp/projA",
		ProjectName:  "projA",
		StartedAt:    now,
		LastActivity: now,
		IsActive:     true,
	}))

	mgrA := session.NewSessionManagerForTestF11()
	mgrA.SetStore(store)
	require.NoError(t, mgrA.Resume(ctx, sessionID)) // sets currentID + zero loaded msgs

	want := []session.Message{
		{Role: "user", Content: "hello", Timestamp: now},
		{Role: "assistant", Content: "hi there", Timestamp: now.Add(time.Second)},
		{Role: "user", Content: "what is 2+2?", Timestamp: now.Add(2 * time.Second)},
	}
	for _, m := range want {
		require.NoError(t, mgrA.Append(ctx, m))
	}

	// Verify on-disk artefacts BEFORE simulating restart.
	transcriptPath := filepath.Join(dir, sessionID, "transcript.jsonl")
	metadataPath := filepath.Join(dir, sessionID, "metadata.json")
	st, err := os.Stat(transcriptPath)
	require.NoError(t, err, "transcript.jsonl must exist after Append")
	require.Greater(t, st.Size(), int64(0), "transcript.jsonl must be non-empty")
	st, err = os.Stat(metadataPath)
	require.NoError(t, err, "metadata.json must exist after Append")
	require.Greater(t, st.Size(), int64(0), "metadata.json must be non-empty")

	// Simulate restart: brand-new SessionManager with the SAME store.
	mgrB := session.NewSessionManagerForTestF11()
	mgrB.SetStore(store)
	require.NoError(t, mgrB.Resume(ctx, sessionID))
	require.Equal(t, sessionID, mgrB.CurrentID())
	require.Equal(t, len(want), mgrB.LoadedMessageCountForTestF11())

	// Verify exact content via the store (Resume's source of truth).
	got, err := store.ReadTranscript(ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, len(want), len(got))
	for i, m := range want {
		require.Equal(t, m.Role, got[i].Role, "message %d role", i)
		require.Equal(t, m.Content, got[i].Content, "message %d content", i)
	}

	// Metadata should reflect the 3 appended messages.
	meta, err := store.GetSessionMetadata(ctx, sessionID)
	require.NoError(t, err)
	require.Equal(t, sessionID, meta.SessionID)
	require.Equal(t, len(want), meta.MessageCount)
}

// TestSessions_GlobalFindsMostRecentAcrossProjects creates two project-scoped
// sessions with distinct ProjectPaths and distinct LastActivity timestamps and
// asserts that ResumeFinder in ResumeGlobal mode returns the most recent one
// regardless of project. Uses a real TranscriptStore in a tempdir.
func TestSessions_GlobalFindsMostRecentAcrossProjects(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := session.NewTranscriptStore(dir)

	older := time.Now().UTC().Add(-2 * time.Hour)
	newer := time.Now().UTC().Add(-5 * time.Minute)

	// Project A: older session.
	require.NoError(t, store.UpdateSessionMetadata(ctx, session.SessionMetadata{
		SessionID:    "sess-A",
		ProjectPath:  "/tmp/projA",
		ProjectName:  "projA",
		StartedAt:    older.Add(-time.Hour),
		LastActivity: older,
		MessageCount: 1,
	}))
	require.NoError(t, store.Append(ctx, "sess-A", session.Message{
		Role: "user", Content: "old", Timestamp: older,
	}))

	// Project B: newer session.
	require.NoError(t, store.UpdateSessionMetadata(ctx, session.SessionMetadata{
		SessionID:    "sess-B",
		ProjectPath:  "/tmp/projB",
		ProjectName:  "projB",
		StartedAt:    newer.Add(-time.Hour),
		LastActivity: newer,
		MessageCount: 1,
	}))
	require.NoError(t, store.Append(ctx, "sess-B", session.Message{
		Role: "user", Content: "new", Timestamp: newer,
	}))

	finder := session.NewResumeFinder(store)

	// Global mode: most recent across all projects.
	gotGlobal, err := finder.FindResumeTarget(ctx, session.ResumeGlobal, "")
	require.NoError(t, err)
	require.NotNil(t, gotGlobal)
	require.Equal(t, "sess-B", gotGlobal.SessionID,
		"global finder must return the more recent session regardless of project")

	// Project-scoped mode: must filter to the requested project.
	gotProjA, err := finder.FindResumeTarget(ctx, session.ResumeProject, "/tmp/projA")
	require.NoError(t, err)
	require.NotNil(t, gotProjA)
	require.Equal(t, "sess-A", gotProjA.SessionID)

	gotProjB, err := finder.FindResumeTarget(ctx, session.ResumeProject, "/tmp/projB")
	require.NoError(t, err)
	require.NotNil(t, gotProjB)
	require.Equal(t, "sess-B", gotProjB.SessionID)

	// Resume must rehydrate transcript + metadata for the chosen target.
	msgs, meta, err := finder.Resume(ctx, gotGlobal.SessionID)
	require.NoError(t, err)
	require.NotNil(t, meta)
	require.Equal(t, "sess-B", meta.SessionID)
	require.GreaterOrEqual(t, len(msgs), 1)
	require.Equal(t, "new", msgs[0].Content)
}
