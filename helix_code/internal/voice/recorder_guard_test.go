package voice

// Standing regression guard (§11.4.135) for HXC-VOICE-START /
// HXC-VOICE-WAIT.
//
// DEFECT (FACT, source-provable): VoiceRecorder.Start() built an
// *exec.Cmd via detectCaptureCmd but never called cmd.Start(), so the
// OS capture process was never launched. The recorder reported
// RecorderRecording while capturing nothing — every downstream
// voice_stop / voice_transcribe then failed because the WAV file was
// never created (ValidateWAV → ErrEmptyRecording / stat error). The
// designed-but-dead VoiceConfig.CaptureCmd was also never wired in.
// Audio path → §11.4.72 top-priority.
//
// §11.4.115 polarity switch (RED_MODE env):
//   RED_MODE=1 → reproduce the defect on a faithful pre-fix stand-in
//               (build the cmd, set status=Recording, but DO NOT Start
//               it) and ASSERT the defect is present (no live process,
//               no file). PASSES on the broken behaviour — proving the
//               guard genuinely catches the bug.
//   RED_MODE=0 (default) → drive the REAL fixed VoiceRecorder against a
//               real capture subprocess and ASSERT the defect is ABSENT
//               (process actually launched, WAV file created + non-empty
//               after Stop()).
//
// Uses a real `sh -c` capture command (no mock — this is anti-bluff
// runtime evidence per §11.4 / CONST-050(A) permits mocks only in unit
// tests, and this drives a REAL subprocess regardless).

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// redMode reports whether the RED reproduction polarity is active.
func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// writerCaptureCmd writes a small executable capture-stand-in script to
// a temp dir and returns its absolute path (suitable for
// NewVoiceRecorderWithCmd). When launched with the destination WAV path
// as its argument, the script writes a valid >44-byte file then sleeps
// (staying alive until Stop() signals + reaps it) — a faithful REAL
// subprocess stand-in for arecord/sox writing the destination file.
// NewVoiceRecorderWithCmd splits on whitespace, so a single bare path
// (no spaces, no shell quoting) is used to survive that split.
func writerCaptureCmd(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("SKIP-OK: /bin/sh unavailable on this host — §11.4.3: %v", err)
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "capture_writer.sh")
	// $1 = destination path appended by NewVoiceRecorderWithCmd.
	// head -c 64 → 64-byte file (>44 so ValidateWAV passes); sleep keeps
	// the process alive so Stop() must genuinely signal + Wait()-reap it.
	body := "#!/bin/sh\nhead -c 64 /dev/zero > \"$1\"\nsleep 30\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write capture stand-in script: %v", err)
	}
	return script
}

