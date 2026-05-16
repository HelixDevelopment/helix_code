package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"dev.helix.code/internal/helixqa"
)

// Client provides a REST client for the HelixCode server API.
type Client struct {
	baseURL string
	client  *http.Client
	token   string
}

// NewClient creates a new server client.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// SetAuthToken sets the Bearer token for authenticated requests.
func (c *Client) SetAuthToken(token string) {
	c.token = token
}

// StartSessionRequest is the request body for starting a QA session.
type StartSessionRequest struct {
	Platforms        []string `json:"platforms"`
	Banks            []string `json:"banks"`
	Autonomous       bool     `json:"autonomous"`
	CoverageTarget   float64  `json:"coverage_target"`
	CuriosityEnabled bool     `json:"curiosity_enabled"`
}

// StartQASession starts a new QA session via the REST API.
func (c *Client) StartQASession(req StartSessionRequest) (*helixqa.SessionState, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest("POST", c.baseURL+"/api/v1/qa/session", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	var state helixqa.SessionState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

// GetQASession retrieves a QA session status.
func (c *Client) GetQASession(id string) (*helixqa.SessionState, error) {
	httpReq, err := http.NewRequest("GET", c.baseURL+"/api/v1/qa/session/"+id+"/status", nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	var state helixqa.SessionState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

// GetReport retrieves a completed QA session report.
func (c *Client) GetReport(sessionID, format string) ([]byte, error) {
	if format == "" {
		format = "markdown"
	}
	httpReq, err := http.NewRequest("GET", c.baseURL+"/api/v1/qa/session/"+sessionID+"/report?format="+format, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// CaptureScreenshot requests an on-demand screenshot.
func (c *Client) CaptureScreenshot(sessionID, platform string, base64 bool) ([]byte, map[string]string, error) {
	url := c.baseURL + "/api/v1/qa/session/" + sessionID + "/screenshot/latest"
	q := ""
	if platform != "" {
		q += "?platform=" + platform
	}
	if base64 {
		sep := "?"
		if q != "" {
			sep = "&"
		}
		q += sep + "encode=base64"
	}
	httpReq, err := http.NewRequest("GET", url+q, nil)
	if err != nil {
		return nil, nil, err
	}
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	meta := map[string]string{
		"platform": platform,
		"format":   resp.Header.Get("Content-Type"),
	}
	return data, meta, nil
}

// ListQASessions retrieves all QA sessions.
func (c *Client) ListQASessions() ([]*helixqa.SessionState, error) {
	httpReq, err := http.NewRequest("GET", c.baseURL+"/api/v1/qa/sessions", nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	var sessions []*helixqa.SessionState
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// CancelQASession cancels a running QA session.
func (c *Client) CancelQASession(sessionID string) error {
	httpReq, err := http.NewRequest("DELETE", c.baseURL+"/api/v1/qa/session/"+sessionID, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// WaitForSession polls a session until it completes or fails.
func (c *Client) WaitForSession(sessionID string, out io.Writer) error {
	for {
		state, err := c.GetQASession(sessionID)
		if err != nil {
			return err
		}
		if out != nil {
			fmt.Fprintf(out, "Session %s: status=%s phase=%s progress=%.0f%%\n",
				sessionID, state.Status, state.Phase, state.PhaseProgress*100)
		}
		switch state.Status {
		case "completed":
			return nil
		case "failed", "cancelled":
			return fmt.Errorf("session %s ended with status: %s", sessionID, state.Status)
		}
		time.Sleep(2 * time.Second)
	}
}
