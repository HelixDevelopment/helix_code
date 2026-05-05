package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Default lifecycle constants. Tests override IdleTimeout via
// SetIdleTimeout; the others are tuned to be liberal enough for the
// in-tree fake server and conservative enough not to mask real-server
// flakiness. None of them are constitutionally interesting — they are
// engineering knobs.
const (
	defaultIdleTimeout       = 5 * time.Minute
	defaultIdleCheckInterval = 50 * time.Millisecond
	defaultSpawnBackoff      = 2 * time.Second
	defaultMaxRestarts       = 3
	defaultRestartWindow     = 30 * time.Second
	defaultGracefulWait      = 2 * time.Second
)

// LSPManager owns a pool of running LSP server processes and exposes a
// file-extension-routed surface for the rest of HelixCode. It lazily
// spawns servers on first use, tears them down after an idle timeout,
// and respawns them on next use after a crash.
//
// The manager is safe for concurrent use from many goroutines.
type LSPManager struct {
	mu      sync.Mutex
	specs   []LSPServerSpec
	servers map[string]*managedServer // key = LSPServerSpec.Name
	log     *zap.Logger
	rootDir string

	idleTimeout     time.Duration
	spawnBackoff    time.Duration
	maxRestartCount int
	restartWindow   time.Duration
	gracefulWait    time.Duration

	closed bool
}

// NewLSPManager constructs a manager using sensible defaults and the
// provided allowlist of server specs. log may be nil (a Nop logger is
// substituted) so callers building a manager during early startup do
// not need to thread a real logger first.
func NewLSPManager(rootDir string, specs []LSPServerSpec, log *zap.Logger) *LSPManager {
	if log == nil {
		log = zap.NewNop()
	}
	specsCopy := make([]LSPServerSpec, len(specs))
	copy(specsCopy, specs)
	return &LSPManager{
		specs:           specsCopy,
		servers:         make(map[string]*managedServer),
		log:             log,
		rootDir:         rootDir,
		idleTimeout:     defaultIdleTimeout,
		spawnBackoff:    defaultSpawnBackoff,
		maxRestartCount: defaultMaxRestarts,
		restartWindow:   defaultRestartWindow,
		gracefulWait:    defaultGracefulWait,
	}
}

// SetIdleTimeout overrides the default idle-after-last-activity timeout.
// Calling this with a value <= 0 leaves the existing setting unchanged.
func (m *LSPManager) SetIdleTimeout(d time.Duration) {
	if d <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idleTimeout = d
	// Nudge each running server's watcher so the change takes effect
	// without waiting for the next event.
	for _, s := range m.servers {
		select {
		case s.touch <- struct{}{}:
		default:
		}
	}
}

// EnsureFor returns a ready LSPClient for the given file path, lazily
// spawning the server (and re-spawning it after a crash) as needed.
// Returns (nil, nil) if no spec in the allowlist matches the file's
// extension — callers must treat that case as "LSP not configured for
// this file" and continue without diagnostics.
func (m *LSPManager) EnsureFor(ctx context.Context, filePath string) (*LSPClient, error) {
	spec, ok := m.specForExtension(filePath)
	if !ok {
		return nil, nil
	}
	srv, err := m.ensureServer(ctx, spec)
	if err != nil {
		return nil, err
	}
	return srv.client, nil
}

// NotifyOpen lazily spawns the right server, opens the document on it,
// and returns. If the file is already open on its server this is a
// no-op. If no spec matches the extension the call is a no-op error-free.
//
// File content is read from disk; a missing file is reported as a
// diagnostic-irrelevant error so callers can decide whether to surface
// it or swallow it.
func (m *LSPManager) NotifyOpen(ctx context.Context, filePath string) error {
	spec, ok := m.specForExtension(filePath)
	if !ok {
		return nil
	}
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("lsp manager: abs(%s): %w", filePath, err)
	}
	srv, err := m.ensureServer(ctx, spec)
	if err != nil {
		return err
	}

	srv.mu.Lock()
	if srv.openFiles[abs] {
		srv.mu.Unlock()
		srv.markActivity()
		return nil
	}
	srv.openFiles[abs] = true
	srv.mu.Unlock()

	content, err := os.ReadFile(abs)
	if err != nil {
		// Roll back the open marker so a retry after fixing the file
		// can succeed cleanly.
		srv.mu.Lock()
		delete(srv.openFiles, abs)
		srv.mu.Unlock()
		return fmt.Errorf("lsp manager: read %s: %w", abs, err)
	}
	if err := srv.client.DidOpen(ctx, abs, spec.LanguageID, string(content)); err != nil {
		return fmt.Errorf("lsp manager: didOpen on %s: %w", spec.Name, err)
	}
	srv.markActivity()
	return nil
}

