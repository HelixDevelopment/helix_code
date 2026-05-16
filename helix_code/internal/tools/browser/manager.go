package browser

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

// SessionFactory constructs a single BrowserSession given the chromium
// path resolved by the manager's ChromeDiscovery. The factory is a
// test seam — production wiring uses defaultSessionFactory which
// spawns a real chromedp ExecAllocator + Context; unit tests override
// to avoid spawning chromium.
type SessionFactory func(ctx context.Context, mgr *BrowserManager, opts BrowserOptions) (*BrowserSession, error)

// BrowserManager owns the single per-process chromedp session for the
// F23 cline-style browser tool surface. Lifecycle: lazy-create on
// first EnsureSession; idempotent close via CloseSession; same
// pointer across concurrent EnsureSession calls (mutex serialises
// only lifecycle transitions, atomic.Pointer makes reads lock-free).
type BrowserManager struct {
	current        atomic.Pointer[BrowserSession]
	screenshotRoot string
	discovery      ChromeDiscovery
	log            *zap.Logger
	mu             sync.Mutex
	sessionFactory SessionFactory
}

// NewBrowserManager wires a manager with the production session
// factory (defaultSessionFactory). The discovery argument resolves
// the chromium binary; pass NewDefaultChromeDiscovery() for the
// real lookup.
func NewBrowserManager(d ChromeDiscovery, log *zap.Logger) *BrowserManager {
	if log == nil {
		log = zap.NewNop()
	}
	return &BrowserManager{
		screenshotRoot: defaultScreenshotRoot(),
		discovery:      d,
		log:            log,
		sessionFactory: defaultSessionFactory,
	}
}

// SetSessionFactory replaces the session factory. Used by unit tests
// to inject a stub that does not spawn chromium. Production code
// should not call this.
func (m *BrowserManager) SetSessionFactory(f SessionFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionFactory = f
}

// EnsureSession returns the active session, lazy-creating one if
// none exists. Concurrent callers all observe the same pointer.
func (m *BrowserManager) EnsureSession(ctx context.Context) (*BrowserSession, error) {
	if s := m.current.Load(); s != nil {
		return s, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.current.Load(); s != nil {
		return s, nil
	}
	opts := OptionsFromEnv()
	s, err := m.sessionFactory(ctx, m, opts)
	if err != nil {
		return nil, err
	}
	m.current.Store(s)
	return s, nil
}

// RequireSession returns the active session or ErrNoActiveSession if
// none has been created. Used by tools that must NOT lazy-create
// (snapshot, click, type, screenshot, close).
func (m *BrowserManager) RequireSession() (*BrowserSession, error) {
	s := m.current.Load()
	if s == nil {
		return nil, ErrNoActiveSession
	}
	return s, nil
}

// CloseSession tears down the active session if any. Idempotent:
// closing a non-existent session is a no-op success.
func (m *BrowserManager) CloseSession() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.current.Swap(nil)
	if s == nil {
		return nil
	}
	return s.Close()
}

// Status returns a snapshot of the manager's current state.
func (m *BrowserManager) Status() ManagerStatus {
	s := m.current.Load()
	if s == nil {
		return ManagerStatus{Active: false}
	}
	return ManagerStatus{
		Active:        true,
		ChromiumPath:  s.chromiumPath,
		ScreenshotDir: s.screenshotDir,
		Headed:        s.headed,
		CreatedAt:     s.createdAt,
	}
}

// ScreenshotRoot returns the configured screenshot tempdir parent.
func (m *BrowserManager) ScreenshotRoot() string { return m.screenshotRoot }

// Discovery exposes the configured ChromeDiscovery (used by the
// default session factory).
func (m *BrowserManager) Discovery() ChromeDiscovery { return m.discovery }

// Logger exposes the manager's zap logger.
func (m *BrowserManager) Logger() *zap.Logger { return m.log }

// defaultScreenshotRoot returns $XDG_DATA_HOME/helixcode/browser/screenshots
// (or $HOME/.local/share/helixcode/browser/screenshots if XDG_DATA_HOME
// is unset).
func defaultScreenshotRoot() string {
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			xdg = filepath.Join(home, ".local", "share")
		} else {
			xdg = os.TempDir()
		}
	}
	return filepath.Join(xdg, "helixcode", "browser", "screenshots")
}

// defaultSessionFactory constructs a real chromedp session: resolves
// chromium via the manager's ChromeDiscovery, builds an ExecAllocator
// with the headless/headed option from env, opens a chromedp.Context,
// and creates the per-session screenshot tempdir. Returns
// ErrChromiumNotFound if discovery fails.
func defaultSessionFactory(ctx context.Context, mgr *BrowserManager, opts BrowserOptions) (*BrowserSession, error) {
	if mgr == nil {
		return nil, fmt.Errorf("browser: nil manager")
	}
	disc := mgr.Discovery()
	if disc == nil {
		return nil, ErrChromiumNotFound
	}
	chromePath, err := disc.FindChrome()
	if err != nil || chromePath == "" {
		return nil, fmt.Errorf("%w: %v", ErrChromiumNotFound, err)
	}
	sessionID, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("browser: session id: %w", err)
	}
	screenshotDir := filepath.Join(mgr.ScreenshotRoot(), sessionID)
	if err := os.MkdirAll(screenshotDir, 0o700); err != nil {
		return nil, fmt.Errorf("browser: mkdir screenshot dir: %w", err)
	}
	allocOpts := append([]chromedp.ExecAllocatorOption{},
		chromedp.ExecPath(chromePath),
		chromedp.WindowSize(opts.ViewportWidth, opts.ViewportHeight),
		chromedp.NoSandbox,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
	)
	if opts.Headless {
		allocOpts = append(allocOpts, chromedp.Headless)
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	chromedpCtx, chromedpCancel := chromedp.NewContext(allocCtx)
	combinedCancel := func() {
		chromedpCancel()
	}
	// Materialise the browser by running an empty action set; this
	// surfaces chromium-launch errors immediately rather than at the
	// first navigate.
	if err := chromedp.Run(chromedpCtx); err != nil {
		combinedCancel()
		allocCancel()
		_ = os.RemoveAll(screenshotDir)
		return nil, fmt.Errorf("browser: chromedp launch failed: %w", err)
	}
	return &BrowserSession{
		ctx:           chromedpCtx,
		cancel:        combinedCancel,
		allocCancel:   allocCancel,
		screenshotDir: screenshotDir,
		chromiumPath:  chromePath,
		headed:        !opts.Headless,
		createdAt:     time.Now(),
		log:           mgr.Logger(),
	}, nil
}

// newSessionID returns a 16-hex-char session id (8 random bytes).
func newSessionID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
