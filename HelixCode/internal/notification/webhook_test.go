package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWebhookChannel(t *testing.T) {
	t.Run("create with URL enables channel", func(t *testing.T) {
		channel := NewWebhookChannel("https://example.com/webhook", nil)

		assert.NotNil(t, channel)
		assert.Equal(t, "webhook", channel.GetName())
		assert.True(t, channel.IsEnabled())
	})

	t.Run("empty URL disables channel", func(t *testing.T) {
		channel := NewWebhookChannel("", nil)

		assert.NotNil(t, channel)
		assert.False(t, channel.IsEnabled())
	})

	t.Run("with custom headers", func(t *testing.T) {
		headers := map[string]string{
			"Authorization": "Bearer token",
			"X-Custom":      "value",
		}
		channel := NewWebhookChannel("https://example.com/webhook", headers)

		assert.NotNil(t, channel)
		config := channel.GetConfig()
		assert.Equal(t, 2, config["headers"])
	})
}

func TestWebhookChannelSend(t *testing.T) {
	t.Run("send to disabled channel returns error", func(t *testing.T) {
		channel := NewWebhookChannel("", nil)

		notification := &Notification{
			Title:   "Test",
			Message: "Test message",
			Type:    NotificationTypeInfo,
		}

		err := channel.Send(context.Background(), notification)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "disabled")
	})

	t.Run("send successful to mock server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			assert.NoError(t, err)
			assert.Equal(t, "Test Title", payload["title"])

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		channel := NewWebhookChannel(server.URL, nil)

		notification := &Notification{
			Title:   "Test Title",
			Message: "Test message",
			Type:    NotificationTypeInfo,
		}

		err := channel.Send(context.Background(), notification)
		assert.NoError(t, err)
	})

	t.Run("send with custom headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		headers := map[string]string{"Authorization": "Bearer token"}
		channel := NewWebhookChannel(server.URL, headers)

		notification := &Notification{
			Title:   "Test",
			Message: "Test",
			Type:    NotificationTypeInfo,
		}

		err := channel.Send(context.Background(), notification)
		assert.NoError(t, err)
	})

	t.Run("send handles server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		channel := NewWebhookChannel(server.URL, nil)

		notification := &Notification{
			Title:   "Test",
			Message: "Test",
			Type:    NotificationTypeInfo,
		}

		err := channel.Send(context.Background(), notification)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status 500")
	})
}

func TestWebhookChannelGetConfig(t *testing.T) {
	headers := map[string]string{"X-Custom": "value"}
	channel := NewWebhookChannel("https://example.com/webhook", headers)

	config := channel.GetConfig()

	assert.Equal(t, "https://example.com/webhook", config["url"])
	assert.Equal(t, "POST", config["method"])
	assert.Equal(t, 1, config["headers"])
}

func TestNewTeamsChannel(t *testing.T) {
	t.Run("create with webhook enables channel", func(t *testing.T) {
		channel := NewTeamsChannel("https://outlook.office.com/webhook/xxx")

		assert.NotNil(t, channel)
		assert.Equal(t, "teams", channel.GetName())
		assert.True(t, channel.IsEnabled())
	})

	t.Run("empty webhook disables channel", func(t *testing.T) {
		channel := NewTeamsChannel("")

		assert.NotNil(t, channel)
		assert.False(t, channel.IsEnabled())
	})
}

func TestTeamsChannelSend(t *testing.T) {
	t.Run("send to disabled channel returns error", func(t *testing.T) {
		channel := NewTeamsChannel("")

		notification := &Notification{
			Title:   "Test",
			Message: "Test message",
			Type:    NotificationTypeInfo,
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
			assert.Equal(t, "MessageCard", payload["@type"])
			assert.Equal(t, "Test Title", payload["title"])

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		channel := NewTeamsChannel(server.URL)

		notification := &Notification{
			Title:   "Test Title",
			Message: "Test message",
			Type:    NotificationTypeInfo,
		}

		err := channel.Send(context.Background(), notification)
		assert.NoError(t, err)
	})

	t.Run("send handles server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		channel := NewTeamsChannel(server.URL)

		notification := &Notification{
			Title:   "Test",
			Message: "Test",
			Type:    NotificationTypeInfo,
		}

		err := channel.Send(context.Background(), notification)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status 400")
	})
}

func TestTeamsChannelGetConfig(t *testing.T) {
	channel := NewTeamsChannel("https://outlook.office.com/webhook/xxx")

	config := channel.GetConfig()

	assert.Equal(t, "https://outlook.office.com/webhook/xxx", config["webhook"])
}

func TestTeamsChannelColors(t *testing.T) {
	channel := NewTeamsChannel("https://example.com")

	// Test color for different notification types by sending to mock server
	// Actual colors from getColorForType implementation:
	// Success: "28a745", Warning: "ffc107", Error/Alert: "dc3545", Default: "0078d7"
	tests := []struct {
		notificationType NotificationType
		expectedColor    string
	}{
		{NotificationTypeInfo, "0078d7"},
		{NotificationTypeSuccess, "28a745"},
		{NotificationTypeWarning, "ffc107"},
		{NotificationTypeError, "dc3545"},
		{NotificationTypeAlert, "dc3545"},
	}

	for _, tt := range tests {
		t.Run(string(tt.notificationType), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]interface{}
				json.NewDecoder(r.Body).Decode(&payload)
				assert.Equal(t, tt.expectedColor, payload["themeColor"])
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			ch := NewTeamsChannel(server.URL)
			notification := &Notification{
				Title:   "Test",
				Message: "Test",
				Type:    tt.notificationType,
			}
			ch.Send(context.Background(), notification)
		})
	}

	// Just to verify the channel is not nil
	_ = channel
}
