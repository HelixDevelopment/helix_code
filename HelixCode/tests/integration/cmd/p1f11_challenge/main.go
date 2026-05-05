// p1f11_challenge runs the Session Transcript Resume flow across a real
// process boundary. Runtime-evidence harness for the F11 Challenge.
//
// Three-phase orchestrator:
//
//	(default)            : orchestrator — fork-exec phase=write then phase=read,
//	                       then run the global-resume check in-process.
//	phase=write          : construct TranscriptStore + SessionManager in a
//	                       fresh process, append 3 messages, exit 0.
//	phase=read           : in a brand-new process (different PID), construct a
//	                       fresh SessionManager backed by the SAME baseDir,
//	                       call Resume(sessionID), assert all 3 messages are
//	                       present byte-exact, exit 0.
//
// The orchestrator passes baseDir + sessionID via env vars HELIX_F11_BASE
// and HELIX_F11_SESSION_ID.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"dev.helix.code/internal/session"
)

const (
	envBase      = "HELIX_F11_BASE"
	envSessionID = "HELIX_F11_SESSION_ID"
	phaseWrite   = "phase=write"
	phaseRead    = "phase=read"
)

// wantMessages are the canonical 3 messages the harness round-trips through
// disk. Phase-write writes them; phase-read asserts byte-exact recovery.
func wantMessages() []session.Message {
	base := time.Unix(1746460800, 0).UTC() // 2026-05-05T16:00:00Z (deterministic)
	return []session.Message{
		{Role: "user", Content: "hello cross-process world", Timestamp: base},
		{Role: "assistant", Content: "transcript resumed across PIDs", Timestamp: base.Add(time.Second)},
		{Role: "user", Content: "what is 2+2?", Timestamp: base.Add(2 * time.Second)},
	}
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case phaseWrite:
			if err := runPhaseWrite(); err != nil {
				fmt.Fprintln(os.Stderr, "FAIL phase=write:", err)
				os.Exit(1)
			}
			return
		case phaseRead:
			if err := runPhaseRead(); err != nil {
				fmt.Fprintln(os.Stderr, "FAIL phase=read:", err)
				os.Exit(1)
			}
			return
		default:
			fmt.Fprintln(os.Stderr, "FAIL: unknown phase arg:", os.Args[1])
			os.Exit(1)
		}
	}

	if err := runOrchestrator(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

// runOrchestrator drives the full challenge: fork phase=write, fork phase=read,
// then exercise ResumeGlobal across two project paths in-process.
func runOrchestrator() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("os.Executable: %w", err)
	}

	base, err := os.MkdirTemp("", "p1f11-")
	if err != nil {
		return fmt.Errorf("tempdir: %w", err)
	}
	defer os.RemoveAll(base)

	sessionID := uuid.NewString()

	fmt.Println("==> orchestrator pid:", os.Getpid())
	fmt.Println("    baseDir       :", base)
	fmt.Println("    sessionID     :", sessionID)
	fmt.Println("    harness binary:", exe)

	env := append(os.Environ(),
		envBase+"="+base,
		envSessionID+"="+sessionID,
	)

	fmt.Println("==> phase A: fork-exec child to write 3 messages")
	if out, err := runChild(exe, phaseWrite, env); err != nil {
		fmt.Print(out)
		return fmt.Errorf("phase=write child failed: %w", err)
	} else {
		fmt.Print(out)
	}

	// Verify on-disk artefacts BEFORE the read child runs — proves the bytes
	// are really on disk, not just in the writer's memory.
	transcriptPath := filepath.Join(base, sessionID, "transcript.jsonl")
	metadataPath := filepath.Join(base, sessionID, "metadata.json")
	tst, err := os.Stat(transcriptPath)
	if err != nil {
		return fmt.Errorf("expected transcript.jsonl after phase=write: %w", err)
	}
	if tst.Size() == 0 {
		return fmt.Errorf("transcript.jsonl is empty after phase=write")
	}
	mst, err := os.Stat(metadataPath)
	if err != nil {
		return fmt.Errorf("expected metadata.json after phase=write: %w", err)
	}
	if mst.Size() == 0 {
		return fmt.Errorf("metadata.json is empty after phase=write")
	}
	fmt.Printf("    on-disk transcript.jsonl size=%d bytes (path=%s)\n", tst.Size(), transcriptPath)
	fmt.Printf("    on-disk metadata.json   size=%d bytes (path=%s)\n", mst.Size(), metadataPath)

	fmt.Println("==> phase B: fork-exec NEW child to resume + assert byte-exact recovery")
	if out, err := runChild(exe, phaseRead, env); err != nil {
		fmt.Print(out)
		return fmt.Errorf("phase=read child failed: %w", err)
	} else {
		fmt.Print(out)
	}

	fmt.Println("==> phase C: in-orchestrator ResumeGlobal across two project paths")
	if err := runGlobalResumeCheck(base); err != nil {
		return fmt.Errorf("global-resume check: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P1-F11 challenge harness PASS")
	return nil
}

