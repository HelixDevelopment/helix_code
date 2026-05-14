package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDiscordChannel(t *testing.T) {
	tests := []struct {
		name        string
		webhook     string
		wantEnabled bool
	}{
		{
			name:        "valid webhook",
			webhook:     "https://discord.com/api/webhooks/123/abc",
			wantEnabled: true,
		},
		{
			name:        "empty webhook",
			webhook:     "",
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := NewDiscordChannel(tt.webhook)
			assert.NotNil(t, channel)
			assert.Equal(t, "discord", channel.GetName())
			assert.Equal(t, tt.wantEnabled, channel.IsEnabled())

			config := channel.GetConfig()
			assert.Equal(t, tt.webhook, config["webhook"])
		})
	}
}

func TestDiscordChannel_Send(t *testing.T) {
	tests := []struct {
		name           string
		notification   *Notification
		serverResponse int
		wantError      bool
		errorContains  string
		checkPayload   func(*testing.T, string)
	}{
		{
			name: "success - info notification",
			notification: &Notification{
				Title:   "Info",
				Message: "This is an info message",
				Type:    NotificationTypeInfo,
			},
			serverResponse: http.StatusNoContent,
			wantError:      false,
			checkPayload: func(t *testing.T, payload string) {
				assert.Contains(t, payload, "Info")
				assert.Contains(t, payload, "This is an info message")
			},
		},
		{
			name: "success - with 200 OK",
			notification: &Notification{
				Title:   "Test",
				Message: "Test message",
				Type:    NotificationTypeSuccess,
			},
			serverResponse: http.StatusOK,
			wantError:      false,
		},
		{
			name: "error - server error 500",
			notification: &Notification{
				Title:   "Test",
				Message: "Test",
				Type:    NotificationTypeError,
			},
			serverResponse: http.StatusInternalServerError,
			wantError:      true,
			errorContains:  "discord returned status 500",
		},
		{
			name: "error - bad request 400",
			notification: &Notification{
				Title:   "Test",
				Message: "Test",
				Type:    NotificationTypeWarning,
			},
			serverResponse: http.StatusBadRequest,
			wantError:      true,
			errorContains:  "discord returned status 400",
		},
		{
			name: "success - with special characters",
			notification: &Notification{
				Title:   "Error: Failed!",
				Message: "Error occurred: Connection timeout @ 10:30 AM",
				Type:    NotificationTypeError,
			},
			serverResponse: http.StatusNoContent,
			wantError:      false,
			checkPayload: func(t *testing.T, payload string) {
				assert.Contains(t, payload, "Error: Failed!")
				assert.Contains(t, payload, "Connection timeout")
			},
		},
		{
			name: "success - with emoji",
			notification: &Notification{
				Title:   "Success! 🎉",
				Message: "Deployment completed ✅",
				Type:    NotificationTypeSuccess,
			},
			serverResponse: http.StatusNoContent,
			wantError:      false,
		},
		{
			name: "success - multiline message",
			notification: &Notification{
				Title:   "Build Report",
				Message: "Line 1\nLine 2\nLine 3",
				Type:    NotificationTypeInfo,
			},
			serverResponse: http.StatusNoContent,
			wantError:      false,
			checkPayload: func(t *testing.T, payload string) {
				assert.Contains(t, payload, "Build Report")
				assert.Contains(t, payload, "Line 1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedPayload string

			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Read payload
				buf := make([]byte, 1024)
				n, _ := r.Body.Read(buf)
				receivedPayload = string(buf[:n])

				w.WriteHeader(tt.serverResponse)
			}))
			defer server.Close()

			channel := NewDiscordChannel(server.URL)
			err := channel.Send(context.Background(), tt.notification)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.checkPayload != nil {
					tt.checkPayload(t, receivedPayload)
				}
			}
		})
	}
}

