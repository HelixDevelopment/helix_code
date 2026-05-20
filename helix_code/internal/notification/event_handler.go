package notification

import (
	"context"
	"fmt"
	"log"

	"dev.helix.code/internal/event"
)

// EventNotificationHandler handles system events and sends notifications
type EventNotificationHandler struct {
	engine *NotificationEngine
}

// NewEventNotificationHandler creates a new event notification handler
func NewEventNotificationHandler(engine *NotificationEngine) *EventNotificationHandler {
	return &EventNotificationHandler{
		engine: engine,
	}
}

// HandleEvent processes an event and sends appropriate notifications
func (h *EventNotificationHandler) HandleEvent(ctx context.Context, evt event.Event) error {
	if h.engine == nil {
		return fmt.Errorf("notification engine not initialized")
	}

	notification := h.eventToNotification(ctx, evt)
	if notification == nil {
		// Not all events need notifications
		return nil
	}

	log.Printf("Sending notification for event: %s (ID: %s)", evt.Type, evt.ID)

	// Send notification through engine which will apply rules
	return h.engine.SendNotification(ctx, notification)
}

// eventToNotification converts an event to a notification
func (h *EventNotificationHandler) eventToNotification(ctx context.Context, evt event.Event) *Notification {
	// Determine notification type and priority based on event severity
	notifType := h.severityToNotificationType(evt.Severity)
	priority := h.severityToPriority(evt.Severity)

	// Create notification based on event type
	switch evt.Type {
	// Task events
	case event.EventTaskCompleted:
		return h.createTaskCompletedNotification(ctx, evt, notifType, priority)
	case event.EventTaskFailed:
		return h.createTaskFailedNotification(ctx, evt, notifType, priority)
	case event.EventTaskStarted:
		// Don't send notifications for task started (too noisy)
		return nil

	// Workflow events
	case event.EventWorkflowCompleted:
		return h.createWorkflowCompletedNotification(ctx, evt, notifType, priority)
	case event.EventWorkflowFailed:
		return h.createWorkflowFailedNotification(ctx, evt, notifType, priority)

	// Worker events
	case event.EventWorkerDisconnected:
		return h.createWorkerDisconnectedNotification(ctx, evt, notifType, priority)
	case event.EventWorkerHealthDegraded:
		return h.createWorkerHealthDegradedNotification(ctx, evt, notifType, priority)

	// System events
	case event.EventSystemError:
		return h.createSystemErrorNotification(ctx, evt, notifType, priority)
	case event.EventSystemStartup:
		return h.createSystemStartupNotification(ctx, evt, notifType, priority)
	case event.EventSystemShutdown:
		return h.createSystemShutdownNotification(ctx, evt, notifType, priority)

	default:
		// Event type doesn't require notification
		return nil
	}
}

// Task notification creators

func (h *EventNotificationHandler) createTaskCompletedNotification(ctx context.Context, evt event.Event, notifType NotificationType, priority NotificationPriority) *Notification {
	taskID := h.getDataString(evt.Data, "task_id", evt.TaskID)
	duration := h.getDataString(evt.Data, "duration", "")

	message := tr(ctx, "internal_notification_message_task_completed", map[string]any{"TaskID": taskID})
	if duration != "" {
		message += tr(ctx, "internal_notification_message_duration_suffix", map[string]any{"Duration": duration})
	}

	return &Notification{
		Title:    tr(ctx, "internal_notification_title_task_completed", nil),
		Message:  message,
		Type:     NotificationTypeSuccess,
		Priority: NotificationPriorityLow,
		Metadata: map[string]interface{}{
			"event_id":   evt.ID,
			"event_type": string(evt.Type),
			"task_id":    taskID,
			"duration":   duration,
			"project_id": evt.ProjectID,
			"user_id":    evt.UserID,
		},
	}
}

func (h *EventNotificationHandler) createTaskFailedNotification(ctx context.Context, evt event.Event, notifType NotificationType, priority NotificationPriority) *Notification {
	taskID := h.getDataString(evt.Data, "task_id", evt.TaskID)
	errorMsg := h.getDataString(evt.Data, "error", tr(ctx, "internal_notification_value_unknown_error", nil))

	message := tr(ctx, "internal_notification_message_task_failed", map[string]any{"TaskID": taskID, "Error": errorMsg})

	return &Notification{
		Title:    tr(ctx, "internal_notification_title_task_failed", nil),
		Message:  message,
		Type:     NotificationTypeError,
		Priority: NotificationPriorityHigh,
		Metadata: map[string]interface{}{
			"event_id":   evt.ID,
			"event_type": string(evt.Type),
			"task_id":    taskID,
			"error":      errorMsg,
			"project_id": evt.ProjectID,
			"user_id":    evt.UserID,
		},
	}
}

