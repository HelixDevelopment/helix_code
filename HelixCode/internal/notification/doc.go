// Package notification provides a multi-channel notification system for HelixCode.
//
// This package implements a comprehensive notification engine that supports multiple
// communication channels and provides enterprise-grade features such as rate limiting,
// retry logic with exponential backoff, queueing, and event-based notification triggers.
//
// # Architecture
//
// The notification system is built around several core components:
//
//   - NotificationEngine: Central hub for managing channels, rules, and templates
//   - NotificationChannel: Interface for implementing various notification channels
//   - NotificationQueue: Worker-based queue for async notification delivery
//   - RateLimiter: Token bucket rate limiting for channel protection
//   - RetryableChannel: Automatic retry with exponential backoff
//   - EventNotificationHandler: Integration with the event bus for automated notifications
//   - Metrics: Comprehensive tracking of notification performance
//
// # Supported Channels
//
// The package includes built-in implementations for common notification channels:
//
//   - Slack: Webhook-based Slack notifications with custom emoji support
//   - Discord: Discord webhook integration
//   - Telegram: Telegram Bot API with HTML formatting
//   - Email: SMTP-based email with multiple recipients support
//   - Microsoft Teams: Teams webhook with Adaptive Card formatting
//   - PagerDuty: Event API v2 for incident management
//   - Jira: Issue creation for task tracking
//   - GitHub Issues: Automated issue creation
//   - Yandex Messenger: Enterprise messaging support
//   - Max: Enterprise communication platform integration
//   - Webhook: Generic webhook for custom integrations
//
// # Basic Usage
//
// Creating and configuring the notification engine:
//
//	engine := notification.NewNotificationEngine()
//
//	// Register channels
//	slackChannel := notification.NewSlackChannel(webhookURL, "#alerts", "HelixCode")
//	engine.RegisterChannel(slackChannel)
//
//	// Send a notification
//	notif := &notification.Notification{
//	    Title:    "Build Completed",
//	    Message:  "Project build completed successfully",
//	    Type:     notification.NotificationTypeSuccess,
//	    Priority: notification.NotificationPriorityMedium,
//	    Channels: []string{"slack"},
//	}
//	err := engine.SendNotification(ctx, notif)
//
// # Notification Types and Priorities
//
// Notifications are categorized by type for appropriate formatting:
//
//	NotificationTypeInfo    - Informational messages
//	NotificationTypeWarning - Warning notifications
//	NotificationTypeError   - Error notifications
//	NotificationTypeSuccess - Success notifications
//	NotificationTypeAlert   - Critical alerts
//
// Priority levels control urgency and routing:
//
//	NotificationPriorityLow    - Background notifications
//	NotificationPriorityMedium - Standard notifications
//	NotificationPriorityHigh   - Important notifications
//	NotificationPriorityUrgent - Critical notifications requiring immediate attention
//
// # Rules Engine
//
// Notification rules automate channel selection and message transformation:
//
//	rule := notification.NotificationRule{
//	    Name:      "Error Escalation",
//	    Condition: "type==error",
//	    Channels:  []string{"slack", "pagerduty"},
//	    Priority:  notification.NotificationPriorityHigh,
//	    Enabled:   true,
//	    Template:  "error_template",
//	}
//	engine.AddRule(rule)
//
// # Rate Limiting
//
// Rate limiting protects channels from being overwhelmed:
//
//	limiter := notification.NewRateLimiter(10, time.Minute)
//	rateLimitedChannel := notification.NewRateLimitedChannel(slackChannel, limiter)
//	engine.RegisterChannel(rateLimitedChannel)
//
// Default rate limits are provided for common channels:
//
//	Slack:    1 per second
//	Discord:  5 per 5 seconds
//	Telegram: 30 per second
//	Email:    10 per minute
//	Webhook:  100 per minute
//
// # Retry Logic
//
// Automatic retries with exponential backoff for transient failures:
//
//	retryConfig := notification.RetryConfig{
//	    MaxRetries:     3,
//	    InitialBackoff: time.Second,
//	    MaxBackoff:     30 * time.Second,
//	    BackoffFactor:  2.0,
//	    Enabled:        true,
//	}
//	retryChannel := notification.NewRetryableChannel(channel, retryConfig)
//
// # Queue-Based Delivery
//
// For high-volume scenarios, use the notification queue:
//
//	queue := notification.NewNotificationQueue(engine, 5, 1000) // 5 workers, 1000 max items
//	queue.Start()
//	defer queue.Stop()
//
//	err := queue.Enqueue(notification, []string{"slack"}, 3) // max 3 retries
//
// # Event Integration
//
// Automatic notifications based on system events:
//
//	handler := notification.NewEventNotificationHandler(engine)
//	handler.RegisterWithEventBus(eventBus)
//
// Supported events include:
//   - Task completion and failure
//   - Workflow completion and failure
//   - Worker disconnection and health degradation
//   - System errors, startup, and shutdown
//
// # Templates
//
// HTML templates for message formatting:
//
//	templateStr := "{{.Title}}: {{.Message}}"
//	engine.LoadTemplate("alert_template", templateStr)
//
// # Metrics
//
// Comprehensive metrics for monitoring:
//
//	metrics := notification.NewMetrics()
//	metrics.RecordSent("slack", duration)
//	metrics.RecordFailed("email")
//
//	successRate := metrics.GetSuccessRate()
//	channelRate := metrics.GetChannelSuccessRate("slack")
//
// # Thread Safety
//
// All components in this package are thread-safe and can be used
// concurrently from multiple goroutines. The engine, queue, and
// metrics tracker use appropriate synchronization primitives.
package notification
