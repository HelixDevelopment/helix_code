package voice

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type VoiceRecorder struct {
	mu         sync.Mutex
	status     RecorderStatus
	cmd        *exec.Cmd
	captureCmd string
	filePath   string
	startedAt  time.Time
	stoppedAt  time.Time
}

func NewVoiceRecorder() *VoiceRecorder {
	return &VoiceRecorder{status: RecorderIdle}
}

// NewVoiceRecorderWithCmd builds a recorder that captures via an
// operator-supplied capture command instead of the auto-detected
// arecord/sox/parec. The command receives the destination WAV path as
// its final argument (whitespace-split). This wires the previously-dead
// VoiceConfig.CaptureCmd field so a configured backend is actually used.
func NewVoiceRecorderWithCmd(captureCmd string) *VoiceRecorder {
	return &VoiceRecorder{status: RecorderIdle, captureCmd: captureCmd}
}

func (r *VoiceRecorder) Start(outputPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status == RecorderRecording {
		return ErrAlreadyRecording
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	cmd, err := r.detectCaptureCmd(outputPath)
	if err != nil {
		return fmt.Errorf("detect capture: %w", err)
	}

	// HXC-VOICE-START: actually LAUNCH the capture process. The prior
	// implementation built *exec.Cmd but never called Start(), so
	// cmd.Process stayed nil, no audio was ever captured, and every
	// downstream voice_stop / voice_transcribe failed on an absent file
	// — a §11.4 PASS-bluff in production code (audio path, §11.4.72).
	if err := cmd.Start(); err != nil {
		// Leave status unchanged (Idle/Stopped) so the recorder is not
		// stuck reporting RecorderRecording for a process that never ran.
		return fmt.Errorf("start capture process: %w", err)
	}

	r.cmd = cmd
	r.filePath = outputPath
	r.status = RecorderRecording
	r.startedAt = time.Now()

	return nil
}

func (r *VoiceRecorder) detectCaptureCmd(outputPath string) (*exec.Cmd, error) {
	// Operator-configured capture command wins (wires the previously
	// unused VoiceConfig.CaptureCmd). The destination path is appended
	// as the final argument.
	if r.captureCmd != "" {
		fields := strings.Fields(r.captureCmd)
		if len(fields) == 0 {
			return nil, ErrNoMicrophone
		}
		if _, err := exec.LookPath(fields[0]); err != nil {
			return nil, fmt.Errorf("%w: configured capture command %q not found", ErrNoMicrophone, fields[0])
		}
		args := append(fields[1:], outputPath)
		return exec.Command(fields[0], args...), nil
	}
	if _, err := exec.LookPath("arecord"); err == nil {
		return exec.Command("arecord", "-f", "cd", "-t", "wav", outputPath), nil
	}
	if _, err := exec.LookPath("sox"); err == nil {
		return exec.Command("sox", "-d", "-r", "16000", "-c", "1", "-b", "16", outputPath), nil
	}
	if _, err := exec.LookPath("parec"); err == nil {
		return exec.Command("parec", "--format=s16le", "--rate=16000", "--channels=1", outputPath), nil
	}
	return nil, ErrNoMicrophone
}

func (r *VoiceRecorder) Stop() error {
	// HXC-VOICE-STOP-NOLOCK (§11.4.72 audio path): the SIGINT→2s→Kill reap
	// below can block for up to 2 seconds. Holding r.mu across it would
	// stall every concurrent IsRecording()/Status()/FilePath()/Duration()
	// for that whole window. Instead, under the lock we (a) reject a
	// non-recording recorder, (b) snapshot the cmd, and (c) write the
	// TERMINAL state (RecorderStopped + stoppedAt) immediately. Flipping
	// the status under the lock here is what makes the reap race-clean:
	//   - a concurrent second Stop() now sees status != RecorderRecording
	//     and returns ErrNotRecording, so r.cmd.Wait() is called EXACTLY
	//     once (a double Wait() would panic);
	//   - Status()/Duration()/etc. observe a consistent terminal state and
	//     never touch r.cmd while the reap goroutine does.
	r.mu.Lock()
	if r.status != RecorderRecording {
		r.mu.Unlock()
		return ErrNotRecording
	}
	cmd := r.cmd
	r.status = RecorderStopped
	r.stoppedAt = time.Now()
	r.mu.Unlock()

	// Reap WITHOUT holding r.mu. cmd is a local snapshot; only this
	// invocation reaches here for a given recording (the status flip above
	// fenced off any concurrent Stop()), so the access is exclusive. A nil
	// Process (Start failed / never started) is a clean no-op.
	if cmd != nil && cmd.Process != nil {
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			_ = cmd.Process.Kill()
		}
		// HXC-VOICE-WAIT: reap the capture process. Without Wait() the
		// finished child becomes a zombie and the goroutines/file
		// descriptors os/exec created for it leak — a §11.4.85
		// resource-exhaustion defect under repeated start/stop cycles.
		// The capture tool exits on SIGINT; if it ignores the signal we
		// escalate to Kill so Wait() always returns promptly.
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}

	return nil
}

func (r *VoiceRecorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status == RecorderRecording
}

func (r *VoiceRecorder) Status() RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *VoiceRecorder) FilePath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.filePath
}

func (r *VoiceRecorder) Duration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == RecorderStopped {
		return r.stoppedAt.Sub(r.startedAt)
	}
	if r.status == RecorderRecording {
		return time.Since(r.startedAt)
	}
	return 0
}

func ValidateWAV(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat wav: %w", err)
	}
	if info.Size() <= 44 {
		return ErrEmptyRecording
	}
	return nil
}
