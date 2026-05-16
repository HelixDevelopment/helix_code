package voice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

type VoiceStartTool struct {
	recorder *VoiceRecorder
	approval.DefaultLevelEdit
}

func NewVoiceStartTool(recorder *VoiceRecorder) *VoiceStartTool {
	return &VoiceStartTool{recorder: recorder}
}

func (t *VoiceStartTool) Name() string        { return "voice_start" }
func (t *VoiceStartTool) Description() string { return "Start recording audio from microphone" }
func (t *VoiceStartTool) Category() tools.ToolCategory {
	return tools.ToolCategory("voice")
}
func (t *VoiceStartTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *VoiceStartTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:       "object",
		Properties: map[string]interface{}{},
		Required:   []string{},
	}
}

func (t *VoiceStartTool) Validate(params map[string]interface{}) error { return nil }

func (t *VoiceStartTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	tmpDir := os.TempDir()
	outputPath := filepath.Join(tmpDir, "helixcode_voice_"+fmt.Sprintf("%d", os.Getpid())+".wav")

	if err := t.recorder.Start(outputPath); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":      "recording",
		"output_path": outputPath,
	}, nil
}

type VoiceStopTool struct {
	recorder *VoiceRecorder
	approval.DefaultLevelEdit
}

func NewVoiceStopTool(recorder *VoiceRecorder) *VoiceStopTool {
	return &VoiceStopTool{recorder: recorder}
}

func (t *VoiceStopTool) Name() string        { return "voice_stop" }
func (t *VoiceStopTool) Description() string { return "Stop recording and save audio file" }
func (t *VoiceStopTool) Category() tools.ToolCategory {
	return tools.ToolCategory("voice")
}
func (t *VoiceStopTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *VoiceStopTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:       "object",
		Properties: map[string]interface{}{},
		Required:   []string{},
	}
}

func (t *VoiceStopTool) Validate(params map[string]interface{}) error { return nil }

func (t *VoiceStopTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.recorder.Stop(); err != nil {
		return nil, err
	}

	path := t.recorder.FilePath()
	duration := t.recorder.Duration()

	if err := ValidateWAV(path); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":    "stopped",
		"file_path": path,
		"duration":  duration.String(),
	}, nil
}

type VoiceTranscribeTool struct {
	recorder     *VoiceRecorder
	transcriber  *VoiceTranscriber
	approval.DefaultLevelEdit
}

func NewVoiceTranscribeTool(recorder *VoiceRecorder, transcriber *VoiceTranscriber) *VoiceTranscribeTool {
	return &VoiceTranscribeTool{recorder: recorder, transcriber: transcriber}
}

func (t *VoiceTranscribeTool) Name() string        { return "voice_transcribe" }
func (t *VoiceTranscribeTool) Description() string { return "Transcribe recorded audio to text" }
func (t *VoiceTranscribeTool) Category() tools.ToolCategory {
	return tools.ToolCategory("voice")
}

func (t *VoiceTranscribeTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:       "object",
		Properties: map[string]interface{}{},
		Required:   []string{},
	}
}

func (t *VoiceTranscribeTool) Validate(params map[string]interface{}) error { return nil }

func (t *VoiceTranscribeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	path := t.recorder.FilePath()
	if path == "" {
		return nil, ErrNotRecording
	}

	result, err := t.transcriber.Transcribe(ctx, path)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"text":   result.Text,
		"engine": result.Engine,
	}, nil
}
