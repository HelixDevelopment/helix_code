package voice

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVoiceRecorder_StartStop(t *testing.T) {
	rec := NewVoiceRecorder()
	assert.Equal(t, RecorderIdle, rec.Status())
	assert.False(t, rec.IsRecording())

	path := filepath.Join(t.TempDir(), "test.wav")

	err := rec.Start(path)
	if err != nil {
		if err == ErrNoMicrophone {
			t.Skip("SKIP-OK: no microphone available")
		}
		t.Fatalf("Start: %v", err)
	}

	assert.True(t, rec.IsRecording())
	assert.Equal(t, RecorderRecording, rec.Status())
	assert.Equal(t, path, rec.FilePath())

	time.Sleep(500 * time.Millisecond)

	err = rec.Stop()
	require.NoError(t, err)

	assert.False(t, rec.IsRecording())
	assert.Equal(t, RecorderStopped, rec.Status())
	assert.Greater(t, rec.Duration(), time.Duration(0))
}

func TestVoiceRecorder_NotRecording(t *testing.T) {
	rec := NewVoiceRecorder()

	err := rec.Stop()
	assert.ErrorIs(t, err, ErrNotRecording)

	assert.False(t, rec.IsRecording())
	assert.Equal(t, time.Duration(0), rec.Duration())
}

func TestVoiceRecorder_AlreadyRecording(t *testing.T) {
	rec := NewVoiceRecorder()
	path := filepath.Join(t.TempDir(), "dbl.wav")

	err := rec.Start(path)
	if err == ErrNoMicrophone {
		t.Skip("SKIP-OK: no microphone")
		return
	}
	require.NoError(t, err)

	err = rec.Start(path)
	assert.ErrorIs(t, err, ErrAlreadyRecording)

	rec.Stop()
}

func TestValidateWAV(t *testing.T) {
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "empty.wav")
	os.WriteFile(emptyPath, make([]byte, 10), 0600)
	assert.ErrorIs(t, ValidateWAV(emptyPath), ErrEmptyRecording)

	bigPath := filepath.Join(dir, "big.wav")
	os.WriteFile(bigPath, make([]byte, 100), 0600)
	assert.NoError(t, ValidateWAV(bigPath))

	assert.Error(t, ValidateWAV("/nonexistent/path.wav"))
}

func TestVoiceTranscriber_NoAPIKey_Fallback(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "test.wav")
	os.WriteFile(audioPath, make([]byte, 100), 0600)

	trans := NewVoiceTranscriber(VoiceConfig{})
	_, err := trans.Transcribe(context.Background(), audioPath)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTranscribeFailed)
}

func TestTranscriptionResult(t *testing.T) {
	r := TranscriptionResult{
		Text:   "hello world",
		Engine: "whisper_api",
	}
	assert.Equal(t, "hello world", r.Text)
	assert.Equal(t, "whisper_api", r.Engine)
}

func TestSentinelErrors(t *testing.T) {
	assert.Error(t, ErrNoMicrophone)
	assert.Error(t, ErrNotRecording)
	assert.Error(t, ErrAlreadyRecording)
	assert.Error(t, ErrTranscribeFailed)
	assert.Error(t, ErrEmptyRecording)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "whisper-1", DefaultModel)
	assert.Equal(t, "https://api.openai.com/v1/audio/transcriptions", WhisperAPIURL)
	assert.Equal(t, TranscriberEngine("whisper_api"), EngineWhisperAPI)
	assert.Equal(t, TranscriberEngine("whisper_cpp"), EngineWhisperCPP)
}
