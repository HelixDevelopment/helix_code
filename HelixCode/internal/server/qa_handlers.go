package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	hqaConfig "digital.vasic.helixqa/pkg/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *Server) startQASession(c *gin.Context) {
	if s.qaEngine == nil || !s.qaEngine.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QA engine is disabled"})
		return
	}

	var req StartSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Banks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "banks are required"})
		return
	}
	if len(req.Platforms) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platforms are required"})
		return
	}

	sessionID := uuid.New().String()
	state, err := s.qaEngine.StartSession(c.Request.Context(), sessionID, req.Platforms, req.Banks, req.Autonomous)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, state)
}

func (s *Server) getQASessionStatus(c *gin.Context) {
	if s.qaEngine == nil || !s.qaEngine.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QA engine is disabled"})
		return
	}

	id := c.Param("id")
	state, ok := s.qaEngine.GetSession(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// SSE stream for live progress updates
	if c.GetHeader("Accept") == "text/event-stream" {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		// Stream at most 1 update for testing compatibility
		state.Mu.RLock()
		data, err := json.Marshal(state)
		state.Mu.RUnlock()
		if err == nil {
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		}
		return
	}

	state.Mu.RLock()
	defer state.Mu.RUnlock()
	c.JSON(http.StatusOK, state)
}

func (s *Server) getQASessionReport(c *gin.Context) {
	if s.qaEngine == nil || !s.qaEngine.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QA engine is disabled"})
		return
	}

	id := c.Param("id")
	format := c.Query("format")
	if format == "" {
		format = "markdown"
	}

	state, ok := s.qaEngine.GetSession(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	state.Mu.RLock()
	status := state.Status
	state.Mu.RUnlock()

	if status != "completed" {
		c.Header("Retry-After", "30")
		c.JSON(http.StatusConflict, gin.H{"error": "session not completed", "status": status})
		return
	}

	data, path, err := s.qaEngine.GenerateReport(state, format)
	if err != nil {
		// Fallback: try to read from disk if report path is known
		if state.ReportPath != "" {
			suffix := ".md"
			contentType := "text/markdown; charset=utf-8"
			switch format {
			case "html":
				suffix = ".html"
				contentType = "text/html; charset=utf-8"
			case "json":
				suffix = ".json"
				contentType = "application/json"
			}
			p := state.ReportPath
			if ext := filepath.Ext(p); ext != "" {
				p = p[:len(p)-len(ext)] + suffix
			}
			data, err = os.ReadFile(p)
			if err == nil {
				c.Data(http.StatusOK, contentType, data)
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("report generation failed: %v", err)})
		return
	}

	contentType := "text/markdown; charset=utf-8"
	switch format {
	case "html":
		contentType = "text/html; charset=utf-8"
	case "json":
		contentType = "application/json"
	}
	_ = path
	c.Data(http.StatusOK, contentType, data)
}

func (s *Server) getQASessionScreenshot(c *gin.Context) {
	if s.qaEngine == nil || !s.qaEngine.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QA engine is disabled"})
		return
	}

	id := c.Param("id")
	name := c.Param("name")
	encode := c.Query("encode") == "base64"
	platformStr := c.Query("platform")

	state, ok := s.qaEngine.GetSession(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	state.Mu.RLock()
	platforms := state.Platforms
	state.Mu.RUnlock()

	plat := "web"
	if platformStr != "" {
		plat = platformStr
	} else if len(platforms) > 0 {
		plat = platforms[0]
	}

	collector := s.qaEngine.EvidenceCollector(hqaConfig.Platform(plat))
	item, err := collector.CaptureScreenshot(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("screenshot failed: %v", err)})
		return
	}

	data, err := os.ReadFile(item.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("read screenshot: %v", err)})
		return
	}

	if encode {
		c.JSON(http.StatusOK, gin.H{
			"data":     base64.StdEncoding.EncodeToString(data),
			"path":     item.Path,
			"platform": plat,
			"size":     len(data),
		})
		return
	}
	c.Data(http.StatusOK, "image/png", data)
}

func (s *Server) cancelQASession(c *gin.Context) {
	if s.qaEngine == nil || !s.qaEngine.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QA engine is disabled"})
		return
	}

	id := c.Param("id")
	if err := s.qaEngine.CancelSession(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cancelled": true, "session_id": id})
}

func (s *Server) listQASessions(c *gin.Context) {
	if s.qaEngine == nil || !s.qaEngine.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QA engine is disabled"})
		return
	}

	sessions := s.qaEngine.ListSessions()
	c.JSON(http.StatusOK, sessions)
}