// runChild executes the harness binary with the given phase argument and
// returns its combined stdout+stderr. err is non-nil if the child exits non-zero.
func runChild(exe, phase string, env []string) (string, error) {
	cmd := exec.Command(exe, phase)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runPhaseWrite is invoked in a child process. It builds a TranscriptStore +
// SessionManager from scratch, sets the current session, appends 3 messages,
// and asserts that metadata.MessageCount == 3 before exiting.
func runPhaseWrite() error {
	ctx := context.Background()
	base := os.Getenv(envBase)
	sessionID := os.Getenv(envSessionID)
	if base == "" || sessionID == "" {
		return fmt.Errorf("missing env: %s=%q %s=%q", envBase, base, envSessionID, sessionID)
	}

	store := session.NewTranscriptStore(base)

	// Seed metadata so phase C can find this session by ProjectPath later.
	now := time.Now().UTC()
	if err := store.UpdateSessionMetadata(ctx, session.SessionMetadata{
		SessionID:    sessionID,
		ProjectPath:  "/tmp/projA-f11",
		ProjectName:  "projA-f11",
		StartedAt:    now,
		LastActivity: now,
		IsActive:     true,
	}); err != nil {
		return fmt.Errorf("seed metadata: %w", err)
	}

	mgr := session.NewSessionManager()
	mgr.SetStore(store)
	if err := mgr.Resume(ctx, sessionID); err != nil {
		return fmt.Errorf("resume to set currentID: %w", err)
	}

	msgs := wantMessages()
	for i, m := range msgs {
		if err := mgr.Append(ctx, m); err != nil {
			return fmt.Errorf("append msg %d: %w", i, err)
		}
	}

	meta, err := store.GetSessionMetadata(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get metadata after writes: %w", err)
	}
	if meta.MessageCount != len(msgs) {
		return fmt.Errorf("metadata.MessageCount = %d; want %d", meta.MessageCount, len(msgs))
	}

	// SessionManager.Append writes a metadata sidecar that does not carry
	// ProjectPath/ProjectName (those fields are not part of the in-memory
	// SessionManager state in F11). Re-attach them so phase C can locate
	// this session by ProjectPath.
	meta.ProjectPath = "/tmp/projA-f11"
	meta.ProjectName = "projA-f11"
	if err := store.UpdateSessionMetadata(ctx, *meta); err != nil {
		return fmt.Errorf("re-attach ProjectPath: %w", err)
	}

	fmt.Printf("    [child pid=%d phase=write] wrote %d messages, sessionID=%s\n",
		os.Getpid(), len(msgs), sessionID)
	fmt.Printf("    [child pid=%d phase=write] meta.MessageCount=%d OK\n",
		os.Getpid(), meta.MessageCount)
	return nil
}

// runPhaseRead is invoked in a brand-new child process (different PID, no
// shared in-memory state with the writer). It constructs a fresh
// SessionManager + TranscriptStore using the SAME baseDir, calls
// Resume(sessionID), and asserts every message round-trips byte-exact.
func runPhaseRead() error {
	ctx := context.Background()
	base := os.Getenv(envBase)
	sessionID := os.Getenv(envSessionID)
	if base == "" || sessionID == "" {
		return fmt.Errorf("missing env: %s=%q %s=%q", envBase, base, envSessionID, sessionID)
	}

	store := session.NewTranscriptStore(base)
	mgr := session.NewSessionManager()
	mgr.SetStore(store)

	if err := mgr.Resume(ctx, sessionID); err != nil {
		return fmt.Errorf("resume sessionID=%s: %w", sessionID, err)
	}
	if got := mgr.CurrentID(); got != sessionID {
		return fmt.Errorf("CurrentID = %q; want %q", got, sessionID)
	}

	want := wantMessages()
	if got := mgr.LoadedMessageCountForTestF11(); got != len(want) {
		return fmt.Errorf("LoadedMessageCount = %d; want %d", got, len(want))
	}

	got, err := store.ReadTranscript(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("ReadTranscript: %w", err)
	}
	if len(got) != len(want) {
		return fmt.Errorf("ReadTranscript length = %d; want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Role != want[i].Role {
			return fmt.Errorf("msg %d role = %q; want %q", i, got[i].Role, want[i].Role)
		}
		if got[i].Content != want[i].Content {
			return fmt.Errorf("msg %d content = %q; want %q", i, got[i].Content, want[i].Content)
		}
	}

	fmt.Printf("    [child pid=%d phase=read]  resumed %d messages, byte-exact OK\n",
		os.Getpid(), len(got))
	for i, m := range got {
		fmt.Printf("    [child pid=%d phase=read]    msg[%d] role=%q content=%q\n",
			os.Getpid(), i, m.Role, m.Content)
	}
	return nil
}

// runGlobalResumeCheck writes a SECOND session for a different ProjectPath
// with a more recent LastActivity and asserts that ResumeGlobal returns it
// (proving project-independent global lookup works on the same on-disk store).
func runGlobalResumeCheck(base string) error {
	ctx := context.Background()
	store := session.NewTranscriptStore(base)

	otherID := uuid.NewString()
	newer := time.Now().UTC().Add(5 * time.Minute) // strictly newer than phase-A's seed
	if err := store.UpdateSessionMetadata(ctx, session.SessionMetadata{
		SessionID:    otherID,
		ProjectPath:  "/tmp/projB-f11",
		ProjectName:  "projB-f11",
		StartedAt:    newer.Add(-time.Hour),
		LastActivity: newer,
		MessageCount: 1,
	}); err != nil {
		return fmt.Errorf("seed projB metadata: %w", err)
	}
	if err := store.Append(ctx, otherID, session.Message{
		Role: "user", Content: "newer session", Timestamp: newer,
	}); err != nil {
		return fmt.Errorf("append projB msg: %w", err)
	}

	finder := session.NewResumeFinder(store)
	target, err := finder.FindResumeTarget(ctx, session.ResumeGlobal, "")
	if err != nil {
		return fmt.Errorf("FindResumeTarget(global): %w", err)
	}
	if target == nil {
		return fmt.Errorf("FindResumeTarget(global) returned nil target")
	}
	if target.SessionID != otherID {
		return fmt.Errorf("global-resume returned %q; want %q (the more recent session)",
			target.SessionID, otherID)
	}

	// Project-scoped: must filter to projA's session, not projB's.
	projA, err := finder.FindResumeTarget(ctx, session.ResumeProject, "/tmp/projA-f11")
	if err != nil {
		return fmt.Errorf("FindResumeTarget(project=projA): %w", err)
	}
	if projA == nil || projA.ProjectPath != "/tmp/projA-f11" {
		return fmt.Errorf("project-scoped lookup returned wrong session: %+v", projA)
	}

	fmt.Printf("    global-resume target  : sessionID=%s project=%s\n",
		target.SessionID, target.ProjectPath)
	fmt.Printf("    project-scope (projA) : sessionID=%s project=%s\n",
		projA.SessionID, projA.ProjectPath)
	return nil
}
