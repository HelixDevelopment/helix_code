package server

import (
	"fmt"
	"net/http"
	"strconv"

	hqaScreenshot "digital.vasic.helixqa/pkg/screenshot"

	"github.com/gin-gonic/gin"
)

// captureScreenshot handles standalone screenshot capture requests.
func (s *Server) captureScreenshot(c *gin.Context) {
	platform := c.Param("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform is required"})
		return
	}

	// Parse optional parameters
	width, _ := strconv.Atoi(c.Query("width"))
	height, _ := strconv.Atoi(c.Query("height"))
	if width == 0 {
		width = 1280
	}
	if height == 0 {
		height = 720
	}

	opts := hqaScreenshot.CaptureOptions{
		Format:   c.DefaultQuery("format", "png"),
		Width:    width,
		Height:   height,
		FullPage: c.Query("full_page") == "true",
	}

	// Use the QA engine's evidence collector for screenshot capture
	if s.qaEngine == nil || !s.qaEngine.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QA engine is disabled"})
		return
	}

	result, err := s.qaEngine.CaptureScreenshot(c.Request.Context(), platform, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("screenshot capture failed: %v", err)})
		return
	}

	encode := c.Query("encode")
	if encode == "base64" {
		c.JSON(http.StatusOK, gin.H{
			"data":     result.Data,
			"format":   result.Format,
			"platform": result.Platform,
			"engine":   result.Engine,
			"size":     len(result.Data),
		})
		return
	}

	contentType := "image/png"
	if result.Format == "jpg" || result.Format == "jpeg" {
		contentType = "image/jpeg"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", len(result.Data)))
	c.Header("X-Screenshot-Engine", result.Engine)
	c.Header("X-Screenshot-Platform", string(result.Platform))
	c.Data(http.StatusOK, contentType, result.Data)
}

// listScreenshotEngines returns the list of supported screenshot engines.
func (s *Server) listScreenshotEngines(c *gin.Context) {
	if s.qaEngine == nil || !s.qaEngine.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QA engine is disabled"})
		return
	}

	engines := s.qaEngine.ListScreenshotEngines(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"engines": engines})
}
