package shell

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// shellLineSink is an alias for the line-sink callback used by ExecuteWithProgress.
// It intentionally mirrors tools.LineSink without importing the parent package
// (dev.helix.code/internal/tools imports dev.helix.code/internal/tools/shell, so
// the reverse import would create a cycle).  The types_background.go layer in
// internal/tools bridges the two at the registry level.
type shellLineSink = func(string)

// ExecuteWithProgress runs the shell command and streams stdout/stderr lines
// through sink as they are produced. The final aggregated output and exit code
// are returned for compatibility with the existing shell tool's Execute return
// shape.
//
// This makes ShellExecutor implement tools.BackgroundAware (via the adapter in
// internal/tools/shell_tools.go). When ToolRegistry dispatches with
// run_in_background:true, the BackgroundManager invokes this method instead of
// Execute, letting the BackgroundTask's output ring receive real-time progress.
func (se *ShellExecutor) ExecuteWithProgress(
	ctx context.Context,
	params map[string]interface{},
	sink shellLineSink,
) (interface{}, error) {
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("shell: command must be a non-empty string")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if cwd, ok := params["cwd"].(string); ok && cwd != "" {
		cmd.Dir = cwd
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("shell: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("shell: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("shell: start: %w", err)
	}

	var (
		mu       sync.Mutex
		lines    []string
		appendLn = func(line string) {
			mu.Lock()
			lines = append(lines, line)
			mu.Unlock()
			if sink != nil {
				sink(line)
			}
		}
	)

	scan := func(rd io.Reader, wg *sync.WaitGroup) {
		defer wg.Done()
		s := bufio.NewScanner(rd)
		s.Buffer(make([]byte, 4096), 1024*1024)
		for s.Scan() {
			appendLn(s.Text())
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scan(stdout, &wg)
	go scan(stderr, &wg)
	wg.Wait()

	waitErr := cmd.Wait()

	mu.Lock()
	output := strings.Join(lines, "\n")
	mu.Unlock()

	if waitErr != nil {
		return map[string]interface{}{
			"output":    output,
			"exit_code": exitCodeFromError(waitErr, cmd),
		}, fmt.Errorf("shell: command exit: %w", waitErr)
	}
	return map[string]interface{}{
		"output":    output,
		"exit_code": 0,
	}, nil
}

// exitCodeFromError extracts the OS exit code from a process error.
func exitCodeFromError(err error, cmd *exec.Cmd) int {
	if cmd != nil && cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}
