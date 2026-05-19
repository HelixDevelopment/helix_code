package helixqa

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dev.helix.code/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEngine_Disabled(t *testing.T) {
	cfg := &config.Config{QA: config.QAConfig{Enabled: false}}
	engine, err := NewEngine(cfg)
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.False(t, engine.Enabled())
}

func TestNewEngine_Enabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		QA: config.QAConfig{
			Enabled:   true,
			OutputDir: tmpDir,
			Platforms: []string{"web"},
			BanksDir:  tmpDir,
		},
		Logging: config.LoggingConfig{Level: "info"},
	}
	engine, err := NewEngine(cfg)
	require.NoError(t, err)
	assert.NotNil(t, engine)
	assert.True(t, engine.Enabled())
}

func TestEngine_StartSession(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a dummy bank file
	bankFile := filepath.Join(tmpDir, "test-bank.yaml")
	require.NoError(t, os.WriteFile(bankFile, []byte("test: true\n"), 0644))

	cfg := &config.Config{
		QA: config.QAConfig{
			Enabled:   true,
			OutputDir: tmpDir,
			Platforms: []string{"web"},
			BanksDir:  tmpDir,
		},
		Logging: config.LoggingConfig{Level: "info"},
	}
	engine, err := NewEngine(cfg)
	require.NoError(t, err)
	require.True(t, engine.Enabled())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	state, err := engine.StartSession(ctx, "test-session-1", []string{"web"}, []string{bankFile}, false)
	require.NoError(t, err)
	require.NotNil(t, state)
	// state.ID / state.Platforms / state.Banks are assigned in the struct
	// literal inside StartSession and never mutated, so they're safe to
	// read without locking. state.Status is mutated by the orchestrator
	// goroutine that StartSession launches BEFORE returning, so checking
	// it for "pending" here is a true data race — the goroutine can
	// advance it to "running" / "completed" before the test reads it.
	// Read mutable fields through state.Mu (the type's own lock).
	assert.Equal(t, "test-session-1", state.ID)
	assert.Equal(t, []string{"web"}, state.Platforms)
	assert.Equal(t, []string{bankFile}, state.Banks)
	state.Mu.RLock()
	status := state.Status
	state.Mu.RUnlock()
	// Status MUST be one of the known states. We do not pin it to
	// "pending" because the goroutine may have already advanced it.
	require.Contains(t, []string{"pending", "running", "completed", "failed", "cancelled"}, status)

	// Verify session is retrievable
	s, ok := engine.GetSession("test-session-1")
	require.True(t, ok)
	assert.Equal(t, state.ID, s.ID)

	// Wait for session to complete or fail (banks may not be valid helix_qa banks)
	done := make(chan struct{})
	go func() {
		for {
			s, _ := engine.GetSession("test-session-1")
			s.Mu.RLock()
			st := s.Status
			s.Mu.RUnlock()
			if st == "completed" || st == "failed" || st == "cancelled" {
				close(done)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	select {
	case <-done:
		// Session finished
	case <-ctx.Done():
		t.Fatal("session did not finish in time")
	}
}

func TestEngine_StartSession_InvalidBank(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		QA: config.QAConfig{
			Enabled:   true,
			OutputDir: tmpDir,
			Platforms: []string{"web"},
		},
		Logging: config.LoggingConfig{Level: "info"},
	}
	engine, err := NewEngine(cfg)
	require.NoError(t, err)

	_, err = engine.StartSession(context.Background(), "test-session-2", []string{"web"}, []string{"/nonexistent/bank.yaml"}, false)
	require.Error(t, err)
	// CONST-046 round-159: literal moved to i18n bundle; with the
	// default NoopTranslator the message ID echoes verbatim. See
	// translator_test.go for the paired-mutation sentinel-injection
	// proof that this path goes through tr().
	assert.Contains(t, err.Error(), "internal_helixqa_bank_not_found")
}

func TestEngine_CancelSession(t *testing.T) {
	tmpDir := t.TempDir()
	bankFile := filepath.Join(tmpDir, "test-bank.yaml")
	require.NoError(t, os.WriteFile(bankFile, []byte("test: true\n"), 0644))

	cfg := &config.Config{
		QA: config.QAConfig{
			Enabled:   true,
			OutputDir: tmpDir,
			Platforms: []string{"web"},
			BanksDir:  tmpDir,
		},
		Logging: config.LoggingConfig{Level: "info"},
	}
	engine, err := NewEngine(cfg)
	require.NoError(t, err)

	state, err := engine.StartSession(context.Background(), "test-session-3", []string{"web"}, []string{bankFile}, false)
	require.NoError(t, err)
	require.NotNil(t, state)

	err = engine.CancelSession("test-session-3")
	require.NoError(t, err)

	// Wait for the orchestrator goroutine to fully terminate so its final
	// Status write has been committed; reading Status before the goroutine
	// settles produced a race against CancelSession's own Status write.
	// Shutdown blocks on the per-engine WaitGroup.
	engine.Shutdown()

	s, ok := engine.GetSession("test-session-3")
	require.True(t, ok)
	s.Mu.RLock()
	status := s.Status
	s.Mu.RUnlock()
	assert.Equal(t, "cancelled", status)
}

func TestEngine_CancelSession_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		QA: config.QAConfig{
			Enabled:   true,
			OutputDir: tmpDir,
		},
		Logging: config.LoggingConfig{Level: "info"},
	}
	engine, err := NewEngine(cfg)
	require.NoError(t, err)

	err = engine.CancelSession("nonexistent")
	require.Error(t, err)
	// CONST-046 round-159: literal moved to i18n bundle; with the
	// default NoopTranslator the message ID echoes verbatim. See
	// translator_test.go for the paired-mutation sentinel-injection
	// proof that this path goes through tr().
	assert.Contains(t, err.Error(), "internal_helixqa_session_not_found")
}

