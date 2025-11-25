# Notification Package

The `notification` package provides multi-channel notification support for the HelixCode platform.

## Overview

This package handles:
- Multi-channel notifications (Slack, Discord, Email, Telegram)
- Notification templates
- Delivery tracking
- Rate limiting
- Notification rules

## Supported Channels

- **Slack** - Webhook-based messages
- **Discord** - Webhook-based messages
- **Email** - SMTP-based emails
- **Telegram** - Bot API messages
- **Webhook** - Generic HTTP webhooks

## Key Types

### NotificationService

The main notification service:

```go
type NotificationService struct {
    channels map[string]Channel
    rules    []*Rule
    config   *Config
    logger   *logging.Logger
}
```

### Channel

```go
type Channel interface {
    Send(ctx context.Context, notification *Notification) error
    Type() ChannelType
    Name() string
}
```

### Notification

```go
type Notification struct {
    ID        string
    Type      NotificationType
    Title     string
    Message   string
    Priority  Priority
    Metadata  map[string]interface{}
    CreatedAt time.Time
}
```

## Usage

### Creating the Service

```go
import "dev.helix.code/internal/notification"

config := &notification.Config{
    Channels: map[string]*notification.ChannelConfig{
        "slack": {
            Type:       notification.ChannelSlack,
            WebhookURL: os.Getenv("SLACK_WEBHOOK_URL"),
            Enabled:    true,
        },
        "email": {
            Type:     notification.ChannelEmail,
            SMTPHost: "smtp.gmail.com",
            SMTPPort: 587,
            Username: os.Getenv("EMAIL_USERNAME"),
            Password: os.Getenv("EMAIL_PASSWORD"),
            From:     "noreply@helixcode.dev",
            Enabled:  true,
        },
    },
}

service := notification.NewService(config)
```

### Sending Notifications

```go
// Simple notification
notif := &notification.Notification{
    Type:     notification.TypeTaskComplete,
    Title:    "Task Completed",
    Message:  "Build task completed successfully",
    Priority: notification.PriorityNormal,
}

err := service.Send(ctx, notif)
```

### Sending to Specific Channels

```go
// Send to Slack only
err := service.SendToChannel(ctx, "slack", notif)

// Send to multiple channels
err := service.SendToChannels(ctx, []string{"slack", "email"}, notif)
```

### Notification Rules

```go
// Create rule to route notifications
rule := &notification.Rule{
    Name:      "critical-alerts",
    Condition: "priority == 'critical'",
    Channels:  []string{"slack", "email", "telegram"},
    Enabled:   true,
}

service.AddRule(rule)

// Notifications matching rules are auto-routed
```

### Templates

```go
// Register template
template := &notification.Template{
    Name:    "task_complete",
    Title:   "Task {{.TaskID}} Completed",
    Message: "Task {{.TaskName}} finished with status: {{.Status}}",
}
service.RegisterTemplate(template)

// Use template
notif, err := service.CreateFromTemplate("task_complete", map[string]interface{}{
    "TaskID":   "task-123",
    "TaskName": "Build",
    "Status":   "success",
})
service.Send(ctx, notif)
```

## Notification Types

```go
type NotificationType string

const (
    TypeTaskComplete   NotificationType = "task_complete"
    TypeTaskFailed     NotificationType = "task_failed"
    TypeWorkerOffline  NotificationType = "worker_offline"
    TypeSystemAlert    NotificationType = "system_alert"
    TypeDeployComplete NotificationType = "deploy_complete"
    TypeBuildComplete  NotificationType = "build_complete"
)
```

## Configuration

```yaml
notifications:
  enabled: true
  default_channel: "slack"
  channels:
    slack:
      enabled: true
      webhook_url: "${SLACK_WEBHOOK_URL}"

    discord:
      enabled: true
      webhook_url: "${DISCORD_WEBHOOK_URL}"

    telegram:
      enabled: true
      bot_token: "${TELEGRAM_BOT_TOKEN}"
      chat_id: "${TELEGRAM_CHAT_ID}"

    email:
      enabled: true
      smtp_host: "smtp.gmail.com"
      smtp_port: 587
      username: "${EMAIL_USERNAME}"
      password: "${EMAIL_PASSWORD}"
      from: "noreply@helixcode.dev"

  rules:
    - name: critical-alerts
      condition: "priority == 'critical'"
      channels: ["slack", "email"]

    - name: task-updates
      condition: "type starts_with 'task_'"
      channels: ["slack"]
```

## Rate Limiting

```go
config := &notification.Config{
    RateLimit: &notification.RateLimitConfig{
        Enabled:       true,
        MaxPerMinute:  60,
        MaxPerHour:    500,
        BurstSize:     10,
    },
}
```

## Testing

```bash
go test -v ./internal/notification/...
```

## Notes

- Use environment variables for secrets
- Configure rate limits to prevent spam
- Set up rules for automatic routing
- Monitor delivery status for reliability
