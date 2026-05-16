package voice

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type VoiceRecorder struct {
	mu       sync.Mutex
	status   RecorderStatus
	cmd      *exec.Cmd
	filePath string
	startedAt time.Time
	stoppedAt time.Time
}

func NewVoiceRecorder() *VoiceRecorder {
	return &VoiceRecorder{status: RecorderIdle}
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

	r.cmd = cmd
	r.filePath = outputPath
	r.status = RecorderRecording
	r.startedAt = time.Now()

	return nil
}

func (r *VoiceRecorder) detectCaptureCmd(outputPath string) (*exec.Cmd, error) {
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
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status != RecorderRecording {
		return ErrNotRecording
	}

	if r.cmd != nil && r.cmd.Process != nil {
		if err := r.cmd.Process.Signal(os.Interrupt); err != nil {
			r.cmd.Process.Kill()
		}
	}

	r.status = RecorderStopped
	r.stoppedAt = time.Now()

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
