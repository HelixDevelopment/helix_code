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
	assert.Equal(t, "test-session-1", state.ID)
	assert.Equal(t, "pending", state.Status)
	assert.Equal(t, []string{"web"}, state.Platforms)
	assert.Equal(t, []string{bankFile}, state.Banks)

	// Verify session is retrievable
	s, ok := engine.GetSession("test-session-1")
	require.True(t, ok)
	assert.Equal(t, state.ID, s.ID)

	// Wait for session to complete or fail (banks may not be valid HelixQA banks)
	done := make(chan struct{})
	go func() {
		for {
			s, _ := engine.GetSession("test-session-1")
			if s.Status == "completed" || s.Status == "failed" || s.Status == "cancelled" {
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
	assert.Contains(t, err.Error(), "bank not found")
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

	s, ok := engine.GetSession("test-session-3")
	require.True(t, ok)
	assert.Equal(t, "cancelled", s.Status)
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
	assert.Contains(t, err.Error(), "not found")
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
