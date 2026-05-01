// Package helixqa provides an embedded wrapper for the HelixQA testing framework.
package helixqa

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"dev.helix.code/internal/config"

	hqaConfig "digital.vasic.helixqa/pkg/config"
	hqaEvidence "digital.vasic.helixqa/pkg/evidence"
	hqaOrchestrator "digital.vasic.helixqa/pkg/orchestrator"
	"digital.vasic.helixqa/pkg/reporter"
)

// SessionState tracks a single QA session within the HelixCode server.
type SessionState struct {
	ID               string                  `json:"id"`
	Status           string                  `json:"status"` // pending|running|completed|failed|cancelled
	Phase            string                  `json:"phase"`
	PhaseProgress    float64                 `json:"phase_progress"`
	Platforms        []string                `json:"platforms"`
	Banks            []string                `json:"banks"`
	StartTime        time.Time               `json:"start_time"`
	EndTime          *time.Time              `json:"end_time,omitempty"`
	Result           *hqaOrchestrator.Result `json:"result,omitempty"`
	AutonomousResult interface{}             `json:"autonomous_result,omitempty"`
	ReportPath       string                  `json:"report_path,omitempty"`
	CancelFunc       context.CancelFunc      `json:"-"`
	Mu               sync.RWMutex            `json:"-"`
}

// Engine is the singleton QA engine embedded in the HelixCode server.
type Engine struct {
	sessions    map[string]*SessionState
	sessionMu   sync.RWMutex
	cfg         *config.Config
	qaCfg       *hqaConfig.Config
	evidenceDir string
	enabled     bool
}

// NewEngine builds the embedded QA engine from HelixCode configuration.
func NewEngine(cfg *config.Config) (*Engine, error) {
	if !cfg.QA.Enabled {
		return &Engine{enabled: false}, nil
	}
	qaCfg, err := buildQAConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("helixqa config build: %w", err)
	}
	return &Engine{
		sessions:    make(map[string]*SessionState),
		cfg:         cfg,
		qaCfg:       qaCfg,
		evidenceDir: cfg.QA.OutputDir,
		enabled:     true,
	}, nil
}

// Enabled returns true if the QA engine is configured and active.
func (e *Engine) Enabled() bool {
	return e.enabled
}

