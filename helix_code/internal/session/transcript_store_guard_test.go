package session

// Standing regression guard (§11.4.135) for the world-readable session-state
// file modes in TranscriptStore. A session transcript carries the full
// conversation — user prompts and tool output that can embed API keys, tokens,
// and other credentials — so it is sensitive-data per CONST-042 / §12.1 and
// CONST-053 §4. The store used to write transcript.jsonl + metadata.json with
// mode 0644 and the session directory with 0755, letting any other local user
// read another user's session.
//
// §11.4.115 polarity: a single RED_MODE switch.
//   RED_MODE=1 — reproduce the defect on a FAITHFUL pre-fix stand-in: write a
//                file with the OLD 0644 mode and assert the world/group read
//                bits ARE present (the defect is observed).
//   RED_MODE=0 (default) — drive the REAL fixed TranscriptStore: append a
//                message, then assert the produced transcript, metadata, and
//                session directory are owner-only (no group/other bits).

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// worldOrGroupReadable reports whether any group/other permission bit is set.
func worldOrGroupReadable(m os.FileMode) bool {
	return m.Perm()&0o077 != 0
}

func TestTranscriptStore_FileModes_NotWorldReadable(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		// Faithful pre-fix stand-in: a file written with the OLD 0644 mode.
		dir := t.TempDir()
		p := filepath.Join(dir, "transcript.jsonl")
		if err := os.WriteFile(p, []byte("{}\n"), 0644); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if !worldOrGroupReadable(fi.Mode()) {
			t.Fatalf("RED_MODE: expected the pre-fix 0644 stand-in to be "+
				"group/other-readable, got %v — stand-in does not model the defect",
				fi.Mode().Perm())
		}
		t.Logf("RED_MODE: reproduced the defect — 0644 session file is "+
			"group/other-readable (mode=%v)", fi.Mode().Perm())
		return
	}

	// GREEN: the real fixed code path.
	dir := t.TempDir()
	s := NewTranscriptStore(dir)
	ctx := context.Background()

	if err := s.Append(ctx, "guard-sess", Message{
		Role:    "user",
		Content: "here is a token sk-SHOULD-NOT-LEAK-1234567890",
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Also exercise the UpdateSessionMetadata path directly.
	if err := s.UpdateSessionMetadata(ctx, SessionMetadata{
		SessionID:   "guard-sess",
		ProjectPath: "/some/project",
	}); err != nil {
		t.Fatalf("UpdateSessionMetadata: %v", err)
	}

	targets := []string{
		filepath.Join(dir, "guard-sess"),
		filepath.Join(dir, "guard-sess", "transcript.jsonl"),
		filepath.Join(dir, "guard-sess", "metadata.json"),
	}
	for _, p := range targets {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if worldOrGroupReadable(fi.Mode()) {
			t.Errorf("session-state path %s is group/other-accessible (mode=%v); "+
				"must be owner-only per CONST-042/CONST-053", filepath.Base(p), fi.Mode().Perm())
		}
	}
}
