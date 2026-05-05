package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// shellRunnerPayload is the JSON document written to a hook script's stdin.
type shellRunnerPayload struct {
	Type      string                 `json:"type"`
	Timestamp string                 `json:"timestamp"`
	SessionID string                 `json:"session_id,omitempty"`
	Source    string                 `json:"source,omitempty"`
	Data      map[string]interface{} `json:"data"`
}

// shellRunnerModify is the JSON document a hook may print to stdout to mutate
// event.Data for downstream handlers. Read in F05; back-propagation to
// originating-operation params is N1 (out of scope).
type shellRunnerModify struct {
	Data map[string]interface{} `json:"data"`
}

// NewShellRunner returns a HookFunc that exec's scriptPath with the event
// payload on stdin. Behaviour:
//   - Non-zero exit → returns error wrapping captured stderr (treated as block).
//   - timeout > 0 → applied via context.WithTimeout; deadline = block.
//   - Missing script → returns error (fail-closed).
//   - Stdout JSON matching {"data":{...}} → merged into event.Data for
//     downstream handlers; malformed stdout JSON is logged and ignored.
//   - Caller's context cancellation aborts the script (including child processes).
//
// Platform note: setProcessGroup / killProcessGroup are implemented in
// shell_runner_unix.go (//go:build unix) and shell_runner_windows.go
// (//go:build windows) respectively.
func NewShellRunner(scriptPath string, timeout time.Duration) HookFunc {
	return func(ctx context.Context, event *Event) error {
		runCtx := ctx
		var cancel context.CancelFunc
		if timeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		payload := shellRunnerPayload{
			Type:      string(event.Type),
			Timestamp: event.Timestamp.Format(time.RFC3339),
			Source:    event.Source,
			Data:      event.Data,
		}
		if sid, ok := event.Metadata["session_id"]; ok {
			payload.SessionID = sid
		}
		stdinJSON, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshalling event payload: %w", err)
		}

		// Use exec.Command (not CommandContext) so we can manage process group
		// termination ourselves — CommandContext only kills the direct process,
		// leaving grandchild processes (e.g. `sleep`) alive.
		cmd := exec.Command(scriptPath) //nolint:gosec
		cmd.Stdin = bytes.NewReader(stdinJSON)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		// Place the script in its own process group (Unix) so we can kill the
		// whole group (script + all its children) when the context expires.
		// On Windows this is a no-op; process tree cleanup relies on
		// cmd.Process.Kill() in killProcessGroup.
		setProcessGroup(cmd)

		if startErr := cmd.Start(); startErr != nil {
			return fmt.Errorf("hook script %s: %w", scriptPath, startErr)
		}

		// Wait in a goroutine and signal the done channel.
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case runErr := <-done:
			// Process finished normally (or with non-zero exit).
			if runErr != nil {
				return fmt.Errorf("hook script %s: %w (stderr: %s)", scriptPath, runErr, stderr.String())
			}
		case <-runCtx.Done():
			// Context cancelled or timed out — kill the entire process group.
			if cmd.Process != nil {
				killProcessGroup(cmd)
			}
			// Drain the wait goroutine.
			<-done
			return fmt.Errorf("hook script %s: %w", scriptPath, runCtx.Err())
		}

		// Read modify-payload from stdout. Malformed = log + ignore.
		if stdout.Len() > 0 {
			var mod shellRunnerModify
			if jerr := json.Unmarshal(stdout.Bytes(), &mod); jerr != nil {
				log.Printf("WARN hooks shell_runner: stdout from %s is not valid JSON; ignoring (%v)", scriptPath, jerr)
			} else if mod.Data != nil {
				if event.Data == nil {
					event.Data = map[string]interface{}{}
				}
				for k, v := range mod.Data {
					event.Data[k] = v
				}
			}
		}
		return nil
	}
}
