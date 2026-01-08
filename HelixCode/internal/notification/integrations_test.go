package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPagerDutyChannel(t *testing.T) {
	t.Run("create with integration key enables channel", func(t *testing.T) {
		channel := NewPagerDutyChannel("test-integration-key")

		assert.NotNil(t, channel)
		assert.Equal(t, "pagerduty", channel.GetName())
		assert.True(t, channel.IsEnabled())
	})

	t.Run("empty integration key disables channel", func(t *testing.T) {
		channel := NewPagerDutyChannel("")

		assert.NotNil(t, channel)
		assert.False(t, channel.IsEnabled())
	})
}

func TestPagerDutyChannelSend(t *testing.T) {
	t.Run("send to disabled channel returns error", func(t *testing.T) {
		channel := NewPagerDutyChannel("")

		notification := &Notification{
			Title:   "Test",
			Message: "Test message",
			Type:    NotificationTypeAlert,
		}

		err := channel.Send(context.Background(), notification)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "disabled")
	})

	t.Run("send successful to mock server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)

			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			assert.NoError(t, err)
			assert.Equal(t, "trigger", payload["event_action"])

			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"status":"success"}`))
		}))
		defer server.Close()

		channel := NewPagerDutyChannel("test-key")
		// Override the URL to use mock server
		channel.apiURL = server.URL

		notification := &Notification{
			Title:   "Test Alert",
			Message: "Test message",
			Type:    NotificationTypeAlert,
		}

		err := channel.Send(context.Background(), notification)
		assert.NoError(t, err)
	})
}

func TestPagerDutyChannelGetConfig(t *testing.T) {
	channel := NewPagerDutyChannel("my-routing-key")

	config := channel.GetConfig()

	assert.NotEmpty(t, config["integration_key"])
	assert.Equal(t, "https://events.pagerduty.com/v2/enqueue", config["api_url"])
}

func TestPagerDutyGetSeverity(t *testing.T) {
	channel := NewPagerDutyChannel("key")

	tests := []struct {
		notificationType NotificationType
		expectedSeverity string
	}{
		{NotificationTypeInfo, "info"},
		{NotificationTypeSuccess, "info"},
		{NotificationTypeWarning, "warning"},
		{NotificationTypeError, "error"},
		{NotificationTypeAlert, "critical"},
	}

	for _, tt := range tests {
		t.Run(string(tt.notificationType), func(t *testing.T) {
			severity := channel.getSeverity(tt.notificationType)
			assert.Equal(t, tt.expectedSeverity, severity)
		})
	}
}

func TestNewJiraChannel(t *testing.T) {
	t.Run("create with valid config enables channel", func(t *testing.T) {
		channel := NewJiraChannel("https://jira.example.com", "user@example.com", "api-token", "PROJ")

		assert.NotNil(t, channel)
		assert.Equal(t, "jira", channel.GetName())
		assert.True(t, channel.IsEnabled())
	})

	t.Run("empty URL disables channel", func(t *testing.T) {
		channel := NewJiraChannel("", "user", "token", "PROJ")

		assert.NotNil(t, channel)
		assert.False(t, channel.IsEnabled())
	})
}

func TestJiraChannelSend(t *testing.T) {
	t.Run("send to disabled channel returns error", func(t *testing.T) {
		channel := NewJiraChannel("", "user", "token", "PROJ")

		notification := &Notification{
			Title:   "Test",
			Message: "Test message",
			Type:    NotificationTypeError,
		}

		err := channel.Send(context.Background(), notification)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "disabled")
	})

	t.Run("send successful to mock server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/rest/api/3/issue")

			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			assert.NoError(t, err)

			fields := payload["fields"].(map[string]interface{})
			assert.Equal(t, "Test Issue", fields["summary"])

			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":"10001","key":"PROJ-1"}`))
		}))
		defer server.Close()

		channel := NewJiraChannel(server.URL, "user@example.com", "token", "PROJ")

		notification := &Notification{
			Title:   "Test Issue",
			Message: "Test description",
			Type:    NotificationTypeError,
		}

		err := channel.Send(context.Background(), notification)
		assert.NoError(t, err)
	})
}

func TestJiraChannelGetConfig(t *testing.T) {
	channel := NewJiraChannel("https://jira.example.com", "user@example.com", "token", "PROJ")

	config := channel.GetConfig()

	assert.Equal(t, "https://jira.example.com", config["base_url"])
	assert.Equal(t, "PROJ", config["project_key"])
}

func TestJiraGetPriority(t *testing.T) {
	channel := NewJiraChannel("https://jira.example.com", "user", "token", "PROJ")

	tests := []struct {
		priority         NotificationPriority
		expectedJiraPrio string
	}{
		{NotificationPriorityLow, "Low"},
		{NotificationPriorityMedium, "Medium"},
		{NotificationPriorityHigh, "High"},
		{NotificationPriorityUrgent, "Highest"},
	}

	for _, tt := range tests {
		t.Run(string(tt.priority), func(t *testing.T) {
			jiraPrio := channel.getPriority(tt.priority)
			assert.Equal(t, tt.expectedJiraPrio, jiraPrio)
		})
	}
}

func TestNewGitHubIssuesChannel(t *testing.T) {
	t.Run("create with valid config enables channel", func(t *testing.T) {
		channel := NewGitHubIssuesChannel("ghp_token123", "owner", "repo")

		assert.NotNil(t, channel)
		assert.Equal(t, "github", channel.GetName())
		assert.True(t, channel.IsEnabled())
	})

	t.Run("empty token disables channel", func(t *testing.T) {
		channel := NewGitHubIssuesChannel("", "owner", "repo")

		assert.NotNil(t, channel)
		assert.False(t, channel.IsEnabled())
	})
}

func TestGitHubIssuesChannelSend(t *testing.T) {
	t.Run("send to disabled channel returns error", func(t *testing.T) {
		channel := NewGitHubIssuesChannel("", "owner", "repo")

		notification := &Notification{
			Title:   "Test",
			Message: "Test message",
			Type:    NotificationTypeError,
		}

		err := channel.Send(context.Background(), notification)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "disabled")
	})

	t.Run("send successful to mock server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "Bearer ghp_token123", r.Header.Get("Authorization"))

			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			assert.NoError(t, err)
			assert.Equal(t, "Bug Report", payload["title"])

			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":1,"number":123}`))
		}))
		defer server.Close()

		channel := NewGitHubIssuesChannel("ghp_token123", "owner", "repo")
		// Override API URL
		channel.apiURL = server.URL

		notification := &Notification{
			Title:   "Bug Report",
			Message: "Something went wrong",
			Type:    NotificationTypeError,
		}

		err := channel.Send(context.Background(), notification)
		assert.NoError(t, err)
	})
}

func TestGitHubIssuesChannelGetConfig(t *testing.T) {
	channel := NewGitHubIssuesChannel("ghp_secret", "myorg", "myrepo")

	config := channel.GetConfig()

	assert.Equal(t, "myorg", config["owner"])
	assert.Equal(t, "myrepo", config["repo"])
}