func TestDiscordChannel_Send_Disabled(t *testing.T) {
	channel := NewDiscordChannel("")

	notification := &Notification{
		Title:   "Test",
		Message: "Test",
		Type:    NotificationTypeInfo,
	}

	err := channel.Send(context.Background(), notification)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestDiscordChannel_Send_NetworkError(t *testing.T) {
	// Create channel with invalid URL to simulate network error
	channel := NewDiscordChannel("http://invalid-url-that-does-not-exist-12345.com")

	notification := &Notification{
		Title:   "Test",
		Message: "Test",
		Type:    NotificationTypeInfo,
	}

	err := channel.Send(context.Background(), notification)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send to discord")
}

func TestDiscordChannel_Send_AllNotificationTypes(t *testing.T) {
	notificationTypes := []NotificationType{
		NotificationTypeInfo,
		NotificationTypeSuccess,
		NotificationTypeWarning,
		NotificationTypeError,
		NotificationTypeAlert,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	channel := NewDiscordChannel(server.URL)

	for _, notifType := range notificationTypes {
		t.Run(string(notifType), func(t *testing.T) {
			notification := &Notification{
				Title:   "Test " + string(notifType),
				Message: "Testing " + string(notifType),
				Type:    notifType,
			}

			err := channel.Send(context.Background(), notification)
			assert.NoError(t, err)
		})
	}
}

func TestDiscordChannel_Send_WithMetadata(t *testing.T) {
	var receivedPayload string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 2048)
		n, _ := r.Body.Read(buf)
		receivedPayload = string(buf[:n])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	channel := NewDiscordChannel(server.URL)

	notification := &Notification{
		Title:   "Build Failed",
		Message: "The build failed with errors",
		Type:    NotificationTypeError,
		Metadata: map[string]interface{}{
			"build_id": "12345",
			"branch":   "main",
			"commit":   "abc123",
		},
	}

	err := channel.Send(context.Background(), notification)
	require.NoError(t, err)

	// Verify payload contains title and message
	assert.Contains(t, receivedPayload, "Build Failed")
	assert.Contains(t, receivedPayload, "The build failed with errors")
}

func TestDiscordChannel_GetConfig(t *testing.T) {
	webhook := "https://discord.com/api/webhooks/test"
	channel := NewDiscordChannel(webhook)

	config := channel.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, webhook, config["webhook"])
}

func TestDiscordChannel_GetName(t *testing.T) {
	channel := NewDiscordChannel("https://discord.com/api/webhooks/test")
	assert.Equal(t, "discord", channel.GetName())
}

func TestDiscordChannel_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		webhook string
		want    bool
	}{
		{
			name:    "enabled with valid webhook",
			webhook: "https://discord.com/api/webhooks/123/abc",
			want:    true,
		},
		{
			name:    "disabled with empty webhook",
			webhook: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := NewDiscordChannel(tt.webhook)
			assert.Equal(t, tt.want, channel.IsEnabled())
		})
	}
}

func TestDiscordChannel_Send_ConcurrentRequests(t *testing.T) {
	// httptest.Server invokes the handler on a per-connection goroutine, so the
	// closure below runs concurrently. The counter is mutated under -race; use
	// atomic.Int64 to serialise the read-modify-write without serialising the
	// requests themselves (which is what this test is actually exercising).
	var requestCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	channel := NewDiscordChannel(server.URL)

	done := make(chan bool)
	count := 10

	for i := 0; i < count; i++ {
		go func(id int) {
			notification := &Notification{
				Title:   "Concurrent Test",
				Message: "Message",
				Type:    NotificationTypeInfo,
			}
			err := channel.Send(context.Background(), notification)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < count; i++ {
		<-done
	}

	assert.Equal(t, int64(count), requestCount.Load())
}

func TestDiscordChannel_Send_ContextCancellation(t *testing.T) {
	// Anti-bluff (CONST-035 / §11.9): originally `_ = err`; round-40
	// first tightening pinned the broken behaviour (http.Post ignored
	// ctx); this second tightening pins the CORRECT behaviour after
	// engine.go was promoted to http.NewRequestWithContext +
	// DefaultClient.Do. A cancelled ctx MUST now surface a context
	// cancellation error, so callers waiting on a long-running webhook
	// can abort cleanly.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	channel := NewDiscordChannel(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	notification := &Notification{
		Title:   "Test",
		Message: "Test",
		Type:    NotificationTypeInfo,
	}

	err := channel.Send(ctx, notification)
	require.Error(t, err, "Send with a pre-cancelled ctx must surface a context-cancellation error")
	assert.ErrorIs(t, err, context.Canceled,
		"the cancellation error must wrap context.Canceled (proves http.NewRequestWithContext path is wired)")
}

func TestDiscordChannel_Send_LargePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	channel := NewDiscordChannel(server.URL)

	// Create a large message
	largeMessage := ""
	for i := 0; i < 1000; i++ {
		largeMessage += "This is a test message. "
	}

	notification := &Notification{
		Title:   "Large Payload Test",
		Message: largeMessage,
		Type:    NotificationTypeInfo,
	}

	err := channel.Send(context.Background(), notification)
	assert.NoError(t, err)
}