// NotifyChange forwards full-content didChange to the appropriate server.
// No-op error-free if the file's extension does not match any spec or
// the file was never opened on a server (the manager does not lazy-open
// on a change — callers should NotifyOpen first; this matches what the
// post-Edit auto-trigger in T08 will do).
func (m *LSPManager) NotifyChange(ctx context.Context, filePath, newContent string) error {
	spec, ok := m.specForExtension(filePath)
	if !ok {
		return nil
	}
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("lsp manager: abs(%s): %w", filePath, err)
	}

	m.mu.Lock()
	srv, exists := m.servers[spec.Name]
	m.mu.Unlock()
	if !exists || srv.client == nil {
		return nil
	}
	srv.mu.Lock()
	opened := srv.openFiles[abs]
	srv.mu.Unlock()
	if !opened {
		// Treat didChange-without-prior-didOpen as a soft open: the
		// LSPClient does not enforce ordering and the fake server (and
		// most real servers) will accept content with version >= 2.
		// We still record it so future changes are tracked.
		srv.mu.Lock()
		srv.openFiles[abs] = true
		srv.mu.Unlock()
		if err := srv.client.DidOpen(ctx, abs, spec.LanguageID, newContent); err != nil {
			return fmt.Errorf("lsp manager: implicit didOpen on %s: %w", spec.Name, err)
		}
		srv.markActivity()
		return nil
	}
	if err := srv.client.DidChange(ctx, abs, newContent); err != nil {
		return fmt.Errorf("lsp manager: didChange on %s: %w", spec.Name, err)
	}
	srv.markActivity()
	return nil
}

// GetDiagnostics returns the most-recent diagnostics published for the
// file by whichever server matches its extension. Empty if unrouted or
// not yet seen.
func (m *LSPManager) GetDiagnostics(filePath string) []Diagnostic {
	spec, ok := m.specForExtension(filePath)
	if !ok {
		return nil
	}
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil
	}
	m.mu.Lock()
	srv := m.servers[spec.Name]
	m.mu.Unlock()
	if srv == nil || srv.client == nil {
		return nil
	}
	srv.markActivity()
	return srv.client.GetDiagnostics(abs)
}

// AllDiagnostics returns the union of diagnostics across every running
// server. Useful for /lsp status output.
func (m *LSPManager) AllDiagnostics() []Diagnostic {
	m.mu.Lock()
	clients := make([]*LSPClient, 0, len(m.servers))
	for _, s := range m.servers {
		if s.client != nil {
			clients = append(clients, s.client)
		}
	}
	m.mu.Unlock()
	var out []Diagnostic
	for _, c := range clients {
		out = append(out, c.AllDiagnostics()...)
	}
	return out
}

// Servers returns a stable snapshot of every currently-managed server.
// Order is sorted by spec name.
func (m *LSPManager) Servers() []ServerInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ServerInfo, 0, len(m.servers))
	for _, s := range m.servers {
		s.mu.Lock()
		info := ServerInfo{
			Spec:       s.spec,
			Name:       s.spec.Name,
			Status:     s.status,
			PID:        s.pid,
			OpenFiles:  len(s.openFiles),
			LastActive: s.lastActive,
		}
		if !s.startedAt.IsZero() && s.status != ServerStatusStopped && s.status != ServerStatusCrashed {
			info.Uptime = time.Since(s.startedAt)
		}
		s.mu.Unlock()
		out = append(out, info)
	}
	return out
}

