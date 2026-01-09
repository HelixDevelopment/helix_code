# Notification Test Utilities

This package provides mock servers for testing notification integrations.

## Overview

The testutil package contains mock HTTP servers that simulate external notification services (Slack, Discord, etc.) for unit and integration testing without requiring actual service connections.

## Mock Servers

### MockSlackServer

Simulates Slack webhook endpoints:

```go
server := testutil.NewMockSlackServer()
defer server.Close()

// Use server.URL as your Slack webhook URL
slackNotifier := notification.NewSlackNotifier(server.URL)

// Send notification
slackNotifier.Send("Test message")

// Verify received requests
requests := server.GetRequests()
assert.Len(t, requests, 1)
assert.Equal(t, "Test message", requests[0].Text)
```

### MockDiscordServer

Simulates Discord webhook endpoints:

```go
server := testutil.NewMockDiscordServer()
defer server.Close()

discordNotifier := notification.NewDiscordNotifier(server.URL)
discordNotifier.Send("Test message")

requests := server.GetRequests()
assert.Equal(t, "Test message", requests[0].Content)
```

### MockEmailServer

Simulates SMTP server for email testing:

```go
server := testutil.NewMockEmailServer()
defer server.Close()

emailNotifier := notification.NewEmailNotifier(server.Host(), server.Port())
emailNotifier.Send("to@example.com", "Subject", "Body")

emails := server.GetEmails()
assert.Equal(t, "to@example.com", emails[0].To)
```

### MockTelegramServer

Simulates Telegram Bot API:

```go
server := testutil.NewMockTelegramServer()
defer server.Close()

telegramNotifier := notification.NewTelegramNotifier(server.URL, "bot-token", "chat-id")
telegramNotifier.Send("Test message")

messages := server.GetMessages()
assert.Equal(t, "Test message", messages[0].Text)
```

## Request Structures

### SlackRequest

```go
type SlackRequest struct {
    Channel   string `json:"channel"`
    Username  string `json:"username"`
    Text      string `json:"text"`
    IconEmoji string `json:"icon_emoji"`
}
```

### DiscordRequest

```go
type DiscordRequest struct {
    Content  string `json:"content"`
    Username string `json:"username"`
}
```

## Thread Safety

All mock servers are thread-safe and can handle concurrent requests:

```go
server := testutil.NewMockSlackServer()

var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(n int) {
        defer wg.Done()
        // Send concurrent notifications
    }(i)
}
wg.Wait()

// All requests are captured
assert.Len(t, server.GetRequests(), 10)
```

## Error Simulation

Configure mock servers to return errors:

```go
server := testutil.NewMockSlackServer()
server.SetErrorRate(0.5)  // 50% of requests will fail
server.SetErrorCode(500)  // Return 500 status code
```

## Usage in Tests

```go
func TestNotificationEngine(t *testing.T) {
    // Create mock servers
    slackServer := testutil.NewMockSlackServer()
    defer slackServer.Close()

    // Configure notification engine
    engine := notification.NewNotificationEngine()
    engine.AddChannel(notification.NewSlackNotifier(slackServer.URL))

    // Test notification
    err := engine.Notify(notification.Event{
        Type:    "task_complete",
        Message: "Task finished successfully",
    })

    assert.NoError(t, err)
    assert.Len(t, slackServer.GetRequests(), 1)
}
```

## Testing

```bash
go test -v ./internal/notification/testutil/...
```

## See Also

- `internal/notification/` - Main notification package
- `internal/notification/README.md` - Notification engine documentation
