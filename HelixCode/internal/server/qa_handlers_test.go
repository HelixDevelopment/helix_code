package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/helixqa"
	"dev.helix.code/internal/redis"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupQATestServer(t *testing.T) (*Server, *httptest.ResponseRecorder, *gin.Context, string) {
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	bankFile := filepath.Join(tmpDir, "test-bank.yaml")
	require.NoError(t, os.WriteFile(bankFile, []byte("test: true\n"), 0644))

	cfg := &config.Config{
		Server: config.ServerConfig{Address: "0.0.0.0", Port: 8080},
		Auth: config.AuthConfig{
			JWTSecret:     "test-secret",
			TokenExpiry:   3600,
			SessionExpiry: 86400,
		},
		QA: config.QAConfig{
			Enabled:   true,
			OutputDir: tmpDir,
			Platforms: []string{"web"},
			BanksDir:  tmpDir,
		},
		Logging: config.LoggingConfig{Level: "info"},
	}

	// Create a mock database and redis
	db := &database.Database{}
	rds, err := redis.NewClient(&config.RedisConfig{Host: "", Port: 0, Password: ""})
	require.NoError(t, err)

	server := New(cfg, db, rds)
	require.NotNil(t, server)
	require.NotNil(t, server.qaEngine)
	require.True(t, server.qaEngine.Enabled())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return server, w, c, bankFile
}

func TestStartQASession_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Server: config.ServerConfig{Address: "0.0.0.0", Port: 8080},
		Auth:   config.AuthConfig{JWTSecret: "test-secret", TokenExpiry: 3600},
		QA:     config.QAConfig{Enabled: false},
	}
	server := New(cfg, nil, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	server.startQASession(c)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "disabled")
}

func TestStartQASession_InvalidJSON(t *testing.T) {
	server, w, c, _ := setupQATestServer(t)
	c.Request, _ = http.NewRequest("POST", "/api/v1/qa/session", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	server.startQASession(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStartQASession_MissingBanks(t *testing.T) {
	server, w, c, _ := setupQATestServer(t)
	req := StartSessionRequest{Platforms: []string{"web"}}
	body, _ := json.Marshal(req)
	c.Request, _ = http.NewRequest("POST", "/api/v1/qa/session", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	server.startQASession(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetQASessionStatus_NotFound(t *testing.T) {
	server, w, c, _ := setupQATestServer(t)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	server.getQASessionStatus(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCancelQASession_NotFound(t *testing.T) {
	server, w, c, _ := setupQATestServer(t)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	server.cancelQASession(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListQASessions(t *testing.T) {
	server, w, c, _ := setupQATestServer(t)

	server.listQASessions(c)
	assert.Equal(t, http.StatusOK, w.Code)

	var sessions []*helixqa.SessionState
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sessions))
	assert.Empty(t, sessions)
}

func TestGetQASessionReport_NotCompleted(t *testing.T) {
	server, w, c, _ := setupQATestServer(t)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	server.getQASessionReport(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestStartQASession_Success(t *testing.T) {
	server, w, c, bankFile := setupQATestServer(t)
	req := StartSessionRequest{
		Platforms: []string{"web"},
		Banks:     []string{bankFile},
	}
	body, _ := json.Marshal(req)
	c.Request, _ = http.NewRequest("POST", "/api/v1/qa/session", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	server.startQASession(c)
	assert.Equal(t, http.StatusCreated, w.Code)

	var state helixqa.SessionState
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &state))
	assert.NotEmpty(t, state.ID)
	assert.Equal(t, "pending", state.Status)
	assert.Contains(t, state.Platforms, "web")
}

func TestGetQASessionStatus_AfterCreation(t *testing.T) {
	server, w, c, bankFile := setupQATestServer(t)
	req := StartSessionRequest{
		Platforms: []string{"web"},
		Banks:     []string{bankFile},
	}
	body, _ := json.Marshal(req)
	c.Request, _ = http.NewRequest("POST", "/api/v1/qa/session", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	server.startQASession(c)

	var created helixqa.SessionState
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Params = gin.Params{{Key: "id", Value: created.ID}}
	server.getQASessionStatus(c2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var state helixqa.SessionState
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &state))
	assert.Equal(t, created.ID, state.ID)
}

func TestListQASessions_WithData(t *testing.T) {
	server, _, c, bankFile := setupQATestServer(t)
	// Create two sessions
	for i := 0; i < 2; i++ {
		req := StartSessionRequest{
			Platforms: []string{"web"},
			Banks:     []string{bankFile},
		}
		body, _ := json.Marshal(req)
		c.Request, _ = http.NewRequest("POST", "/api/v1/qa/session", bytes.NewBuffer(body))
		c.Request.Header.Set("Content-Type", "application/json")
		server.startQASession(c)
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	server.listQASessions(c2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var sessions []*helixqa.SessionState
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &sessions))
	assert.Len(t, sessions, 2)
}

func TestCancelQASession_Existing(t *testing.T) {
	server, w, c, bankFile := setupQATestServer(t)
	req := StartSessionRequest{
		Platforms: []string{"web"},
		Banks:     []string{bankFile},
	}
	body, _ := json.Marshal(req)
	c.Request, _ = http.NewRequest("POST", "/api/v1/qa/session", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	server.startQASession(c)

	var created helixqa.SessionState
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Params = gin.Params{{Key: "id", Value: created.ID}}
	server.cancelQASession(c2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	assert.True(t, resp["cancelled"].(bool))
	assert.Equal(t, created.ID, resp["session_id"])
}

func TestGetQASessionReport_SessionNotCompleted(t *testing.T) {
	server, w, c, bankFile := setupQATestServer(t)
	req := StartSessionRequest{
		Platforms: []string{"web"},
		Banks:     []string{bankFile},
	}
	body, _ := json.Marshal(req)
	c.Request, _ = http.NewRequest("POST", "/api/v1/qa/session", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	server.startQASession(c)

	var created helixqa.SessionState
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Params = gin.Params{{Key: "id", Value: created.ID}}
	server.getQASessionReport(c2)

	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestGetQASessionScreenshot_NotFound(t *testing.T) {
	server, w, c, _ := setupQATestServer(t)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	server.getQASessionScreenshot(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