// Restart kills (graceful → SIGTERM → SIGKILL) the named server and
// drops it from the pool. The next NotifyOpen / EnsureFor / GetDiagnostics
// for a matching extension will lazily respawn it.
func (m *LSPManager) Restart(ctx context.Context, name string) error {
	if err := m.Stop(ctx, name); err != nil {
		// Stop already logs; surface the error so the caller sees it
		// but proceed to delete the entry so the next call respawns.
		m.log.Warn("restart: stop step returned", zap.String("server", name), zap.Error(err))
	}
	m.mu.Lock()
	delete(m.servers, name)
	m.mu.Unlock()
	return nil
}

// Stop tears down the named server (graceful + SIGTERM + SIGKILL ladder)
// and marks its entry Stopped. Idempotent.
func (m *LSPManager) Stop(ctx context.Context, name string) error {
	m.mu.Lock()
	srv := m.servers[name]
	m.mu.Unlock()
	if srv == nil {
		return nil
	}
	return m.stopServer(ctx, srv)
}

// Shutdown stops every managed server and marks the manager as closed.
// Subsequent EnsureFor / NotifyOpen calls return ErrManagerClosed.
func (m *LSPManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	servers := make([]*managedServer, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	m.mu.Unlock()

	var firstErr error
	for _, s := range servers {
		if err := m.stopServer(ctx, s); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ErrManagerClosed is returned by EnsureFor / NotifyOpen / NotifyChange
// after Shutdown has been called.
var ErrManagerClosed = errors.New("lsp manager: closed")

// ---------- internals ----------

// specForExtension picks the first allowlist entry whose FileExtensions
// list contains filepath.Ext(p) (case-insensitive). The router is
// first-match-wins so callers can shadow built-ins with project-specific
// overrides simply by listing them earlier.
func (m *LSPManager) specForExtension(p string) (LSPServerSpec, bool) {
	ext := strings.ToLower(filepath.Ext(p))
	if ext == "" {
		return LSPServerSpec{}, false
	}
	for _, s := range m.specs {
		for _, e := range s.FileExtensions {
			if strings.ToLower(e) == ext {
				return s, true
			}
		}
	}
	return LSPServerSpec{}, false
}

// ensureServer returns a Ready managedServer for the given spec,
// spawning (or respawning after crash) as needed. Holds m.mu only
// long enough to peek/insert the entry; the actual exec.Start happens
// outside the lock.
func (m *LSPManager) ensureServer(ctx context.Context, spec LSPServerSpec) (*managedServer, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrManagerClosed
	}
	srv, ok := m.servers[spec.Name]
	if ok {
		switch srv.statusLocked() {
		case ServerStatusReady, ServerStatusStarting, ServerStatusIdle:
			m.mu.Unlock()
			return srv, nil
		case ServerStatusCrashed, ServerStatusStopped:
			// Fall through to respawn, dropping the dead entry.
			delete(m.servers, spec.Name)
		case ServerStatusStopping:
			// Wait for stop to finish, then respawn.
			delete(m.servers, spec.Name)
		}
	}
	// Reserve the slot with a Starting entry so concurrent callers
	// converge on the same managedServer pointer.
	srv = &managedServer{
		spec:       spec,
		status:     ServerStatusStarting,
		openFiles:  make(map[string]bool),
		touch:      make(chan struct{}, 1),
		lastActive: time.Now(),
	}
	m.servers[spec.Name] = srv
	m.mu.Unlock()

	if err := m.spawn(ctx, srv); err != nil {
		m.mu.Lock()
		delete(m.servers, spec.Name)
		m.mu.Unlock()
		return nil, err
	}
	return srv, nil
}

// spawn executes the server binary, wires stdio, builds the LSPClient,
// performs the LSP handshake, then starts the wait + idle goroutines.
func (m *LSPManager) spawn(ctx context.Context, srv *managedServer) error {
	// Use a detached context for the subprocess: caller-provided ctx may
	// be a short-lived request context, but the server should outlive
	// it. Cancellation flows through stopServer instead.
	cmd := exec.Command(srv.spec.Binary, srv.spec.Args...)
	cmd.Dir = m.rootDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("lsp manager: stdin pipe %s: %w", srv.spec.Name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("lsp manager: stdout pipe %s: %w", srv.spec.Name, err)
	}
	// Capture stderr for crash diagnostics; tests want clean output so
	// we discard by default. The fake server is silent unless -debug.
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("lsp manager: start %s (%s): %w", srv.spec.Name, srv.spec.Binary, err)
	}

	transport := &subprocessTransport{stdout: stdout, stdin: stdin}
	client := NewLSPClient(transport, srv.spec.Name)

	srv.mu.Lock()
	srv.cmd = cmd
	srv.stdin = stdin
	srv.stdout = stdout
	srv.client = client
	srv.pid = cmd.Process.Pid
	srv.startedAt = time.Now()
	srv.lastActive = time.Now()
	srv.exited = make(chan struct{})
	srv.mu.Unlock()

	if err := client.Initialize(ctx, m.rootDir, srv.spec.InitializationOpts); err != nil {
		// Initialize failed: tear down before reporting.
		_ = client.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		srv.mu.Lock()
		srv.status = ServerStatusCrashed
		srv.mu.Unlock()
		return fmt.Errorf("lsp manager: initialize %s: %w", srv.spec.Name, err)
	}

	srv.mu.Lock()
	srv.status = ServerStatusReady
	srv.mu.Unlock()

	// Goroutine 1: wait for the process to exit. Marks Crashed unless
	// stopServer beat us to it (in which case status will already be
	// Stopped or Stopping).
	go m.watchProcess(srv)

	// Goroutine 2: idle timeout. Signals a stop after idleTimeout of
	// inactivity and exits.
	go m.watchIdle(srv)

	m.log.Info("lsp manager: spawned",
		zap.String("server", srv.spec.Name),
		zap.Int("pid", srv.pid),
		zap.String("binary", srv.spec.Binary),
	)
	return nil
}

// watchProcess blocks on cmd.Wait(); when it returns the process is
// gone. We mark Crashed only if the manager did not initiate the stop.
// On return, srv.exited is closed so stopServer can synchronise on
// "Wait has finished" without polling cmd.ProcessState (which races
// with cmd.Wait()'s internal state writes).
func (m *LSPManager) watchProcess(srv *managedServer) {
	srv.mu.Lock()
	cmd := srv.cmd
	exited := srv.exited
	srv.mu.Unlock()
	if cmd == nil {
		return
	}
	_ = cmd.Wait()
	close(exited)

	srv.mu.Lock()
	srv.restartHistory = append(srv.restartHistory, time.Now())
	switch srv.status {
	case ServerStatusStopping, ServerStatusStopped:
		// Manager-initiated; nothing to log.
		srv.status = ServerStatusStopped
		srv.mu.Unlock()
	default:
		srv.status = ServerStatusCrashed
		pid := srv.pid
		srv.mu.Unlock()
		m.log.Warn("lsp manager: server crashed",
			zap.String("server", srv.spec.Name),
			zap.Int("pid", pid),
		)
	}
}

// watchIdle polls every idleCheckInterval and triggers a Stop when
// (now - lastActive) > idleTimeout. Exits as soon as the server status
// is Stopping/Stopped/Crashed.
func (m *LSPManager) watchIdle(srv *managedServer) {
	for {
		m.mu.Lock()
		idleTimeout := m.idleTimeout
		m.mu.Unlock()
		if idleTimeout <= 0 {
			idleTimeout = defaultIdleTimeout
		}

		srv.mu.Lock()
		status := srv.status
		last := srv.lastActive
		srv.mu.Unlock()
		if status == ServerStatusStopped || status == ServerStatusStopping || status == ServerStatusCrashed {
			return
		}

		idleFor := time.Since(last)
		if idleFor >= idleTimeout {
			m.log.Info("lsp manager: idle timeout",
				zap.String("server", srv.spec.Name),
				zap.Duration("idle_for", idleFor),
			)
			ctx, cancel := context.WithTimeout(context.Background(), m.gracefulWait*2)
			_ = m.stopServer(ctx, srv)
			cancel()
			return
		}

		// Sleep until either the next check tick or a touch arrives.
		wait := idleTimeout - idleFor
		if wait > defaultIdleCheckInterval {
			wait = defaultIdleCheckInterval
		}
		select {
		case <-srv.touch:
			// Activity recorded; loop will recompute idleFor on next pass.
		case <-time.After(wait):
		}
	}
}

// stopServer runs the graceful → SIGTERM → SIGKILL ladder for a single
// server. Idempotent; re-stopping a Stopped server is a no-op.
func (m *LSPManager) stopServer(ctx context.Context, srv *managedServer) error {
	srv.mu.Lock()
	if srv.status == ServerStatusStopped {
		srv.mu.Unlock()
		return nil
	}
	if srv.status == ServerStatusCrashed {
		// Crashed servers may still leak the client transport; close it.
		client := srv.client
		srv.mu.Unlock()
		if client != nil {
			_ = client.Close()
		}
		srv.mu.Lock()
		srv.status = ServerStatusStopped
		srv.mu.Unlock()
		return nil
	}
	srv.status = ServerStatusStopping
	client := srv.client
	cmd := srv.cmd
	exited := srv.exited
	srv.mu.Unlock()

	// Step 1: graceful LSP shutdown + exit. The fake server (and any
	// well-behaved real server) exits its process from the LSP `exit`
	// notification; this often satisfies the wait path with no signals.
	if client != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, m.gracefulWait)
		_ = client.Shutdown(shutdownCtx)
		cancel()
	}

	// Step 2..4: signal escalation, gated on the exited channel that
	// watchProcess closes after cmd.Wait() returns. Using the channel
	// avoids polling cmd.ProcessState, which races with cmd.Wait()'s
	// internal state writes (per Go race detector).
	if cmd != nil && cmd.Process != nil && exited != nil {
		select {
		case <-exited:
		case <-time.After(m.gracefulWait):
			// Step 3: SIGTERM.
			_ = cmd.Process.Signal(os.Interrupt)
			select {
			case <-exited:
			case <-time.After(m.gracefulWait):
				// Step 4: SIGKILL.
				_ = cmd.Process.Kill()
				select {
				case <-exited:
				case <-time.After(m.gracefulWait):
					// Give up; watchProcess goroutine still owns Wait().
				}
			}
		}
	}

	if client != nil {
		_ = client.Close()
	}

	srv.mu.Lock()
	srv.status = ServerStatusStopped
	srv.mu.Unlock()
	return nil
}

