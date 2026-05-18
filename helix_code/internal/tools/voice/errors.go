package voice

import (
	"errors"
	"fmt"
)

// Predefined errors for voice operations
var (
	// Device errors
	ErrNoDevicesFound    = errors.New("no audio input devices found")
	ErrDeviceNotFound    = errors.New("specified device not found")
	ErrDeviceUnavailable = errors.New("device is not available")
	ErrDeviceInUse       = errors.New("device is already in use")

	// Recording errors
	ErrAlreadyRecording   = errors.New("recording already in progress")
	ErrNotRecording       = errors.New("no recording in progress")
	ErrRecordingTimeout   = errors.New("recording exceeded maximum duration")
	ErrAudioCaptureFailed = errors.New("failed to capture audio")
	ErrInvalidFormat      = errors.New("invalid audio format")

	// Transcription errors
	ErrTranscriptionFailed = errors.New("transcription failed")
	ErrInvalidAPIKey       = errors.New("invalid or missing API key")
	ErrFileTooLarge        = errors.New("audio file exceeds size limit")
	ErrUnsupportedFormat   = errors.New("unsupported audio format")
	ErrNoSpeechDetected    = errors.New("no speech detected in audio")

	// File errors
	ErrFileNotFound    = errors.New("audio file not found")
	ErrFileReadFailed  = errors.New("failed to read audio file")
	ErrFileWriteFailed = errors.New("failed to write audio file")

	// ErrPlatformAudioCaptureNotWired surfaces the historical §11.4
	// PASS-bluff in AudioRecorder.recordRealAudio: when the device's
	// driver was not "Mock" (i.e. real-mode capture was requested),
	// the function silently fell back to recordMockAudio and
	// produced a 440 Hz sine wave instead of any platform-level
	// capture. Operators saw "recording succeeded" + a WAV file with
	// fabricated samples. Article XI §11.9 / CONST-035 / CONST-050(A).
	// The new recordRealAudio path flips a.recording=false and logs
	// this sentinel loudly so monitoring catches the missing
	// platform bridge (CoreAudio/ALSA/WASAPI not yet wired).
	ErrPlatformAudioCaptureNotWired = errors.New(
		"voice: platform audio capture (CoreAudio/ALSA/WASAPI) not wired — " +
			"set the device Driver to \"Mock\" for deterministic test samples, " +
			"or compile in a platform-specific capture bridge (§11.4 PASS-bluff removed)")
)

// VoiceError wraps errors with additional context
type VoiceError struct {
	Op      string // Operation that failed
	Err     error  // Underlying error
	Context string // Additional context
}

// Error implements the error interface
func (e *VoiceError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("%s: %v (%s)", e.Op, e.Err, e.Context)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *VoiceError) Unwrap() error {
	return e.Err
}

// NewVoiceError creates a new VoiceError
func NewVoiceError(op string, err error, context string) error {
	return &VoiceError{
		Op:      op,
		Err:     err,
		Context: context,
	}
}