func TestVoiceRecorder_StartLaunchesProcess_Guard(t *testing.T) {
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "guard.wav")

	if redMode() {
		// RED: faithful pre-fix stand-in. Reproduce the exact broken
		// behaviour: build the command but DO NOT Start() it.
		rec := &VoiceRecorder{status: RecorderIdle, captureCmd: writerCaptureCmd(t)}
		cmd, err := rec.detectCaptureCmd(outPath)
		if err != nil {
			t.Skipf("SKIP-OK: no capture backend on this host — §11.4.3: %v", err)
		}
		// Mirror the PRE-FIX Start(): assign cmd + flip status, but never
		// call cmd.Start().
		rec.mu.Lock()
		rec.cmd = cmd
		rec.filePath = outPath
		rec.status = RecorderRecording
		rec.mu.Unlock()

		// DEFECT ASSERTION 1: no process was ever launched.
		rec.mu.Lock()
		proc := rec.cmd.Process
		rec.mu.Unlock()
		if proc != nil {
			t.Fatalf("RED expected cmd.Process==nil (process never launched), got pid=%d", proc.Pid)
		}

		// DEFECT ASSERTION 2: the capture file was never created, so the
		// downstream ValidateWAV the voice_stop tool performs fails.
		time.Sleep(200 * time.Millisecond)
		if _, statErr := os.Stat(outPath); statErr == nil {
			t.Fatalf("RED expected NO capture file (process never ran), but %s exists", outPath)
		}
		if vErr := ValidateWAV(outPath); vErr == nil {
			t.Fatalf("RED expected ValidateWAV to fail on absent file, got nil")
		}
		t.Logf("RED reproduced: Start() stand-in set status=Recording but launched no process and wrote no file")
		return
	}

	// GREEN: drive the REAL fixed recorder end-to-end.
	rec := NewVoiceRecorderWithCmd(writerCaptureCmd(t))
	if err := rec.Start(outPath); err != nil {
		t.Fatalf("Start() failed on real fixed recorder: %v", err)
	}

	// The capture process MUST have actually been launched.
	rec.mu.Lock()
	proc := rec.cmd.Process
	rec.mu.Unlock()
	if proc == nil {
		t.Fatalf("GREEN: Start() reported Recording but cmd.Process is nil — defect NOT fixed")
	}

	// Give the real subprocess a moment to write the destination file.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(outPath); err == nil && info.Size() > 44 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	// ABSENCE-OF-DEFECT: a real, non-empty capture file exists — the
	// exact thing the broken code never produced.
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("GREEN: capture file missing after Start/Stop — defect present: %v", err)
	}
	if info.Size() <= 44 {
		t.Fatalf("GREEN: capture file too small (%d bytes) — process did not write — defect present", info.Size())
	}
	if err := ValidateWAV(outPath); err != nil {
		t.Fatalf("GREEN: ValidateWAV failed on real captured file — defect present: %v", err)
	}
	if rec.Status() != RecorderStopped {
		t.Fatalf("GREEN: expected RecorderStopped after Stop(), got %v", rec.Status())
	}
}

// TestVoiceRecorder_StopReapsProcess_Guard proves HXC-VOICE-WAIT: after
// Stop(), the launched capture process is reaped (no zombie / no leaked
// wait). GREEN-only (the pre-fix code never launched a process, so there
// was nothing to reap — the RED reproduction lives in the test above).
func TestVoiceRecorder_StopReapsProcess_Guard(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: reap behaviour only meaningful against the fixed code (pre-fix launched no process)")
	}
	outPath := filepath.Join(t.TempDir(), "reap.wav")
	rec := NewVoiceRecorderWithCmd(writerCaptureCmd(t))
	if err := rec.Start(outPath); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	rec.mu.Lock()
	cmd := rec.cmd
	rec.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		t.Fatalf("Start() reported Recording but no process was launched — defect present (HXC-VOICE-START regressed)")
	}
	pid := cmd.Process.Pid

	start := time.Now()
	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
	// Stop() must return promptly (well under the 2s escalation budget)
	// because the writer exits on SIGINT.
	if elapsed := time.Since(start); elapsed > 2500*time.Millisecond {
		t.Fatalf("Stop() took %v — Wait()/Kill escalation did not reap the process promptly", elapsed)
	}
	// After Wait(), ProcessState is populated — proof the child was reaped
	// (not left a zombie). A nil ProcessState means Wait() never ran.
	if cmd.ProcessState == nil {
		t.Fatalf("ProcessState nil after Stop() — process pid=%d was NOT reaped (zombie/leak)", pid)
	}
}