// Workflow notification creators

func (h *EventNotificationHandler) createWorkflowCompletedNotification(ctx context.Context, evt event.Event, notifType NotificationType, priority NotificationPriority) *Notification {
	workflowID := h.getDataString(evt.Data, "workflow_id", "")
	workflowName := h.getDataString(evt.Data, "workflow_name", workflowID)

	return &Notification{
		Title:    tr(ctx, "internal_notification_title_workflow_completed", nil),
		Message:  tr(ctx, "internal_notification_message_workflow_completed", map[string]any{"WorkflowName": workflowName}),
		Type:     NotificationTypeSuccess,
		Priority: NotificationPriorityMedium,
		Metadata: map[string]interface{}{
			"event_id":      evt.ID,
			"event_type":    string(evt.Type),
			"workflow_id":   workflowID,
			"workflow_name": workflowName,
			"project_id":    evt.ProjectID,
			"user_id":       evt.UserID,
		},
	}
}

func (h *EventNotificationHandler) createWorkflowFailedNotification(ctx context.Context, evt event.Event, notifType NotificationType, priority NotificationPriority) *Notification {
	workflowID := h.getDataString(evt.Data, "workflow_id", "")
	workflowName := h.getDataString(evt.Data, "workflow_name", workflowID)
	errorMsg := h.getDataString(evt.Data, "error", tr(ctx, "internal_notification_value_unknown_error", nil))

	return &Notification{
		Title:    tr(ctx, "internal_notification_title_workflow_failed", nil),
		Message:  tr(ctx, "internal_notification_message_workflow_failed", map[string]any{"WorkflowName": workflowName, "Error": errorMsg}),
		Type:     NotificationTypeError,
		Priority: NotificationPriorityHigh,
		Metadata: map[string]interface{}{
			"event_id":      evt.ID,
			"event_type":    string(evt.Type),
			"workflow_id":   workflowID,
			"workflow_name": workflowName,
			"error":         errorMsg,
			"project_id":    evt.ProjectID,
			"user_id":       evt.UserID,
		},
	}
}

// Worker notification creators

func (h *EventNotificationHandler) createWorkerDisconnectedNotification(ctx context.Context, evt event.Event, notifType NotificationType, priority NotificationPriority) *Notification {
	workerID := h.getDataString(evt.Data, "worker_id", evt.WorkerID)
	workerHost := h.getDataString(evt.Data, "host", tr(ctx, "internal_notification_value_unknown_host", nil))
	reason := h.getDataString(evt.Data, "reason", tr(ctx, "internal_notification_value_unknown_reason", nil))

	return &Notification{
		Title:    tr(ctx, "internal_notification_title_worker_disconnected", nil),
		Message:  tr(ctx, "internal_notification_message_worker_disconnected", map[string]any{"WorkerID": workerID, "Host": workerHost, "Reason": reason}),
		Type:     NotificationTypeWarning,
		Priority: NotificationPriorityMedium,
		Metadata: map[string]interface{}{
			"event_id":   evt.ID,
			"event_type": string(evt.Type),
			"worker_id":  workerID,
			"host":       workerHost,
			"reason":     reason,
		},
	}
}

func (h *EventNotificationHandler) createWorkerHealthDegradedNotification(ctx context.Context, evt event.Event, notifType NotificationType, priority NotificationPriority) *Notification {
	workerID := h.getDataString(evt.Data, "worker_id", evt.WorkerID)
	workerHost := h.getDataString(evt.Data, "host", tr(ctx, "internal_notification_value_unknown_host", nil))
	healthStatus := h.getDataString(evt.Data, "health_status", tr(ctx, "internal_notification_value_health_degraded", nil))

	return &Notification{
		Title:    tr(ctx, "internal_notification_title_worker_health_degraded", nil),
		Message:  tr(ctx, "internal_notification_message_worker_health_degraded", map[string]any{"WorkerID": workerID, "Host": workerHost, "HealthStatus": healthStatus}),
		Type:     NotificationTypeWarning,
		Priority: NotificationPriorityMedium,
		Metadata: map[string]interface{}{
			"event_id":      evt.ID,
			"event_type":    string(evt.Type),
			"worker_id":     workerID,
			"host":          workerHost,
			"health_status": healthStatus,
		},
	}
}