func TestEngine_ListSessions(t *testing.T) {
	tmpDir := t.TempDir()
	bankFile := filepath.Join(tmpDir, "test-bank.yaml")
	require.NoError(t, os.WriteFile(bankFile, []byte("test: true\n"), 0644))

	cfg := &config.Config{
		QA: config.QAConfig{
			Enabled:   true,
			OutputDir: tmpDir,
			Platforms: []string{"web"},
			BanksDir:  tmpDir,
		},
		Logging: config.LoggingConfig{Level: "info"},
	}
	engine, err := NewEngine(cfg)
	require.NoError(t, err)

	// Initially empty
	sessions := engine.ListSessions()
	assert.Empty(t, sessions)

	_, err = engine.StartSession(context.Background(), "s1", []string{"web"}, []string{bankFile}, false)
	require.NoError(t, err)
	_, err = engine.StartSession(context.Background(), "s2", []string{"web"}, []string{bankFile}, false)
	require.NoError(t, err)

	sessions = engine.ListSessions()
	assert.Len(t, sessions, 2)
}

func TestBuildQAConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		QA: config.QAConfig{
			Enabled:       true,
			OutputDir:     tmpDir,
			Platforms:     []string{"web", "android"},
			BanksDir:      tmpDir,
			CoverageTarget: 0.95,
			ReportFormats: []string{"html"},
			Autonomous:    true,
			CuriosityEnabled: true,
			VisionProvider: "ollama",
			LLMProvider:   "openai",
			LLMAPIKey:     "test-key",
			RecordScreenshots: true,
			RecordVideo:   true,
		},
		Logging: config.LoggingConfig{Level: "debug"},
	}

	qaCfg, err := buildQAConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, tmpDir, qaCfg.OutputDir)
	assert.Len(t, qaCfg.Platforms, 2)
	assert.True(t, qaCfg.ValidateSteps)
	assert.True(t, qaCfg.Record)
	assert.True(t, qaCfg.Verbose)
	assert.Equal(t, 0.95, qaCfg.Autonomous.CoverageTarget)
	assert.True(t, qaCfg.Autonomous.CuriosityEnabled)
	assert.Equal(t, "ollama", qaCfg.Autonomous.VisionProvider)
	assert.Equal(t, "openai", qaCfg.Autonomous.LLMProvider)
	assert.Equal(t, "test-key", qaCfg.Autonomous.LLMAPIKey)
	assert.True(t, qaCfg.Autonomous.RecordingScreenshots)
	assert.True(t, qaCfg.Autonomous.RecordingVideo)
}