// StartSession begins a new QA session and returns its ID.
func (e *Engine) StartSession(ctx context.Context, id string, platforms, banks []string, autonomous bool) (*SessionState, error) {
	if !e.enabled {
		return nil, fmt.Errorf("QA is disabled")
	}
	if id == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	// Validate banks exist
	for _, bank := range banks {
		if _, err := os.Stat(bank); err != nil {
			return nil, fmt.Errorf("bank not found: %s", bank)
		}
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	state := &SessionState{
		ID:        id,
		Status:    "pending",
		Platforms: platforms,
		Banks:     banks,
		StartTime: time.Now(),
		CancelFunc: cancel,
	}

	e.sessionMu.Lock()
	e.sessions[id] = state
	e.sessionMu.Unlock()

	go func() {
		defer cancel()
		state.Mu.Lock()
		state.Status = "running"
		state.Phase = "orchestration"
		state.PhaseProgress = 0.0
		state.Mu.Unlock()

		cfg := e.qaCfg
		cfg.Banks = banks
		parsedPlatforms, err := hqaConfig.ParsePlatforms(strings.Join(platforms, ","))
		if err != nil {
			state.Mu.Lock()
			state.Status = "failed"
			state.Phase = "error"
			now := time.Now()
			state.EndTime = &now
			state.Mu.Unlock()
			return
		}
		cfg.Platforms = parsedPlatforms

		orc := hqaOrchestrator.New(cfg)
		res, err := orc.Run(sessionCtx)

		state.Mu.Lock()
		defer state.Mu.Unlock()
		if err != nil && sessionCtx.Err() != context.Canceled {
			state.Status = "failed"
		} else if sessionCtx.Err() == context.Canceled {
			state.Status = "cancelled"
		} else {
			state.Status = "completed"
			state.Result = res
			if res != nil {
				state.ReportPath = res.ReportPath
			}
		}
		now := time.Now()
		state.EndTime = &now
		state.PhaseProgress = 1.0
	}()

	return state, nil
}

// GetSession retrieves a session by ID.
func (e *Engine) GetSession(id string) (*SessionState, bool) {
	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()
	s, ok := e.sessions[id]
	return s, ok
}

// CancelSession signals cancellation for a running session.
func (e *Engine) CancelSession(id string) error {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	s, ok := e.sessions[id]
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.CancelFunc != nil {
		s.CancelFunc()
		s.Status = "cancelled"
		now := time.Now()
		s.EndTime = &now
	}
	return nil
}

// ListSessions returns all session states.
func (e *Engine) ListSessions() []*SessionState {
	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()
	out := make([]*SessionState, 0, len(e.sessions))
	for _, s := range e.sessions {
		out = append(out, s)
	}
	return out
}

// EvidenceCollector returns the evidence collector for on-demand screenshots.
func (e *Engine) EvidenceCollector(platform hqaConfig.Platform) *hqaEvidence.Collector {
	return hqaEvidence.New(
		hqaEvidence.WithOutputDir(e.evidenceDir),
		hqaEvidence.WithPlatform(platform),
	)
}

// GenerateReport creates a report for a completed session in the requested format.
func (e *Engine) GenerateReport(state *SessionState, format string) ([]byte, string, error) {
	if state == nil || state.Result == nil || state.Result.Report == nil {
		return nil, "", fmt.Errorf("no report available")
	}
	rpt := reporter.New(
		reporter.WithOutputDir(e.evidenceDir),
		reporter.WithReportFormat(hqaConfig.ReportMarkdown),
	)

	switch format {
	case "html":
		rpt = reporter.New(
			reporter.WithOutputDir(e.evidenceDir),
			reporter.WithReportFormat(hqaConfig.ReportHTML),
		)
	case "json":
		rpt = reporter.New(
			reporter.WithOutputDir(e.evidenceDir),
			reporter.WithReportFormat(hqaConfig.ReportJSON),
		)
	}

	path, err := rpt.WriteReport(state.Result.Report, e.evidenceDir)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	return data, path, err
}

func buildQAConfig(cfg *config.Config) (*hqaConfig.Config, error) {
	qc := cfg.QA
	platforms, err := hqaConfig.ParsePlatforms(strings.Join(qc.Platforms, ","))
	if err != nil {
		return nil, fmt.Errorf("parse platforms: %w", err)
	}
	banks := hqaConfig.ParseBanks(qc.BanksDir)
	if len(banks) == 0 && qc.BanksDir != "" {
		banks = []string{qc.BanksDir}
	}

	reportFormat := hqaConfig.ReportMarkdown
	if len(qc.ReportFormats) > 0 {
		switch qc.ReportFormats[0] {
		case "html":
			reportFormat = hqaConfig.ReportHTML
		case "json":
			reportFormat = hqaConfig.ReportJSON
		}
	}

	return &hqaConfig.Config{
		Banks:         banks,
		Platforms:     platforms,
		Device:        qc.DeviceID,
		OutputDir:     qc.OutputDir,
		ReportFormat:  reportFormat,
		ValidateSteps: true,
		Record:        qc.RecordVideo,
		Verbose:       cfg.Logging.Level == "debug",
		Timeout:       2 * time.Hour,
		StepTimeout:   5 * time.Minute,
		Autonomous: hqaConfig.AutonomousConfig{
			Enabled:              qc.Autonomous,
			CoverageTarget:       qc.CoverageTarget,
			CuriosityEnabled:     qc.CuriosityEnabled,
			CuriosityTimeout:     30 * time.Minute,
			VisionProvider:       qc.VisionProvider,
			LLMProvider:          qc.LLMProvider,
			LLMAPIKey:            qc.LLMAPIKey,
			RecordingScreenshots: qc.RecordScreenshots,
			RecordingVideo:       qc.RecordVideo,
		},
	}, nil
}
