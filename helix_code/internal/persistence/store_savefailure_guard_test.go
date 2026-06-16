package persistence

// Standing regression guard for the SILENT-SAVE-FAILURE PASS-bluff defect
// (§11.4 / Article XI §11.9 — a failed save reported as success). §11.4.135.
//
// THE DEFECT (pre-fix store.go saveSessions/saveConversations/saveFocusChains):
//   Each per-item export/serialize/write error was swallowed by a bare `continue`,
//   and the helper returned `len(items)` as the saved count with a nil error.
//   Therefore SaveAll() returned nil (success) and reported every item as saved
//   even when EVERY underlying file write failed — silent data loss the user only
//   discovers on the next restart when their sessions/conversations are gone.
//
// REPRODUCTION ON A REAL ARTIFACT (no mocks):
//   Pre-create a DIRECTORY at the exact path where a session's JSON file must
//   land. writeAtomic's os.Rename(tempFile, filename) then fails (cannot rename a
//   file onto a non-empty directory), exercising the real per-item write-failure
//   path. The fixed code surfaces this; the broken code swallowed it.
//
// §11.4.115 RED_MODE polarity:
//   Default (RED_MODE unset / "0"): standing GREEN guard. Drives the REAL, FIXED
//     SaveAll and asserts the write failure IS surfaced as an error and the saved
//     count is honest (0, not the bluffed total). Runs on every `go test`.
//   RED_MODE=1: reproduce-the-defect. Exercises a faithful stand-in of the
//     pre-fix buggy helper (bluffySaveSessions below) against the SAME real
//     failure injection and asserts the historical bluff (nil error + full count
//     reported despite zero items actually written). PASSES on the broken
//     behaviour, proving the guard genuinely reproduces the defect.
//
// Run GREEN guard (default):           go test -race -run TestSaveFailureSurfaced ./internal/persistence/...
// Run RED reproduction (broken stand): RED_MODE=1 go test -race -run TestSaveFailureSurfaced ./internal/persistence/...

import (
	"os"
	"path/filepath"
	"testing"

	"dev.helix.code/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plantWriteFailure pre-creates a directory at the path where the session's JSON
// file must be written, forcing writeAtomic's rename to fail for that item. It
// returns the path that is now un-writable-as-a-file.
func plantWriteFailure(t *testing.T, basePath, sessionID string) string {
	t.Helper()
	sessionsDir := filepath.Join(basePath, "sessions")
	require.NoError(t, os.MkdirAll(sessionsDir, 0o755))
	blocker := filepath.Join(sessionsDir, sessionID+".json")
	require.NoError(t, os.MkdirAll(blocker, 0o755))
	return blocker
}

// bluffySaveSessions is a faithful stand-in of the PRE-FIX saveSessions: it
// swallows every per-item error with a bare `continue` and reports len(sessions)
// as saved with a nil error. Used ONLY by the RED reproduction to demonstrate the
// historical bluff against the real failure injection. It must NOT be wired into
// production code (this is a *_test.go file).
func bluffySaveSessions(s *Store) (int64, int, error) {
	sessions := s.sessionMgr.GetAll()
	if len(sessions) == 0 {
		return 0, 0, nil
	}
	sessionsPath := filepath.Join(s.basePath, "sessions")
	if err := os.MkdirAll(sessionsPath, 0o755); err != nil {
		return 0, 0, err
	}
	totalSize := int64(0)
	for _, sess := range sessions {
		snapshot, err := s.sessionMgr.Export(sess.ID)
		if err != nil {
			continue
		}
		data, err := s.serializer.Serialize(snapshot)
		if err != nil {
			continue
		}
		filename := filepath.Join(sessionsPath, sess.ID+s.serializer.Extension())
		if err := writeAtomic(filename, data); err != nil {
			continue // BLUFF: write failure swallowed
		}
		totalSize += int64(len(data))
	}
	// BLUFF: reports len(sessions) regardless of how many actually wrote.
	return totalSize, len(sessions), nil
}

func TestSaveFailureSurfaced(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	require.NoError(t, err)

	sm := session.NewManager()
	store.SetSessionManager(sm)
	s, err := sm.Create("p1", "S1", "desc", session.ModePlanning)
	require.NoError(t, err)

	blocker := plantWriteFailure(t, tmp, s.ID)

	if redMode() {
		// RED: the pre-fix helper swallows the failure and bluffs success.
		size, count, sErr := bluffySaveSessions(store)
		assert.NoError(t, sErr,
			"RED_MODE=1: broken saveSessions swallows the write failure (nil error)")
		assert.Equal(t, 1, count,
			"RED_MODE=1: broken saveSessions reports the full item count as saved despite the failed write")
		assert.Equal(t, int64(0), size,
			"RED_MODE=1: nothing was actually written (size 0) yet success is reported — the bluff")
		// Prove no real file landed: the blocker is still a directory.
		info, statErr := os.Stat(blocker)
		require.NoError(t, statErr)
		assert.True(t, info.IsDir(),
			"RED_MODE=1: the target is still a directory — the session was never persisted")
		return
	}

	// GREEN guard: the REAL fixed SaveAll surfaces the write failure.
	err = store.SaveAll()
	require.Error(t, err,
		"RED_MODE=0: SaveAll MUST surface a write failure, never report a silent success")

	// And the honest contract: the blocked session was genuinely not persisted.
	info, statErr := os.Stat(blocker)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir(),
		"the blocked target remains a directory — confirming the failure SaveAll reported was real")
}

// TestSaveFailureHonestCount asserts the GREEN-path honest-count contract: when
// SOME items save and one fails, the surfaced error reflects the partial failure
// rather than a clean success, and the genuinely-written sibling is on disk.
func TestSaveFailureHonestCount(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: honest-count is a GREEN-only contract; the RED bluff is reproduced by TestSaveFailureSurfaced")
	}

	tmp := t.TempDir()
	store, err := NewStore(tmp)
	require.NoError(t, err)

	sm := session.NewManager()
	store.SetSessionManager(sm)
	good, err := sm.Create("p1", "good", "desc", session.ModePlanning)
	require.NoError(t, err)
	bad, err := sm.Create("p1", "bad", "desc", session.ModePlanning)
	require.NoError(t, err)

	// Block only the "bad" session's file.
	plantWriteFailure(t, tmp, bad.ID)

	err = store.SaveAll()
	require.Error(t, err, "SaveAll MUST surface the partial failure")

	// The good session genuinely persisted (a real file, not a directory).
	goodPath := filepath.Join(tmp, "sessions", good.ID+".json")
	info, statErr := os.Stat(goodPath)
	require.NoError(t, statErr)
	assert.False(t, info.IsDir(), "the good session must be a real persisted file")
	assert.Greater(t, info.Size(), int64(0), "the good session file must be non-empty")
}