// ---------- managedServer ----------

// managedServer wraps a single running subprocess and its LSPClient.
type managedServer struct {
	mu sync.Mutex

	spec    LSPServerSpec
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	client  *LSPClient
	pid     int
	status  ServerStatus
	openFiles map[string]bool

	startedAt      time.Time
	lastActive     time.Time
	restartHistory []time.Time

	// exited is closed by watchProcess after cmd.Wait() returns, so
	// stopServer can synchronise on "process is reaped" without
	// polling cmd.ProcessState (which the race detector flags).
	exited chan struct{}

	// touch is a non-blocking signal that activity was recorded; the
	// idle watcher drains it to wake up early after a long sleep.
	touch chan struct{}
}

// statusLocked must only be called with m.mu held by the caller.
func (s *managedServer) statusLocked() ServerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// markActivity bumps lastActive and pokes the idle watcher. Safe to
// call from any goroutine.
func (s *managedServer) markActivity() {
	s.mu.Lock()
	s.lastActive = time.Now()
	s.mu.Unlock()
	select {
	case s.touch <- struct{}{}:
	default:
	}
}

// ---------- subprocessTransport ----------

// subprocessTransport adapts a subprocess's stdin (write side) and
// stdout (read side) into the io.ReadWriteCloser shape that
// jsonrpc2.NewStream expects. Closing either end closes both, which
// in turn lets the LSPClient run goroutine drain.
type subprocessTransport struct {
	stdout io.ReadCloser
	stdin  io.WriteCloser
}

func (t *subprocessTransport) Read(p []byte) (int, error)  { return t.stdout.Read(p) }
func (t *subprocessTransport) Write(p []byte) (int, error) { return t.stdin.Write(p) }
func (t *subprocessTransport) Close() error {
	err1 := t.stdin.Close()
	err2 := t.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
