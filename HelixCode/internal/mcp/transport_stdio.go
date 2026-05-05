package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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

// readResult carries a parsed frame or an error from the read loop.
type readResult struct {
	msg *MCPMessage
	err error
}

type stdioTransport struct {
	cfg     StdioConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader // owned exclusively by readLoop; no other caller may read it
	ring    *stderrRing
	readCh  chan readResult
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
		readCh:  make(chan readResult, 4),
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
	go t.readLoop()
	return nil
}

// readLoop is the single goroutine that owns t.stdout. It runs for the lifetime
// of the transport, pushing parsed frames and errors onto t.readCh.
func (t *stdioTransport) readLoop() {
	for {
		line, err := t.stdout.ReadBytes('\n')
		if len(line) > 0 {
			var m MCPMessage
			if uerr := json.Unmarshal(bytes.TrimSpace(line), &m); uerr != nil {
				t.sendRead(readResult{err: fmt.Errorf("%w: %v", ErrProtocol, uerr)})
				if err != nil {
					return
				}
				continue
			}
			t.sendRead(readResult{msg: &m})
		}
		if err != nil {
			// Surface EOF or transport error to the next Recv caller, then stop.
			t.sendRead(readResult{err: err})
			return
		}
	}
}

// sendRead delivers a result onto readCh, or aborts silently if the transport
// is closing so the goroutine is never blocked forever.
func (t *stdioTransport) sendRead(r readResult) {
	select {
	case t.readCh <- r:
	case <-t.closeCh:
	}
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
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.closeCh:
		return nil, ErrTransportClosed
	case r, ok := <-t.readCh:
		if !ok {
			return nil, ErrTransportClosed
		}
		return r.msg, r.err
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
