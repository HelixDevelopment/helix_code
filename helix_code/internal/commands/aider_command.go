package commands

import (
	"context"
	"fmt"

	"dev.helix.code/internal/voice"
)

type AiderCommand struct {
	recorder    *voice.VoiceRecorder
	transcriber *voice.VoiceTranscriber
}

func NewAiderCommand(recorder *voice.VoiceRecorder, transcriber *voice.VoiceTranscriber) *AiderCommand {
	return &AiderCommand{recorder: recorder, transcriber: transcriber}
}

func (c *AiderCommand) Name() string      { return "aider" }
func (c *AiderCommand) Aliases() []string { return []string{"ai"} }
func (c *AiderCommand) Description() string {
	return tr(context.Background(), "internal_commands_aider_description", nil)
}
func (c *AiderCommand) Usage() string {
	return tr(context.Background(), "internal_commands_aider_usage", nil)
}

func (c *AiderCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	args := cmdCtx.Args
	if len(args) == 0 {
		return &CommandResult{
			Success: true,
			Message: tr(ctx, "internal_commands_aider_usage_full", nil),
		}, nil
	}

	subcmd := args[0]
	switch subcmd {
	case "voice":
		return c.handleVoice(ctx, cmdCtx, args[1:])
	case "repomap":
		return c.handleRepoMap(ctx, cmdCtx, args[1:])
	default:
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_aider_unknown_subcommand", map[string]any{"Subcommand": subcmd})}, nil
	}
}

func (c *AiderCommand) handleVoice(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	if len(args) == 0 {
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_aider_voice_usage", nil)}, nil
	}

	switch args[0] {
	case "start":
		path := "/tmp/helixcode_aider_recording.wav"
		if err := c.recorder.Start(path); err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("start recording: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_aider_recording_started", map[string]any{"Path": path})}, nil

	case "stop":
		if err := c.recorder.Stop(); err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("stop recording: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_aider_recording_stopped", map[string]any{
			"File": c.recorder.FilePath(), "Duration": c.recorder.Duration().String()})}, nil

	case "transcribe":
		path := c.recorder.FilePath()
		if path == "" {
			return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_aider_no_recording", nil)}, nil
		}
		result, err := c.transcriber.Transcribe(ctx, path)
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("transcribe: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_aider_transcribed", map[string]any{
			"Engine": result.Engine, "Text": result.Text})}, nil

	default:
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_aider_unknown_voice_subcommand", map[string]any{"Subcommand": args[0]})}, nil
	}
}

func (c *AiderCommand) handleRepoMap(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	return &CommandResult{
		Success: true,
		Message: tr(ctx, "internal_commands_aider_repomap_hint", nil),
	}, nil
}
