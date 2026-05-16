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
	return "Aider integration: voice input and repo-map control"
}
func (c *AiderCommand) Usage() string {
	return "/aider [voice|repomap] [start|stop|transcribe|show]"
}

func (c *AiderCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	args := cmdCtx.Args
	if len(args) == 0 {
		return &CommandResult{
			Success: true,
			Message: "/aider voice [start|stop|transcribe] — voice input\n/aider repomap show — repository map",
		}, nil
	}

	subcmd := args[0]
	switch subcmd {
	case "voice":
		return c.handleVoice(ctx, cmdCtx, args[1:])
	case "repomap":
		return c.handleRepoMap(ctx, cmdCtx, args[1:])
	default:
		return &CommandResult{Success: false, Message: fmt.Sprintf("unknown: %s. Use 'voice' or 'repomap'", subcmd)}, nil
	}
}

func (c *AiderCommand) handleVoice(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	if len(args) == 0 {
		return &CommandResult{Success: false, Message: "usage: /aider voice [start|stop|transcribe]"}, nil
	}

	switch args[0] {
	case "start":
		path := "/tmp/helixcode_aider_recording.wav"
		if err := c.recorder.Start(path); err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("start recording: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: fmt.Sprintf("Recording started → %s", path)}, nil

	case "stop":
		if err := c.recorder.Stop(); err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("stop recording: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: fmt.Sprintf("Recording stopped. File: %s, Duration: %s",
			c.recorder.FilePath(), c.recorder.Duration().String())}, nil

	case "transcribe":
		path := c.recorder.FilePath()
		if path == "" {
			return &CommandResult{Success: false, Message: "No recording available. Use /aider voice start first."}, nil
		}
		result, err := c.transcriber.Transcribe(ctx, path)
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("transcribe: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: fmt.Sprintf("Transcribed (%s): %s", result.Engine, result.Text)}, nil

	default:
		return &CommandResult{Success: false, Message: fmt.Sprintf("unknown voice subcommand: %s", args[0])}, nil
	}
}

func (c *AiderCommand) handleRepoMap(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	return &CommandResult{
		Success: true,
		Message: "Repo-map: use the 'repomap' tool for structured codebase analysis. The tree-sitter-based map includes function, class, and import definitions across Go, Python, JavaScript, TypeScript, Java, C, C++, Rust, and Ruby files.",
	}, nil
}