// TestVoiceRecorder_StatusNonBlockingDuringStop_Guard proves MUST-FIX 1
// (§11.4.72 audio path): Stop() must NOT hold r.mu across its blocking
// SIGINT→2s→Kill reap, or a concurrent Status()/IsRecording()/FilePath()/
// Duration() would stall for up to ~2s.
//
// We drive a REAL capture subprocess that deliberately IGNORES SIGINT and
// only dies on SIGKILL — this forces Stop() down its full 2-second
// escalation path, which is exactly the window an observer would be
// blocked for if Stop() held the lock. While Stop() runs, we hammer
// Status()/IsRecording()/FilePath()/Duration() from a goroutine and assert
// every single call returns in well under 250ms. If the lock were held
// across the reap, these calls would block ~2s and the guard FAILs.
//
// Paired §1.1 mutation: re-acquiring r.mu around the reap block in Stop()
// (the pre-fix lock-held-across-wait shape) makes these observer calls
// block ~2s → this guard FAILs. Restoring the snapshot-and-release shape →
// guard PASSes.
func TestVoiceRecorder_StatusNonBlockingDuringStop_Guard(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: lock-contention behaviour only meaningful against the fixed Stop() (pre-fix launched no process to reap)")
	}
	perlPath, err := exec.LookPath("perl")
	if err != nil {
		// §11.4.3: a reliably-SIGINT-ignoring capture stand-in is needed to
		// force Stop() down its full 2s reap window. Plain /bin/sh `trap ''
		// INT` is not honoured uniformly across platforms (observed on
		// macOS /bin/sh), so we require perl's deterministic SIG IGNORE.
		t.Skipf("SKIP-OK: perl unavailable — cannot build a deterministic SIGINT-ignoring capture stand-in on this host — §11.4.3: %v", err)
	}

	// Capture stand-in that IGNORES INT/TERM and only dies on KILL, writing
	// a valid >44-byte file first. perl's $SIG{INT}='IGNORE' is honoured
	// deterministically across platforms (unlike /bin/sh `trap ''`), so
	// Stop() is FORCED through its full SIGINT→2s-timeout→Kill escalation —
	// maximising the reap window we are proving the lock is NOT held across.
	// $ARGV[0] = the destination path appended by NewVoiceRecorderWithCmd.
	dir := t.TempDir()
	script := filepath.Join(dir, "sigint_ignoring_capture.pl")
	body := "#!/usr/bin/env perl\n" +
		"$SIG{INT}='IGNORE'; $SIG{TERM}='IGNORE';\n" +
		"open(my $fh, '>', $ARGV[0]) or die \"open: $!\";\n" +
		"print $fh ('\\0' x 64);\n" +
		"close($fh);\n" +
		"sleep 60;\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write sigint-ignoring capture stand-in: %v", err)
	}
	// NewVoiceRecorderWithCmd whitespace-splits the capture command and
	// LookPath-validates field[0]. Pass "perl <script>" so field[0]=perl
	// (on PATH) and the script path is its arg; the outPath is appended last
	// → argv = [perl, script, outPath] → $ARGV[0] = outPath.
	captureCmd := perlPath + " " + script

	outPath := filepath.Join(t.TempDir(), "nonblock.wav")
	rec := NewVoiceRecorderWithCmd(captureCmd)
	if err := rec.Start(outPath); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Observer: continuously poke the lock-guarded read methods while Stop()
	// is in its reap window; record the worst-case latency of any single
	// call.
	stopObserver := make(chan struct{})
	worst := make(chan time.Duration, 1)
	go func() {
		var maxLatency time.Duration
		for {
			select {
			case <-stopObserver:
				worst <- maxLatency
				return
			default:
			}
			t0 := time.Now()
			_ = rec.Status()
			_ = rec.IsRecording()
			_ = rec.FilePath()
			_ = rec.Duration()
			if d := time.Since(t0); d > maxLatency {
				maxLatency = d
			}
		}
	}()

	// Let the observer establish a baseline + ensure Start fully settled.
	time.Sleep(50 * time.Millisecond)

	stopStart := time.Now()
	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
	stopElapsed := time.Since(stopStart)

	close(stopObserver)
	maxObserverLatency := <-worst

	// Sanity: Stop() really did exercise the slow escalation path (the
	// subprocess ignored SIGINT), so the lock-hold window we are testing
	// against was genuinely ~2s wide. If Stop() returned almost instantly
	// the test would not be probing the dangerous window.
	if stopElapsed < 1500*time.Millisecond {
		t.Fatalf("Stop() returned in %v — expected ~2s escalation (subprocess should have ignored SIGINT); test is not exercising the lock-hold window", stopElapsed)
	}

	// The actual assertion: no observer call stalled. If Stop() held r.mu
	// across the ~2s reap, at least one Status()/etc. call would have been
	// blocked for ~2s.
	if maxObserverLatency > 250*time.Millisecond {
		t.Fatalf("concurrent Status()/IsRecording()/FilePath()/Duration() stalled %v during Stop() — Stop() is holding r.mu across the blocking reap (MUST-FIX 1 regressed)", maxObserverLatency)
	}
	t.Logf("Stop() took %v (full escalation), worst concurrent observer latency %v (<250ms) — lock released across reap", stopElapsed, maxObserverLatency)
}
