package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type VoiceTranscriber struct {
	config VoiceConfig
	client *http.Client
}

func NewVoiceTranscriber(config VoiceConfig) *VoiceTranscriber {
	return &VoiceTranscriber{
		config: config,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (t *VoiceTranscriber) Transcribe(ctx context.Context, audioPath string) (TranscriptionResult, error) {
	if err := ValidateWAV(audioPath); err != nil {
		return TranscriptionResult{}, err
	}

	if t.config.WhisperAPIKey != "" {
		return t.transcribeWhisperAPI(ctx, audioPath)
	}

	return t.transcribeWhisperCPP(ctx, audioPath)
}

func (t *VoiceTranscriber) transcribeWhisperAPI(ctx context.Context, audioPath string) (TranscriptionResult, error) {
	file, err := os.Open(audioPath)
	if err != nil {
		return t.transcribeWhisperCPP(ctx, audioPath)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return t.transcribeWhisperCPP(ctx, audioPath)
	}
	if _, err := io.Copy(part, file); err != nil {
		return t.transcribeWhisperCPP(ctx, audioPath)
	}

	model := t.config.WhisperModel
	if model == "" {
		model = DefaultModel
	}
	writer.WriteField("model", model)
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", WhisperAPIURL, body)
	if err != nil {
		return t.transcribeWhisperCPP(ctx, audioPath)
	}
	req.Header.Set("Authorization", "Bearer "+t.config.WhisperAPIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := t.client.Do(req)
	if err != nil {
		return t.transcribeWhisperCPP(ctx, audioPath)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return t.transcribeWhisperCPP(ctx, audioPath)
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return t.transcribeWhisperCPP(ctx, audioPath)
	}

	if result.Text == "" {
		return t.transcribeWhisperCPP(ctx, audioPath)
	}

	return TranscriptionResult{
		Text:   result.Text,
		Engine: string(EngineWhisperAPI),
	}, nil
}

func (t *VoiceTranscriber) transcribeWhisperCPP(ctx context.Context, audioPath string) (TranscriptionResult, error) {
	modelPath := t.config.WhisperModel
	if modelPath == "" {
		modelPath = "ggml-base.en.bin"
	}

	if _, err := exec.LookPath("whisper-cli"); err != nil {
		if _, err2 := exec.LookPath("main"); err2 != nil {
			return TranscriptionResult{}, fmt.Errorf("whisper.cpp not found: %w", ErrTranscribeFailed)
		}
	}

	cmd := exec.CommandContext(ctx, "whisper-cli",
		"-m", modelPath,
		"-f", audioPath,
		"--no-timestamps",
		"-otxt",
	)
	out, err := cmd.Output()
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("whisper-cli: %w: %s", ErrTranscribeFailed, string(out))
	}

	text := string(bytes.TrimSpace(out))
	if text == "" {
		return TranscriptionResult{}, ErrTranscribeFailed
	}

	return TranscriptionResult{
		Text:   text,
		Engine: string(EngineWhisperCPP),
	}, nil
}