// System notification creators

func (h *EventNotificationHandler) createSystemErrorNotification(ctx context.Context, evt event.Event, notifType NotificationType, priority NotificationPriority) *Notification {
	component := h.getDataString(evt.Data, "component", evt.Source)
	errorMsg := h.getDataString(evt.Data, "error", tr(ctx, "internal_notification_value_unknown_error", nil))

	return &Notification{
		Title:    tr(ctx, "internal_notification_title_system_error", nil),
		Message:  tr(ctx, "internal_notification_message_system_error", map[string]any{"Component": component, "Error": errorMsg}),
		Type:     NotificationTypeError,
		Priority: NotificationPriorityUrgent,
		Metadata: map[string]interface{}{
			"event_id":   evt.ID,
			"event_type": string(evt.Type),
			"component":  component,
			"error":      errorMsg,
		},
	}
}

func (h *EventNotificationHandler) createSystemStartupNotification(ctx context.Context, evt event.Event, notifType NotificationType, priority NotificationPriority) *Notification {
	version := h.getDataString(evt.Data, "version", "")

	message := tr(ctx, "internal_notification_message_system_started", nil)
	if version != "" {
		message += tr(ctx, "internal_notification_message_version_suffix", map[string]any{"Version": version})
	}

	return &Notification{
		Title:    tr(ctx, "internal_notification_title_system_started", nil),
		Message:  message,
		Type:     NotificationTypeInfo,
		Priority: NotificationPriorityLow,
		Metadata: map[string]interface{}{
			"event_id":   evt.ID,
			"event_type": string(evt.Type),
			"version":    version,
		},
	}
}

func (h *EventNotificationHandler) createSystemShutdownNotification(ctx context.Context, evt event.Event, notifType NotificationType, priority NotificationPriority) *Notification {
	reason := h.getDataString(evt.Data, "reason", tr(ctx, "internal_notification_value_normal_shutdown", nil))

	return &Notification{
		Title:    tr(ctx, "internal_notification_title_system_shutdown", nil),
		Message:  tr(ctx, "internal_notification_message_system_shutdown", map[string]any{"Reason": reason}),
		Type:     NotificationTypeWarning,
		Priority: NotificationPriorityMedium,
		Metadata: map[string]interface{}{
			"event_id":   evt.ID,
			"event_type": string(evt.Type),
			"reason":     reason,
		},
	}
}

// Helper methods

func (h *EventNotificationHandler) severityToNotificationType(severity event.EventSeverity) NotificationType {
	switch severity {
	case event.SeverityCritical:
		return NotificationTypeAlert
	case event.SeverityError:
		return NotificationTypeError
	case event.SeverityWarning:
		return NotificationTypeWarning
	default:
		return NotificationTypeInfo
	}
}

func (h *EventNotificationHandler) severityToPriority(severity event.EventSeverity) NotificationPriority {
	switch severity {
	case event.SeverityCritical:
		return NotificationPriorityUrgent
	case event.SeverityError:
		return NotificationPriorityHigh
	case event.SeverityWarning:
		return NotificationPriorityMedium
	default:
		return NotificationPriorityLow
	}
}

func (h *EventNotificationHandler) getDataString(data map[string]interface{}, key string, defaultValue string) string {
	if data == nil {
		return defaultValue
	}
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// RegisterWithEventBus subscribes the handler to relevant events
func (h *EventNotificationHandler) RegisterWithEventBus(bus *event.EventBus) {
	// Task events
	bus.Subscribe(event.EventTaskCompleted, h.HandleEvent)
	bus.Subscribe(event.EventTaskFailed, h.HandleEvent)

	// Workflow events
	bus.Subscribe(event.EventWorkflowCompleted, h.HandleEvent)
	bus.Subscribe(event.EventWorkflowFailed, h.HandleEvent)

	// Worker events
	bus.Subscribe(event.EventWorkerDisconnected, h.HandleEvent)
	bus.Subscribe(event.EventWorkerHealthDegraded, h.HandleEvent)

	// System events
	bus.Subscribe(event.EventSystemError, h.HandleEvent)
	bus.Subscribe(event.EventSystemStartup, h.HandleEvent)
	bus.Subscribe(event.EventSystemShutdown, h.HandleEvent)

	log.Println("Notification event handler registered with event bus")
}
