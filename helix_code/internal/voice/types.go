package voice

import (
	"errors"
	"time"
)

type RecorderStatus int

const (
	RecorderIdle    RecorderStatus = iota
	RecorderRecording
	RecorderStopped
)

type TranscriberEngine string

const (
	EngineWhisperAPI TranscriberEngine = "whisper_api"
	EngineWhisperCPP TranscriberEngine = "whisper_cpp"
)

type TranscriptionResult struct {
	Text     string        `json:"text"`
	Duration time.Duration `json:"duration_ms"`
	Engine   string        `json:"engine"`
}

type VoiceConfig struct {
	WhisperAPIKey string
	WhisperModel  string
	CaptureCmd    string
}

var (
	ErrNoMicrophone     = errors.New("no microphone available")
	ErrNotRecording     = errors.New("recorder is not recording")
	ErrAlreadyRecording = errors.New("recorder is already recording")
	ErrTranscribeFailed = errors.New("transcription failed")
	ErrEmptyRecording   = errors.New("recording is empty (0 bytes)")
)

const (
	DefaultModel      = "whisper-1"
	DefaultCaptureCmd = "arecord"
	WhisperAPIURL     = "https://api.openai.com/v1/audio/transcriptions"
)
