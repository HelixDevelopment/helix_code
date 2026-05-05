package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// StdioConfig configures a stdio MCP server subprocess.
type StdioConfig struct {
	Command []string
	Env     map[string]string
	Cwd     string
}

// stderrRing is a 64KB ring buffer for subprocess stderr.
type stderrRing struct {
	mu  sync.Mutex
	buf bytes.Buffer
	cap int
}

func (r *stderrRing) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.buf.Len()+len(p) > r.cap {
		drop := r.buf.Len() + len(p) - r.cap
		if drop > r.buf.Len() {
			r.buf.Reset()
		} else {
			r.buf.Next(drop)
		}
	}
	return r.buf.Write(p)
}

func (r *stderrRing) Snapshot() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]byte, r.buf.Len())
	copy(out, r.buf.Bytes())
	return out
}

type stdioTransport struct {
	cfg     StdioConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	ring    *stderrRing
	mu      sync.Mutex
	closed  bool
	closeCh chan struct{}
}

// NewStdioTransport creates a new stdio MCP transport that launches a subprocess
// and communicates via newline-delimited JSON-RPC on stdin/stdout.
func NewStdioTransport(cfg StdioConfig) *stdioTransport {
	return &stdioTransport{
		cfg:     cfg,
		ring:    &stderrRing{cap: 64 * 1024},
		closeCh: make(chan struct{}),
	}
}

func (t *stdioTransport) Type() TransportType { return TransportStdio }

func (t *stdioTransport) Open(ctx context.Context) error {
	if len(t.cfg.Command) == 0 {
		return fmt.Errorf("mcp stdio: empty command")
	}
	t.cmd = exec.CommandContext(ctx, t.cfg.Command[0], t.cfg.Command[1:]...)
	if t.cfg.Cwd != "" {
		t.cmd.Dir = t.cfg.Cwd
	}
	t.cmd.Env = mergeEnv(t.cfg.Env)
	configureProcAttrs(t.cmd)

	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stdin pipe: %w", err)
	}
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stdout pipe: %w", err)
	}
	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("mcp stdio: stderr pipe: %w", err)
	}
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("mcp stdio: start %v: %w", t.cfg.Command, err)
	}
	t.stdin = stdin
	t.stdout = bufio.NewReaderSize(stdout, 1024*1024)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				t.ring.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	return nil
}

func (t *stdioTransport) Send(ctx context.Context, msg *MCPMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.stdin == nil {
		return ErrTransportClosed
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("mcp stdio: marshal: %w", err)
	}
	b = append(b, '\n')
	if _, err := t.stdin.Write(b); err != nil {
		return fmt.Errorf("mcp stdio: write: %w", err)
	}
	return nil
}

func (t *stdioTransport) Recv(ctx context.Context) (*MCPMessage, error) {
	if t.closed || t.stdout == nil {
		return nil, ErrTransportClosed
	}
	type result struct {
		msg *MCPMessage
		err error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := t.stdout.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && len(line) == 0 {
				ch <- result{nil, io.EOF}
				return
			}
		}
		var m MCPMessage
		if uerr := json.Unmarshal(bytes.TrimSpace(line), &m); uerr != nil {
			ch <- result{nil, fmt.Errorf("%w: %v", ErrProtocol, uerr)}
			return
		}
		ch <- result{&m, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err == io.EOF {
			return nil, io.EOF
		}
		if r.err != nil && r.msg == nil {
			return nil, r.err
		}
		return r.msg, nil
	case <-t.closeCh:
		return nil, ErrTransportClosed
	}
}

func (t *stdioTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	close(t.closeCh)
	t.mu.Unlock()
	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		_ = killProcessGroup(t.cmd)
		_, _ = t.cmd.Process.Wait()
	}
	return nil
}

// PID returns the process ID of the running subprocess, or 0 if not started.
func (t *stdioTransport) PID() int {
	if t.cmd == nil || t.cmd.Process == nil {
		return 0
	}
	return t.cmd.Process.Pid
}

// Stderr returns a snapshot of the subprocess stderr ring buffer (up to 64KB).
func (t *stdioTransport) Stderr() []byte {
	return t.ring.Snapshot()
}

func mergeEnv(extra map[string]string) []string {
	out := append([]string(nil), getEnv()...)
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}
